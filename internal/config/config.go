package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/macfox/tokfence/internal/security"
)

const (
	DefaultPort          = 9471
	DefaultHost          = "127.0.0.1"
	DefaultRetentionDays = 90
	DefaultSocketPath    = "~/.tokfence/tokfence.sock"
)

const (
	maxUnixSocketPathLen = 103
)

type DaemonConfig struct {
	Port             int    `toml:"port"`
	Host             string `toml:"host"`
	SocketPath       string `toml:"socket_path"`
	ImmuneEnabled    bool   `toml:"immune_enabled"`
	DefaultClientID  string `toml:"default_client_id"`
	DefaultSessionID string `toml:"default_session_id"`
	DefaultScope     string `toml:"default_capability_scope"`
}

type RiskDefaultsConfig struct {
	InitialState string `toml:"initial_state"`
}

type LoggingConfig struct {
	DBPath        string `toml:"db_path"`
	RetentionDays int    `toml:"retention_days"`
}

type NotificationsConfig struct {
	BudgetWarningPercent int `toml:"budget_warning_percent"`
}

type ProviderConfig struct {
	Upstream     string            `toml:"upstream"`
	ExtraHeaders map[string]string `toml:"extra_headers"`
}

type Config struct {
	Daemon        DaemonConfig              `toml:"daemon"`
	RiskDefaults  RiskDefaultsConfig        `toml:"risk_defaults"`
	Logging       LoggingConfig             `toml:"logging"`
	Notifications NotificationsConfig       `toml:"notifications"`
	Providers     map[string]ProviderConfig `toml:"providers"`
}

func DefaultProviders() map[string]ProviderConfig {
	return map[string]ProviderConfig{
		"anthropic": {
			Upstream: "https://api.anthropic.com",
			ExtraHeaders: map[string]string{
				"anthropic-version": "2023-06-01",
			},
		},
		"openai": {
			Upstream: "https://api.openai.com",
		},
		"google": {
			Upstream: "https://generativelanguage.googleapis.com",
		},
		"mistral": {
			Upstream: "https://api.mistral.ai",
		},
		"openrouter": {
			Upstream: "https://openrouter.ai/api",
		},
		"groq": {
			Upstream: "https://api.groq.com/openai",
		},
	}
}

func Default() Config {
	return Config{
		Daemon: DaemonConfig{
			Port:             DefaultPort,
			Host:             DefaultHost,
			SocketPath:       DefaultSocketPath,
			ImmuneEnabled:    true,
			DefaultClientID:  "tokfence-cli",
			DefaultSessionID: "default",
			DefaultScope:     "proxy",
		},
		RiskDefaults: RiskDefaultsConfig{
			InitialState: "GREEN",
		},
		Logging: LoggingConfig{
			DBPath:        "~/.tokfence/tokfence.db",
			RetentionDays: DefaultRetentionDays,
		},
		Notifications: NotificationsConfig{
			BudgetWarningPercent: 80,
		},
		Providers: DefaultProviders(),
	}
}

func DefaultConfigPath() string {
	return "~/.tokfence/config.toml"
}

func DataDir() string {
	return "~/.tokfence"
}

func DefaultSocketPathValue() string {
	return DefaultSocketPath
}

func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("path is empty")
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Clean(path), nil
}

func EnsureSecureDataDir() (string, error) {
	dir, err := ExpandPath(DataDir())
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", fmt.Errorf("set data dir perms: %w", err)
	}
	return dir, nil
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		path = DefaultConfigPath()
	}
	expanded, err := ExpandPath(path)
	if err != nil {
		return cfg, fmt.Errorf("expand config path: %w", err)
	}
	if _, err := os.Stat(expanded); err != nil {
		if os.IsNotExist(err) {
			cfg.Logging.DBPath, _ = ExpandPath(cfg.Logging.DBPath)
			return cfg, nil
		}
		return cfg, fmt.Errorf("stat config: %w", err)
	}

	loaded := Config{}
	if _, err := toml.DecodeFile(expanded, &loaded); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}

	if loaded.Daemon.Port != 0 {
		cfg.Daemon.Port = loaded.Daemon.Port
	}
	if loaded.Daemon.Host != "" {
		cfg.Daemon.Host = loaded.Daemon.Host
	}
	if loaded.Daemon.SocketPath != "" {
		cfg.Daemon.SocketPath = loaded.Daemon.SocketPath
	}
	if loaded.Daemon.DefaultClientID != "" {
		cfg.Daemon.DefaultClientID = loaded.Daemon.DefaultClientID
	}
	if loaded.Daemon.DefaultSessionID != "" {
		cfg.Daemon.DefaultSessionID = loaded.Daemon.DefaultSessionID
	}
	if loaded.Daemon.DefaultScope != "" {
		cfg.Daemon.DefaultScope = loaded.Daemon.DefaultScope
	}
	if loaded.Daemon.ImmuneEnabled != cfg.Daemon.ImmuneEnabled {
		cfg.Daemon.ImmuneEnabled = loaded.Daemon.ImmuneEnabled
	}
	if loaded.RiskDefaults.InitialState != "" {
		state, err := security.ParseRiskState(loaded.RiskDefaults.InitialState)
		if err != nil {
			return cfg, fmt.Errorf("invalid risk_defaults.initial_state: %w", err)
		}
		cfg.RiskDefaults = RiskDefaultsConfig{
			InitialState: string(state),
		}
	}
	if loaded.Logging.DBPath != "" {
		cfg.Logging.DBPath = loaded.Logging.DBPath
	}
	if loaded.Logging.RetentionDays != 0 {
		cfg.Logging.RetentionDays = loaded.Logging.RetentionDays
	}
	if loaded.Notifications.BudgetWarningPercent != 0 {
		cfg.Notifications.BudgetWarningPercent = loaded.Notifications.BudgetWarningPercent
	}

	if loaded.Providers != nil {
		for name, p := range loaded.Providers {
			base, ok := cfg.Providers[name]
			if !ok {
				base = ProviderConfig{}
			}
			if p.Upstream != "" {
				base.Upstream = p.Upstream
			}
			if p.ExtraHeaders != nil {
				if base.ExtraHeaders == nil {
					base.ExtraHeaders = map[string]string{}
				}
				for hk, hv := range p.ExtraHeaders {
					base.ExtraHeaders[hk] = hv
				}
			}
			cfg.Providers[name] = base
		}
	}

	cfg.Logging.DBPath, err = ExpandPath(cfg.Logging.DBPath)
	if err != nil {
		return cfg, fmt.Errorf("expand db path: %w", err)
	}

	if cfg.Daemon.Host == "" {
		cfg.Daemon.Host = DefaultHost
	}
	if cfg.Daemon.SocketPath == "" {
		cfg.Daemon.SocketPath = DefaultSocketPath
	}
	cfg.Daemon.SocketPath, err = ExpandPath(cfg.Daemon.SocketPath)
	if err != nil {
		return cfg, fmt.Errorf("expand socket path: %w", err)
	}
	if len([]byte(cfg.Daemon.SocketPath)) > maxUnixSocketPathLen {
		return cfg, fmt.Errorf("socket path too long: %d bytes, max %d", len([]byte(cfg.Daemon.SocketPath)), maxUnixSocketPathLen)
	}
	socketDir := filepath.Dir(cfg.Daemon.SocketPath)
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		return cfg, fmt.Errorf("create socket directory: %w", err)
	}
	if cfg.Daemon.Host == "0.0.0.0" {
		return cfg, errors.New("insecure daemon.host=0.0.0.0 is not allowed")
	}
	if cfg.Daemon.Port == 0 {
		cfg.Daemon.Port = DefaultPort
	}

	return cfg, nil
}

func Save(path string, cfg Config) error {
	if path == "" {
		path = DefaultConfigPath()
	}
	expanded, err := ExpandPath(path)
	if err != nil {
		return fmt.Errorf("expand config path: %w", err)
	}

	dir := filepath.Dir(expanded)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil && !os.IsPermission(err) {
		return fmt.Errorf("set config directory perms: %w", err)
	}

	tmpPath := expanded + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open temp config file: %w", err)
	}
	encodeErr := toml.NewEncoder(file).Encode(cfg)
	syncErr := file.Sync()
	closeErr := file.Close()
	if encodeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("encode config: %w", encodeErr)
	}
	if syncErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync temp config file: %w", syncErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp config file: %w", closeErr)
	}

	if err := os.Rename(tmpPath, expanded); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config file: %w", err)
	}
	if err := os.Chmod(expanded, 0o600); err != nil && !os.IsPermission(err) {
		return fmt.Errorf("set config file perms: %w", err)
	}
	return nil
}
