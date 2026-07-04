package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// searchPaths returns the ordered list of directories to search for config.yaml.
// Priority (highest first) is enforced by loading later files only when earlier
// ones are absent; environment variables always win via applyEnvOverrides.
func searchPaths() []string {
	paths := []string{}

	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, filepath.Join(cwd, "config.yaml"))
	}
	paths = append(paths, filepath.Join("/etc", "toolset", "config.yaml"))
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "toolset", "config.yaml"))
	}
	return paths
}

// Load builds the effective configuration. It starts from defaults, overlays the
// first config.yaml found on the search path, then applies environment overrides.
func Load() (Config, string, error) {
	cfg := Defaults()
	usedPath := ""

	for _, p := range searchPaths() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, p, fmt.Errorf("parsing %s: %w", p, err)
		}
		usedPath = p
		break
	}

	applyEnvOverrides(&cfg)

	if err := cfg.Validate(); err != nil {
		return cfg, usedPath, err
	}
	return cfg, usedPath, nil
}

// applyEnvOverrides mutates cfg based on TOOLSET_* environment variables.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TOOLSET_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("TOOLSET_SERVER_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = n
		}
	}
	if v := os.Getenv("TOOLSET_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("TOOLSET_AUTH_MODE"); v != "" {
		cfg.Auth.Mode = v
	}
	if v := os.Getenv("TOOLSET_DB_PATH"); v != "" {
		cfg.DB.Path = v
	}
}

// Validate performs basic sanity checks on the configuration.
func (c Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server.port: %d", c.Server.Port)
	}
	switch c.Auth.Mode {
	case AuthModeNone, AuthModeToken:
	default:
		return fmt.Errorf("invalid auth.mode: %q (want none|token)", c.Auth.Mode)
	}
	if c.DB.Path == "" {
		return fmt.Errorf("db.path must not be empty")
	}
	return nil
}
