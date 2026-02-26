package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Server ServerConfig
	Store  StoreConfig
	Agent  AgentConfig
	Log    LogConfig
}

type ServerConfig struct {
	Port int    // default 7117
	Host string // default "127.0.0.1"
}

type StoreConfig struct {
	Type    string // "bolt" or "memory"
	DataDir string // default "~/.orca/data"
}

type AgentConfig struct {
	ClaudeCLI           string // path to claude binary (default: "claude", resolved via PATH)
	DefaultModel        string // default "claude-sonnet-4-20250514"
	DefaultMaxTokens    int    // default 8192
	DefaultTimeout      int    // default 300 (seconds)
	HealthCheckInterval int    // default 30 (seconds)
}

type LogConfig struct {
	Level  string // default "info"
	Format string // default "console"
}

// DefaultConfig returns a Config populated with all default values.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port: 7117,
			Host: "127.0.0.1",
		},
		Store: StoreConfig{
			Type:    "bolt",
			DataDir: defaultDataDir(),
		},
		Agent: AgentConfig{
			ClaudeCLI:           "claude",
			DefaultModel:        "claude-sonnet-4-20250514",
			DefaultMaxTokens:    8192,
			DefaultTimeout:      300,
			HealthCheckInterval: 30,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "console",
		},
	}
}

// ServerAddress returns the listen address in "host:port" format.
func (c *Config) ServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}

// DBPath returns the full path to the BoltDB file (DataDir + "/orca.db").
func (c *Config) DBPath() string {
	return filepath.Join(c.Store.DataDir, "orca.db")
}

// defaultDataDir resolves the default data directory.
// It uses os.UserHomeDir() + "/.orca/data", falling back to "/tmp/orca/data"
// if the home directory cannot be determined.
func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("/tmp", "orca", "data")
	}
	return filepath.Join(home, ".orca", "data")
}
