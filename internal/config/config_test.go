package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaultsWhenFileMissing(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Daemon.Host != "127.0.0.1" {
		t.Fatalf("host = %s, want 127.0.0.1", cfg.Daemon.Host)
	}
	if !strings.Contains(cfg.Logging.DBPath, ".tokfence") {
		t.Fatalf("db path = %s, expected tokfence path", cfg.Logging.DBPath)
	}
	if cfg.Daemon.SocketPath != "" && !strings.Contains(cfg.Daemon.SocketPath, ".tokfence/tokfence.sock") {
		t.Fatalf("socket_path = %s, expected default path in tokfence home", cfg.Daemon.SocketPath)
	}
	if cfg.Daemon.ImmuneEnabled != true {
		t.Fatalf("immune_enabled = %v, want true", cfg.Daemon.ImmuneEnabled)
	}
	if cfg.RiskDefaults.InitialState != "GREEN" {
		t.Fatalf("risk_defaults.initial_state = %s, want GREEN", cfg.RiskDefaults.InitialState)
	}
}

func TestLoadRejectsInsecureHost(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "[daemon]\nhost='0.0.0.0'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected Load() to reject host=0.0.0.0")
	}
}

func TestLoadExpandsSocketPathAndCreatesParent(t *testing.T) {
	home, err := os.MkdirTemp("/tmp", "tokfence-home-")
	if err != nil {
		t.Fatalf("create short temp home: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)

	path := filepath.Join(t.TempDir(), "config.toml")
	cfgContent := "[daemon]\nsocket_path = \"~/.tokfence/private/socket/tokfence.sock\"\n"
	if err := os.WriteFile(path, []byte(cfgContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	expectedPath := filepath.Join(home, ".tokfence", "private", "socket", "tokfence.sock")
	if loaded.Daemon.SocketPath != expectedPath {
		t.Fatalf("socket_path = %s, want %s", loaded.Daemon.SocketPath, expectedPath)
	}
	if _, err := os.Stat(filepath.Dir(expectedPath)); err != nil {
		t.Fatalf("expected parent dir to exist after load: %v", err)
	}
}

func TestLoadRejectsOverlongSocketPath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "config.toml")
	longName := bytes.Repeat([]byte("a"), 120)
	content := fmt.Sprintf("[daemon]\nsocket_path = \"/%s.sock\"\n", string(longName))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatalf("expected Load() to reject overlong socket path")
	}
}

func TestLoadInvalidRiskDefaultsReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "[risk_defaults]\ninitial_state = \"BAD\"\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected invalid risk initial_state to error")
	}
}
