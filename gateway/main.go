// Command gateway is the Toolset API HTTP gateway.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/yourusername/toolset-api/gateway/internal/auth"
	"github.com/yourusername/toolset-api/gateway/internal/config"
	"github.com/yourusername/toolset-api/gateway/internal/db"
	"github.com/yourusername/toolset-api/gateway/internal/handlers"
	"github.com/yourusername/toolset-api/gateway/internal/registry"
)

// Version is injected at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	// Support a lightweight `gateway healthcheck` subcommand for Docker.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, cfgPath, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	logger := newLogger(cfg.Server.LogLevel)
	if cfgPath != "" {
		logger.Info("loaded config", "path", cfgPath)
	} else {
		logger.Info("no config.yaml found; using defaults + env")
	}

	// Initialize the database and run migrations.
	database, err := db.Open(db.Config{
		Path:           cfg.DB.Path,
		MaxConnections: cfg.DB.MaxConnections,
		MigrationsDir:  migrationsDir(),
	})
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer database.Close()
	logger.Info("database ready", "path", database.Path())

	reg := registry.New(database.DB)

	// Log tool readiness based on config.
	for name, tc := range cfg.Tools {
		state := "waiting"
		if tc.Enabled {
			state = "enabled"
		}
		logger.Info("tool configured", "tool", name, "container", tc.Container, "state", state)
	}

	authn := auth.New(auth.AuthConfig{
		Mode:             cfg.Auth.Mode,
		DefaultRateLimit: cfg.Auth.RateLimit,
	}, database.DB, nil)

	h := handlers.New(reg, Version)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Global middleware.
	e.Use(auth.RequestID())
	e.Use(echomw.Recover())
	e.Use(requestLogger(logger))

	// Public root & health (no auth).
	e.GET("/", h.Root)
	e.GET("/health", h.Health)

	// Authenticated API group.
	api := e.Group("")
	api.Use(authn.Middleware())

	api.GET("/tools", h.Tools)
	api.POST("/tools/list", h.MCPToolsList)
	api.POST("/tools/call", h.MCPToolsCall)

	api.POST("/mcp/initialize", h.MCPInitialize)
	api.POST("/mcp/tools/list", h.MCPToolsList)
	api.POST("/mcp/tools/call", h.MCPToolsCall)
	api.POST("/mcp/resources/read", h.MCPResourcesRead)
	api.GET("/mcp/schema", h.MCPSchema)

	// Choose a port, falling back if the primary is occupied.
	addr, err := pickAddr(cfg.Server.Port, cfg.Server.FallbackPort, logger)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start the server.
	serverErr := make(chan error, 1)
	go func() {
		logger.Info("gateway listening", "addr", addr, "auth_mode", cfg.Auth.Mode, "version", Version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Wait for interrupt or server error.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return fmt.Errorf("server error: %w", err)
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	// Graceful shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}
	logger.Info("gateway stopped cleanly")
	return nil
}

// pickAddr returns the first available listen address, preferring primary.
func pickAddr(primary, fallback int, logger *slog.Logger) (string, error) {
	for _, port := range []int{primary, fallback} {
		if port <= 0 {
			continue
		}
		addr := fmt.Sprintf(":%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			if port == fallback && fallback != primary {
				logger.Warn("primary port unavailable; using fallback", "primary", primary, "fallback", fallback)
			}
			return addr, nil
		}
		logger.Warn("port unavailable", "port", port, "err", err)
	}
	return "", fmt.Errorf("no available port (tried %d, %d)", primary, fallback)
}

// migrationsDir resolves the migrations directory relative to the binary's
// working context. Falls back to ./migrations then ../migrations.
func migrationsDir() string {
	candidates := []string{"migrations", "gateway/migrations", "../migrations"}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return "migrations"
}

// newLogger builds a slog logger at the requested level.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// requestLogger logs each request with its injected request ID.
func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			rid, _ := c.Get(auth.CtxRequestID).(string)
			logger.Info("request",
				"request_id", rid,
				"method", c.Request().Method,
				"path", c.Request().URL.Path,
				"status", c.Response().Status,
				"dur_ms", time.Since(start).Milliseconds(),
			)
			return err
		}
	}
}

// runHealthcheck performs an in-process health probe used by Docker HEALTHCHECK.
func runHealthcheck() int {
	port := os.Getenv("TOOLSET_PORT")
	if port == "" {
		port = "8080"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck status:", resp.StatusCode)
		return 1
	}
	return 0
}
