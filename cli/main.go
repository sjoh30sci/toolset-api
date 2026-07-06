// Command toolset is the CLI for managing the Toolset API stack.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// Version, BuildDate, and GitCommit are injected at build time via
// -ldflags "-X main.Version=... -X main.BuildDate=... -X main.GitCommit=...".
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "none"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "toolset",
		Short:   "Manage the Toolset API stack",
		Version: Version,
	}
	root.SetVersionTemplate(fmt.Sprintf("toolset %s (commit %s, built %s)\n", Version, GitCommit, BuildDate))
	root.AddCommand(initCmd(), upCmd(), downCmd(), logsCmd(), statusCmd(), packageCmd())
	return root
}

// --- Commands ---------------------------------------------------------------

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold config, docker-compose, and runtime directories",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, d := range []string{"data", "logs"} {
				if err := os.MkdirAll(d, 0o755); err != nil {
					return err
				}
				fmt.Printf("ensured directory: %s\n", d)
			}
			if err := copyIfMissing("config.yaml.example", "config.yaml"); err != nil {
				return err
			}
			if err := copyIfMissing(".env.example", ".env.local"); err != nil {
				return err
			}
			fmt.Println("init complete")
			return nil
		},
	}
}

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start the stack (docker-compose up -d) and wait for health",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCompose("up", "-d"); err != nil {
				return err
			}
			fmt.Print("waiting for gateway health")
			for i := 0; i < 30; i++ {
				if ok, _ := probeHealth(); ok {
					fmt.Println("\ngateway healthy")
					return nil
				}
				fmt.Print(".")
				time.Sleep(time.Second)
			}
			fmt.Println("\nwarning: gateway did not report healthy in time")
			return nil
		},
	}
}

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the stack (docker-compose down)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCompose("down")
		},
	}
}

func logsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs <service>",
		Short: "Follow logs for a service",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cargs := []string{"logs", "-f"}
			if len(args) == 1 {
				cargs = append(cargs, args[0])
			}
			return runCompose(cargs...)
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Query the gateway /health endpoint and report tool status",
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, body := probeHealth()
			if !ok {
				fmt.Println("gateway: UNREACHABLE")
				return fmt.Errorf("gateway not responding")
			}
			var pretty map[string]any
			if err := json.Unmarshal(body, &pretty); err == nil {
				out, _ := json.MarshalIndent(pretty, "", "  ")
				fmt.Println(string(out))
			} else {
				fmt.Println(string(body))
			}
			return nil
		},
	}
}

// --- Helpers ----------------------------------------------------------------

// findComposeDir walks up from cwd looking for a docker-compose.yml.
func findComposeDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.yml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("docker-compose.yml not found in cwd or any parent")
		}
		dir = parent
	}
}

// runCompose invokes docker-compose in the discovered compose directory.
func runCompose(args ...string) error {
	dir, err := findComposeDir()
	if err != nil {
		return err
	}
	c := exec.Command("docker-compose", args...)
	c.Dir = dir
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	return c.Run()
}

// probeHealth queries the local gateway health endpoint.
func probeHealth() (bool, []byte) {
	port := os.Getenv("TOOLSET_PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode == http.StatusOK, body
}

// copyIfMissing copies src to dst only when dst does not already exist.
func copyIfMissing(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		fmt.Printf("exists, skipping: %s\n", dst)
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Printf("skip %s (source %s missing)\n", dst, src)
		return nil
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("created: %s\n", dst)
	return nil
}
