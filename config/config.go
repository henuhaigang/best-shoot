package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Duration wraps time.Duration for JSON string parsing.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// ServerConfig holds server configuration.
type ServerConfig struct {
	ListenAddr        string   `json:"listen_addr"`
	HeartbeatTimeout  Duration `json:"heartbeat_timeout"`
	HeartbeatInterval Duration `json:"heartbeat_interval"`
	LogLevel          string   `json:"log_level"`
}

// DefaultServerConfig returns sensible defaults.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		ListenAddr:        ":7700",
		HeartbeatTimeout:  Duration{90 * time.Second},
		HeartbeatInterval: Duration{30 * time.Second},
		LogLevel:          "info",
	}
}

// ClientConfig holds client configuration.
type ClientConfig struct {
	ServerAddr        string   `json:"server_addr"`
	ClientID          string   `json:"client_id"`
	ReconnectBase     Duration `json:"reconnect_base"`
	ReconnectMax      Duration `json:"reconnect_max"`
	HeartbeatInterval Duration `json:"heartbeat_interval"`
	LogLevel          string   `json:"log_level"`
}

// DefaultClientConfig returns sensible defaults.
func DefaultClientConfig() ClientConfig {
	return ClientConfig{
		ServerAddr:        "127.0.0.1:7700",
		ClientID:          "",
		ReconnectBase:     Duration{time.Second},
		ReconnectMax:      Duration{30 * time.Second},
		HeartbeatInterval: Duration{30 * time.Second},
		LogLevel:          "info",
	}
}

// Load reads a JSON config file and merges with defaults.
func LoadServer(path string) (ServerConfig, error) {
	cfg := DefaultServerConfig()
	if path == "" {
		return cfg, nil
	}
	if err := loadJSON(path, &cfg); err != nil {
		return cfg, fmt.Errorf("load server config: %w", err)
	}
	return cfg, nil
}

// LoadClient reads a JSON config file and merges with defaults.
func LoadClient(path string) (ClientConfig, error) {
	cfg := DefaultClientConfig()
	if path == "" {
		return cfg, nil
	}
	if err := loadJSON(path, &cfg); err != nil {
		return cfg, fmt.Errorf("load client config: %w", err)
	}
	return cfg, nil
}

// LoadServerFromEnv loads server config from environment variables.
// Env vars override file config if both are set.
func LoadServerFromEnv(cfg *ServerConfig) {
	if v := os.Getenv("TUNNEL_LISTEN"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("TUNNEL_HEARTBEAT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HeartbeatTimeout = Duration{d}
		}
	}
	if v := os.Getenv("TUNNEL_HEARTBEAT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HeartbeatInterval = Duration{d}
		}
	}
	if v := os.Getenv("TUNNEL_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}

// LoadClientFromEnv loads client config from environment variables.
func LoadClientFromEnv(cfg *ClientConfig) {
	if v := os.Getenv("TUNNEL_SERVER"); v != "" {
		cfg.ServerAddr = v
	}
	if v := os.Getenv("TUNNEL_CLIENT_ID"); v != "" {
		cfg.ClientID = v
	}
	if v := os.Getenv("TUNNEL_RECONNECT_BASE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReconnectBase = Duration{d}
		}
	}
	if v := os.Getenv("TUNNEL_RECONNECT_MAX"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReconnectMax = Duration{d}
		}
	}
	if v := os.Getenv("TUNNEL_HEARTBEAT_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.HeartbeatInterval = Duration{d}
		}
	}
	if v := os.Getenv("TUNNEL_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
}

func loadJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
