// Command executor-server runs untrusted code inside an nsjail sandbox and
// exposes a small HTTP API. The same binary ships in both the exec-light and
// exec-heavy images; the set of available language runtimes differs by image,
// not by code.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// maxCodeSize caps the accepted source payload.
	maxCodeSize = 50 * 1024 * 1024 // 50MB
	// defaultTimeout applies when the request omits a timeout.
	defaultTimeout = 30
	// maxTimeout is the hard upper bound on any single execution.
	maxTimeout = 300
	// listenAddr is the fixed internal port for the executor API.
	listenAddr = ":8765"
)

// nsjailConfig is the path to the sandbox configuration baked into the image.
// It can be overridden via NSJAIL_CONFIG for testing or custom deployments.
var nsjailConfig = envOr("NSJAIL_CONFIG", "/etc/nsjail/nsjail.cfg")

// nsjailBin is the nsjail executable path.
var nsjailBin = envOr("NSJAIL_BIN", "/usr/bin/nsjail")

// useNsjail toggles sandboxing. It is enabled by default; set NSJAIL_DISABLE=1
// to run commands directly (used only in constrained CI where nsjail's
// namespaces are unavailable).
var useNsjail = os.Getenv("NSJAIL_DISABLE") != "1"

// ExecRequest is the inbound execution request.
type ExecRequest struct {
	Code     string            `json:"code"`
	Language string            `json:"language"`
	Timeout  int               `json:"timeout"` // seconds, default 30
	Stdin    string            `json:"stdin"`   // optional
	Env      map[string]string `json:"env"`     // optional
}

// ExecResponse is the execution result returned to the caller.
type ExecResponse struct {
	Status   string  `json:"status"` // success, timeout, error
	ExitCode int     `json:"exit_code"`
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Duration float64 `json:"duration_seconds"`
	Error    string  `json:"error,omitempty"`
}

// langSpec describes how to build and run a language.
type langSpec struct {
	// filename is the source file written into the work dir (empty for
	// interpreted languages executed via -c/-e).
	filename string
	// compile builds the command list used to compile the source. It returns
	// nil for interpreted languages. workDir and paths are absolute.
	compile func(workDir, src, out string) []string
	// run builds the command list used to execute the program. For interpreted
	// languages, code is the raw source.
	run func(workDir, src, out, code string) []string
}

// languages is the registry of supported runtimes. Availability of the
// underlying toolchain is image-dependent (light vs heavy).
var languages = map[string]langSpec{
	"python": {
		run: func(_, _, _, code string) []string { return []string{"python3.11", "-c", code} },
	},
	"node": {
		run: func(_, _, _, code string) []string { return []string{"node", "-e", code} },
	},
	"bash": {
		run: func(_, _, _, code string) []string { return []string{"bash", "-c", code} },
	},
	"c": {
		filename: "main.c",
		compile:  func(_, src, out string) []string { return []string{"gcc", "-O2", "-o", out, src} },
		run:      func(_, _, out, _ string) []string { return []string{out} },
	},
	"cpp": {
		filename: "main.cpp",
		compile:  func(_, src, out string) []string { return []string{"g++", "-O2", "-std=c++20", "-o", out, src} },
		run:      func(_, _, out, _ string) []string { return []string{out} },
	},
	"assembly": {
		filename: "main.asm",
		compile: func(workDir, src, out string) []string {
			obj := filepath.Join(workDir, "main.o")
			// Combined assemble + link via a shell so a single command drives both.
			return []string{"bash", "-c",
				fmt.Sprintf("nasm -f elf64 -o %q %q && ld -o %q %q", obj, src, out, obj)}
		},
		run: func(_, _, out, _ string) []string { return []string{out} },
	},
	"java": {
		filename: "Main.java",
		compile:  func(_, src, _ string) []string { return []string{"javac", src} },
		run:      func(workDir, _, _, _ string) []string { return []string{"java", "-cp", workDir, "Main"} },
	},
	"rust": {
		filename: "main.rs",
		compile:  func(_, src, out string) []string { return []string{"rustc", "-O", "-o", out, src} },
		run:      func(_, _, out, _ string) []string { return []string{out} },
	},
	"csharp": {
		filename: "Program.cs",
		compile: func(workDir, src, _ string) []string {
			dll := filepath.Join(workDir, "Program.dll")
			return []string{"csc", "-nologo", "-out:" + dll, src}
		},
		run: func(workDir, _, _, _ string) []string {
			return []string{"dotnet", filepath.Join(workDir, "Program.dll")}
		},
	},
}

// dotnet is an accepted alias for csharp.
func init() { languages["dotnet"] = languages["csharp"] }

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/exec", handleExec)

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("executor server listening on %s (nsjail=%v)", listenAddr, useNsjail)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// handleHealth handles GET /health.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleExec handles POST /exec.
func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxCodeSize+1024)
	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if len(req.Code) > maxCodeSize {
		http.Error(w, "Code too large", http.StatusRequestEntityTooLarge)
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultTimeout
	}
	if req.Timeout > maxTimeout {
		req.Timeout = maxTimeout
	}

	resp := executeCode(req)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// executeCode compiles (if needed) and runs the requested code under a
// per-request deadline. Compilation and execution share the timeout budget.
func executeCode(req ExecRequest) ExecResponse {
	start := time.Now()

	spec, ok := languages[req.Language]
	if !ok {
		return ExecResponse{
			Status:   "error",
			Error:    fmt.Sprintf("Unsupported language: %s", req.Language),
			Duration: time.Since(start).Seconds(),
		}
	}

	workDir, err := os.MkdirTemp("/tmp", "toolset_exec_")
	if err != nil {
		return ExecResponse{
			Status:   "error",
			Error:    fmt.Sprintf("failed to create work dir: %v", err),
			Duration: time.Since(start).Seconds(),
		}
	}
	defer os.RemoveAll(workDir)

	var srcPath, outPath string
	if spec.filename != "" {
		srcPath = filepath.Join(workDir, spec.filename)
		if err := os.WriteFile(srcPath, []byte(req.Code), 0o644); err != nil {
			return ExecResponse{
				Status:   "error",
				Error:    fmt.Sprintf("failed to write source: %v", err),
				Duration: time.Since(start).Seconds(),
			}
		}
		outPath = filepath.Join(workDir, "main")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Timeout)*time.Second)
	defer cancel()

	// Compile step (unsandboxed: the compiler is trusted, the output is not).
	if spec.compile != nil {
		cArgs := spec.compile(workDir, srcPath, outPath)
		var cErr bytes.Buffer
		cCmd := exec.CommandContext(ctx, cArgs[0], cArgs[1:]...)
		cCmd.Dir = workDir
		cCmd.Stderr = &cErr
		if err := cCmd.Run(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return ExecResponse{
					Status:   "timeout",
					Error:    fmt.Sprintf("Compilation timeout after %d seconds", req.Timeout),
					Duration: time.Since(start).Seconds(),
				}
			}
			return ExecResponse{
				Status:   "error",
				Stderr:   cErr.String(),
				Error:    fmt.Sprintf("Compilation failed: %v", err),
				Duration: time.Since(start).Seconds(),
			}
		}
	}

	// Run step (sandboxed under nsjail when enabled).
	runArgs := spec.run(workDir, srcPath, outPath, req.Code)
	cmd := exec.CommandContext(ctx, runArgs[0], runArgs[1:]...)
	if useNsjail {
		cmd = wrapNsjail(ctx, workDir, req.Timeout, runArgs)
	}
	cmd.Dir = workDir

	cmd.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "HOME=" + workDir}
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return ExecResponse{
			Status:   "timeout",
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			Duration: time.Since(start).Seconds(),
			Error:    fmt.Sprintf("Execution timeout after %d seconds", req.Timeout),
		}
	}

	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return ExecResponse{
				Status:   "error",
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Error:    fmt.Sprintf("Execution failed: %v", runErr),
				Duration: time.Since(start).Seconds(),
			}
		}
	}

	return ExecResponse{
		Status:   "success",
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: time.Since(start).Seconds(),
	}
}

// wrapNsjail builds an nsjail-wrapped command. The program's work dir is
// bind-mounted read-write so compiled artifacts remain reachable inside the jail.
func wrapNsjail(ctx context.Context, workDir string, timeout int, runArgs []string) *exec.Cmd {
	args := []string{
		"--config", nsjailConfig,
		"--time_limit", strconv.Itoa(timeout),
		"--bindmount", workDir + ":" + workDir,
		"--cwd", workDir,
		"--",
	}
	args = append(args, runArgs...)
	return exec.CommandContext(ctx, nsjailBin, args...)
}

// envOr returns the environment variable value or a fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
