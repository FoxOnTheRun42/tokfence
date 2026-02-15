package config

import (
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
