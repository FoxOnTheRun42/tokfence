package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultPort          = 9471
	DefaultHost          = "127.0.0.1"
	DefaultRetentionDays = 90
)

type DaemonConfig struct {
	Port int    `toml:"port"`
	Host string `toml:"host"`
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
			Port: DefaultPort,
			Host: DefaultHost,
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
