package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultServerConfig(t *testing.T) {
	cfg := DefaultServerConfig()
	if cfg.ListenAddr != ":7700" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":7700")
	}
	if cfg.HeartbeatTimeout.Duration != 90*time.Second {
		t.Errorf("HeartbeatTimeout = %v, want %v", cfg.HeartbeatTimeout, 90*time.Second)
	}
}

func TestDefaultClientConfig(t *testing.T) {
	cfg := DefaultClientConfig()
	if cfg.ServerAddr != "127.0.0.1:7700" {
		t.Errorf("ServerAddr = %q, want %q", cfg.ServerAddr, "127.0.0.1:7700")
	}
	if cfg.ReconnectBase.Duration != time.Second {
		t.Errorf("ReconnectBase = %v, want %v", cfg.ReconnectBase, time.Second)
	}
}

func TestLoadServerFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.json")
	os.WriteFile(path, []byte(`{"listen_addr":":9900","heartbeat_timeout":"60s"}`), 0644)

	cfg, err := LoadServer(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":9900" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9900")
	}
	if cfg.HeartbeatTimeout.Duration != 60*time.Second {
		t.Errorf("HeartbeatTimeout = %v, want %v", cfg.HeartbeatTimeout, 60*time.Second)
	}
	// Default should be preserved for unspecified fields
	if cfg.HeartbeatInterval.Duration != 30*time.Second {
		t.Errorf("HeartbeatInterval = %v, want default", cfg.HeartbeatInterval)
	}
}

func TestLoadClientFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "client.json")
	os.WriteFile(path, []byte(`{"server_addr":"tunnel.example.com:7700","client_id":"prod-1"}`), 0644)

	cfg, err := LoadClient(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ServerAddr != "tunnel.example.com:7700" {
		t.Errorf("ServerAddr = %q", cfg.ServerAddr)
	}
	if cfg.ClientID != "prod-1" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}
}

func TestLoadServerEmptyPath(t *testing.T) {
	cfg, err := LoadServer("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultServerConfig() {
		t.Error("expected defaults")
	}
}

func TestLoadClientEmptyPath(t *testing.T) {
	cfg, err := LoadClient("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg != DefaultClientConfig() {
		t.Error("expected defaults")
	}
}

func TestLoadServerFromEnv(t *testing.T) {
	os.Setenv("TUNNEL_LISTEN", ":8800")
	os.Setenv("TUNNEL_HEARTBEAT_TIMEOUT", "45s")
	defer os.Unsetenv("TUNNEL_LISTEN")
	defer os.Unsetenv("TUNNEL_HEARTBEAT_TIMEOUT")

	cfg := DefaultServerConfig()
	LoadServerFromEnv(&cfg)

	if cfg.ListenAddr != ":8800" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8800")
	}
	if cfg.HeartbeatTimeout.Duration != 45*time.Second {
		t.Errorf("HeartbeatTimeout = %v, want %v", cfg.HeartbeatTimeout, 45*time.Second)
	}
}

func TestLoadClientFromEnv(t *testing.T) {
	os.Setenv("TUNNEL_SERVER", "remote:7700")
	os.Setenv("TUNNEL_CLIENT_ID", "env-node")
	os.Setenv("TUNNEL_RECONNECT_BASE", "2s")
	defer os.Unsetenv("TUNNEL_SERVER")
	defer os.Unsetenv("TUNNEL_CLIENT_ID")
	defer os.Unsetenv("TUNNEL_RECONNECT_BASE")

	cfg := DefaultClientConfig()
	LoadClientFromEnv(&cfg)

	if cfg.ServerAddr != "remote:7700" {
		t.Errorf("ServerAddr = %q", cfg.ServerAddr)
	}
	if cfg.ClientID != "env-node" {
		t.Errorf("ClientID = %q", cfg.ClientID)
	}
	if cfg.ReconnectBase.Duration != 2*time.Second {
		t.Errorf("ReconnectBase = %v", cfg.ReconnectBase)
	}
}

func TestLoadServerInvalidFile(t *testing.T) {
	_, err := LoadServer("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadServerInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`not json`), 0644)

	_, err := LoadServer(path)
	if err == nil {
		t.Error("expected error for bad JSON")
	}
}
