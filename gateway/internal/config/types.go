// Package config defines the configuration data structures for the gateway.
package config

// Config is the top-level configuration for the gateway. It is populated from
// (in increasing priority): defaults -> config.yaml -> environment variables.
type Config struct {
	Server ServerConfig          `yaml:"server"`
	Auth   AuthConfig            `yaml:"auth"`
	DB     DBConfig              `yaml:"db"`
	Tools  map[string]ToolConfig `yaml:"tools"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         int    `yaml:"port"`
	FallbackPort int    `yaml:"fallback_port"`
	LogLevel     string `yaml:"log_level"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	// Mode is "none" or "token".
	Mode string `yaml:"mode"`
	// APITokens maps a logical tool name to its bearer token (token mode only).
	APITokens map[string]string `yaml:"api_tokens"`
	// RateLimit is the default requests-per-minute per token or IP.
	RateLimit int `yaml:"rate_limit"`
}

// DBConfig holds SQLite connection settings.
type DBConfig struct {
	Path           string `yaml:"path"`
	MaxConnections int    `yaml:"max_connections"`
}

// ToolConfig describes an individual downstream tool service.
type ToolConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Container   string `yaml:"container"`
	SandboxRoot string `yaml:"sandbox_root,omitempty"`
}

// Auth modes.
const (
	AuthModeNone  = "none"
	AuthModeToken = "token"
)

// Defaults returns a Config populated with sensible defaults.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Port:         8080,
			FallbackPort: 18080,
			LogLevel:     "info",
		},
		Auth: AuthConfig{
			Mode:      AuthModeNone,
			APITokens: map[string]string{},
			RateLimit: 100,
		},
		DB: DBConfig{
			Path:           "/data/toolset.db",
			MaxConnections: 10,
		},
		Tools: map[string]ToolConfig{},
	}
}
