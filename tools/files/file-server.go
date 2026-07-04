// Command file-server is a minimal, sandboxed HTTP file service for the
// Toolset API. All operations are confined to a sandbox root (default
// /data/files). Paths are validated to prevent traversal (`..`), absolute
// paths, and symlink escape. Structured JSON logs are written to stdout.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// maxFileSize caps write payloads and read responses at 100MB.
const maxFileSize = 100 * 1024 * 1024

// sandboxRoot is the resolved absolute root all operations are confined to.
var sandboxRoot string

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	root := os.Getenv("FILES_SANDBOX_ROOT")
	if root == "" {
		root = "/data/files"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("creating sandbox root %q: %w", root, err)
	}
	// Resolve symlinks in the root itself so later comparisons are stable.
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolving sandbox root %q: %w", root, err)
	}
	sandboxRoot = resolved

	port := os.Getenv("FILES_PORT")
	if port == "" {
		port = "8765"
	}

	logInfo("file-server starting", map[string]any{"sandbox_root": sandboxRoot, "port": port})

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/files/read", handleRead)
	mux.HandleFunc("/files/write", handleWrite)
	mux.HandleFunc("/files/list", handleList)
	mux.HandleFunc("/files/delete", handleDelete)
	mux.HandleFunc("/files/move", handleMove)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-stop:
		logInfo("shutdown signal received", map[string]any{"signal": sig.String()})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	logInfo("file-server stopped cleanly", nil)
	return nil
}

// --- Request/response types -------------------------------------------------

type readReq struct {
	Path string `json:"path"`
}

type writeReq struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type listReq struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

type deleteReq struct {
	Path string `json:"path"`
}

type moveReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// --- Handlers ---------------------------------------------------------------

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleRead(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req readReq
	if !decode(w, r, &req) {
		return
	}
	abs, err := resolvePath(req.Path)
	if err != nil {
		logInfo("read rejected", map[string]any{"path": req.Path, "result": "bad_path", "err": err.Error()})
		writeErr(w, http.StatusBadRequest, "path outside sandbox")
		return
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		logInfo("read not found", map[string]any{"path": req.Path, "result": "not_found"})
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil || info.IsDir() {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if info.Size() > maxFileSize {
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "read failed")
		return
	}
	logInfo("read ok", map[string]any{"path": req.Path, "op": "read", "result": "ok", "bytes": len(data)})
	writeJSON(w, http.StatusOK, map[string]any{"content": string(data)})
}

func handleWrite(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req writeReq
	if !decode(w, r, &req) {
		return
	}
	if len(req.Content) > maxFileSize {
		logInfo("write rejected", map[string]any{"path": req.Path, "result": "too_large"})
		writeErr(w, http.StatusRequestEntityTooLarge, "file too large")
		return
	}
	abs, err := resolveForCreate(req.Path)
	if err != nil {
		logInfo("write rejected", map[string]any{"path": req.Path, "result": "bad_path", "err": err.Error()})
		writeErr(w, http.StatusBadRequest, "path outside sandbox")
		return
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "write failed")
		return
	}
	if err := os.WriteFile(abs, []byte(req.Content), 0o644); err != nil {
		writeErr(w, http.StatusInternalServerError, "write failed")
		return
	}
	logInfo("write ok", map[string]any{"path": req.Path, "op": "write", "result": "ok", "bytes": len(req.Content)})
	writeJSON(w, http.StatusCreated, map[string]any{"path": req.Path, "size": len(req.Content)})
}

func handleList(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req listReq
	if !decode(w, r, &req) {
		return
	}
	abs, err := resolvePath(req.Path)
	if err != nil {
		logInfo("list rejected", map[string]any{"path": req.Path, "result": "bad_path", "err": err.Error()})
		writeErr(w, http.StatusBadRequest, "path outside sandbox")
		return
	}
	info, err := os.Stat(abs)
	if errors.Is(err, os.ErrNotExist) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil || !info.IsDir() {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}

	files := []string{}
	dirs := []string{}

	if req.Recursive {
		err = filepath.Walk(abs, func(p string, fi os.FileInfo, werr error) error {
			if werr != nil {
				return werr
			}
			if p == abs {
				return nil
			}
			rel, rerr := filepath.Rel(abs, p)
			if rerr != nil {
				return rerr
			}
			rel = filepath.ToSlash(rel)
			if fi.IsDir() {
				dirs = append(dirs, rel)
			} else {
				files = append(files, rel)
			}
			return nil
		})
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "list failed")
			return
		}
	} else {
		entries, derr := os.ReadDir(abs)
		if derr != nil {
			writeErr(w, http.StatusInternalServerError, "list failed")
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			} else {
				files = append(files, e.Name())
			}
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)

	logInfo("list ok", map[string]any{"path": req.Path, "op": "list", "result": "ok", "files": len(files), "dirs": len(dirs)})
	writeJSON(w, http.StatusOK, map[string]any{"files": files, "dirs": dirs})
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req deleteReq
	if !decode(w, r, &req) {
		return
	}
	abs, err := resolvePath(req.Path)
	if err != nil {
		logInfo("delete rejected", map[string]any{"path": req.Path, "result": "bad_path", "err": err.Error()})
		writeErr(w, http.StatusBadRequest, "path outside sandbox")
		return
	}
	if _, err := os.Stat(abs); errors.Is(err, os.ErrNotExist) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := os.RemoveAll(abs); err != nil {
		writeErr(w, http.StatusInternalServerError, "delete failed")
		return
	}
	logInfo("delete ok", map[string]any{"path": req.Path, "op": "delete", "result": "ok"})
	w.WriteHeader(http.StatusNoContent)
}

func handleMove(w http.ResponseWriter, r *http.Request) {
	if !requirePost(w, r) {
		return
	}
	var req moveReq
	if !decode(w, r, &req) {
		return
	}
	if req.From == req.To {
		writeErr(w, http.StatusBadRequest, "from and to are the same path")
		return
	}
	src, err := resolvePath(req.From)
	if err != nil {
		logInfo("move rejected", map[string]any{"from": req.From, "result": "bad_path", "err": err.Error()})
		writeErr(w, http.StatusBadRequest, "path outside sandbox")
		return
	}
	dst, err := resolveForCreate(req.To)
	if err != nil {
		logInfo("move rejected", map[string]any{"to": req.To, "result": "bad_path", "err": err.Error()})
		writeErr(w, http.StatusBadRequest, "path outside sandbox")
		return
	}
	if _, err := os.Stat(src); errors.Is(err, os.ErrNotExist) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		writeErr(w, http.StatusInternalServerError, "move failed")
		return
	}
	if err := os.Rename(src, dst); err != nil {
		writeErr(w, http.StatusInternalServerError, "move failed")
		return
	}
	logInfo("move ok", map[string]any{"from": req.From, "to": req.To, "op": "move", "result": "ok"})
	writeJSON(w, http.StatusCreated, map[string]any{"from": req.From, "to": req.To})
}

// --- Path safety ------------------------------------------------------------

// cleanRel validates a user-supplied relative path and returns it cleaned. It
// rejects absolute paths and any path that escapes the sandbox via `..`.
func cleanRel(p string) (string, error) {
	if p == "" {
		return "", errors.New("empty path")
	}
	// Normalize separators and reject absolute paths (unix or windows-style).
	p = filepath.ToSlash(p)
	if strings.HasPrefix(p, "/") || filepath.IsAbs(p) || (len(p) > 1 && p[1] == ':') {
		return "", errors.New("absolute path not allowed")
	}
	if strings.Contains(p, "..") {
		return "", errors.New("path traversal not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(p))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("path traversal not allowed")
	}
	return clean, nil
}

// withinSandbox reports whether abs is inside sandboxRoot.
func withinSandbox(abs string) bool {
	rel, err := filepath.Rel(sandboxRoot, abs)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// resolvePath validates a path that is expected to exist, resolving symlinks to
// guarantee the final target is inside the sandbox.
func resolvePath(p string) (string, error) {
	clean, err := cleanRel(p)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(sandboxRoot, clean)
	if !withinSandbox(abs) {
		return "", errors.New("path outside sandbox")
	}
	// If the target exists, resolve symlinks and re-check containment.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		if !withinSandbox(resolved) {
			return "", errors.New("symlink escapes sandbox")
		}
		return resolved, nil
	}
	return abs, nil
}

// resolveForCreate validates a path that may not exist yet (write/move dest).
// It resolves symlinks on the parent directory to prevent symlink escape.
func resolveForCreate(p string) (string, error) {
	clean, err := cleanRel(p)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(sandboxRoot, clean)
	if !withinSandbox(abs) {
		return "", errors.New("path outside sandbox")
	}
	parent := filepath.Dir(abs)
	if resolved, err := filepath.EvalSymlinks(parent); err == nil {
		if !withinSandbox(resolved) {
			return "", errors.New("symlink escapes sandbox")
		}
		return filepath.Join(resolved, filepath.Base(abs)), nil
	}
	return abs, nil
}

// --- HTTP helpers -----------------------------------------------------------

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	return true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxFileSize+1024))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "failed to read body")
		return false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

// --- Logging ----------------------------------------------------------------

// logInfo emits a single-line JSON log record to stdout.
func logInfo(msg string, fields map[string]any) {
	rec := map[string]any{
		"level": "info",
		"time":  time.Now().UTC().Format(time.RFC3339),
		"msg":   msg,
	}
	for k, v := range fields {
		rec[k] = v
	}
	b, err := json.Marshal(rec)
	if err != nil {
		fmt.Fprintf(os.Stdout, `{"level":"info","msg":%q}`+"\n", msg)
		return
	}
	fmt.Fprintln(os.Stdout, string(b))
}
