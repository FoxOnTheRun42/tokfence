package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/macfox/tokfence/internal/config"
	"github.com/macfox/tokfence/internal/vault"
)

type LaunchConfig struct {
	ContainerName string
	Image         string
	StateDir      string
	WorkspaceDir  string
	GatewayPort   int
	Pull          bool
	OpenBrowser   bool
}

type LaunchResult struct {
	ContainerID  string   `json:"container_id"`
	GatewayURL   string   `json:"gateway_url"`
	GatewayToken string   `json:"gateway_token"`
	DashboardURL string   `json:"dashboard_url"`
	Providers    []string `json:"providers"`
	PrimaryModel string   `json:"primary_model"`
	ConfigPath   string   `json:"config_path"`
	Status       string   `json:"status"`
}

type Launcher struct {
	Config LaunchConfig
	TokCfg config.Config
	Vault  vault.Vault
	Stdout io.Writer
}

type LaunchAlreadyRunningError struct {
	ContainerName string
}

func (e *LaunchAlreadyRunningError) Error() string {
	return fmt.Sprintf("container %q is already running", e.ContainerName)
}

func DefaultLaunchConfig() LaunchConfig {
	return LaunchConfig{
		ContainerName: "tokfence-openclaw",
		Image:         "ghcr.io/openclaw/openclaw:latest",
		StateDir:      "~/.tokfence/openclaw",
		WorkspaceDir:  "~/openclaw/workspace",
		GatewayPort:   18789,
		Pull:          true,
		OpenBrowser:   true,
	}
}

func (l *Launcher) Preflight(ctx context.Context) []error {
	out := l.Stdout
	failures := []error{}
	mark := func(msg string) {
		if out != nil {
			fmt.Fprintln(out, msg)
		}
	}
	fail := func(err error) {
		failures = append(failures, err)
	}

	if err := DockerAvailable(ctx); err != nil {
		fail(err)
	} else {
		mark("✓ Docker is running")
	}

	daemonAddr := net.JoinHostPort(l.TokCfg.Daemon.Host, fmt.Sprintf("%d", l.TokCfg.Daemon.Port))
	conn, err := net.DialTimeout("tcp", daemonAddr, 2*time.Second)
	if err != nil {
		fail(fmt.Errorf("tokfence daemon unreachable on %s", daemonAddr))
	} else {
		_ = conn.Close()
		mark(fmt.Sprintf("✓ Tokfence daemon is running on %s", daemonAddr))
	}

	vaultProviders, err := l.Vault.List(ctx)
	if err != nil {
		fail(fmt.Errorf("list vault providers: %w", err))
	} else if len(vaultProviders) == 0 {
		fail(errors.New("no API keys configured"))
	} else {
		sort.Strings(vaultProviders)
		mark(fmt.Sprintf("✓ Found %d API keys: %s", len(vaultProviders), strings.Join(vaultProviders, ", ")))
	}

	status, statusErr := ContainerStatus(ctx, l.Config.ContainerName)
	if statusErr != nil {
		fail(statusErr)
	} else {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "running":
			fail(&LaunchAlreadyRunningError{ContainerName: l.Config.ContainerName})
		case "exited", "created":
			if err := StopAndRemoveContainer(ctx, l.Config.ContainerName); err != nil {
				fail(err)
			}
		}
	}

	if strings.ToLower(strings.TrimSpace(status)) != "running" {
		if IsPortAvailable(l.Config.GatewayPort) {
			mark(fmt.Sprintf("✓ Port %d is available", l.Config.GatewayPort))
		} else {
			fail(fmt.Errorf("port %d is not available", l.Config.GatewayPort))
		}
	}

	return failures
}

func (l *Launcher) Launch(ctx context.Context) (*LaunchResult, error) {
	preflight := l.Preflight(ctx)
	running, others := splitPreflightErrors(preflight)
	if len(others) > 0 {
		return nil, errors.Join(others...)
	}
	if running != nil {
		result, statusErr := l.Status(ctx)
		if statusErr != nil {
			return nil, running
		}
		if result == nil {
			return nil, running
		}
		return result, nil
	}

	stateDir, err := config.ExpandPath(l.Config.StateDir)
	if err != nil {
		return nil, fmt.Errorf("expand state dir: %w", err)
	}
	workspaceDir, err := config.ExpandPath(l.Config.WorkspaceDir)
	if err != nil {
		return nil, fmt.Errorf("expand workspace dir: %w", err)
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}

	providers, err := l.Vault.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vault providers: %w", err)
	}
	jsonConfig, gatewayToken, err := GenerateOpenClawConfig(providers, l.TokCfg)
	if err != nil {
		return nil, fmt.Errorf("generate openclaw config: %w", err)
	}

	configPath := filepath.Join(stateDir, "openclaw.json")
	if err := os.WriteFile(configPath, jsonConfig, 0o600); err != nil {
		return nil, fmt.Errorf("write openclaw config: %w", err)
	}
	if err := os.Chmod(configPath, 0o600); err != nil && !os.IsPermission(err) {
		return nil, fmt.Errorf("set openclaw config mode: %w", err)
	}

	var cfg openClawConfig
	if err := json.Unmarshal(jsonConfig, &cfg); err != nil {
		return nil, fmt.Errorf("parse generated config: %w", err)
	}

	if l.Stdout != nil {
		fmt.Fprintf(l.Stdout, "✓ Generated OpenClaw config (primary: %s)\n", cfg.Agents.Defaults.Model.Primary)
	}

	if l.Config.Pull {
		if l.Stdout != nil {
			fmt.Fprintf(l.Stdout, "⠋ Pulling %s...\n", l.Config.Image)
		}
		if err := PullImage(ctx, l.Config.Image, l.Stdout); err != nil {
			return nil, err
		}
		if l.Stdout != nil {
			fmt.Fprintln(l.Stdout, "✓ Image ready")
		}
	}

	containerID, err := RunContainer(ctx, ContainerOpts{
		Name:  l.Config.ContainerName,
		Image: l.Config.Image,
		Volumes: []string{
			stateDir + ":/home/node/.openclaw",
			workspaceDir + ":/home/node/.openclaw/workspace",
		},
		Ports: []string{
			fmt.Sprintf("%d:18789", l.Config.GatewayPort),
		},
		ExtraHosts: []string{
			"host.docker.internal:host-gateway",
		},
		Restart: "unless-stopped",
	})
	if err != nil {
		return nil, fmt.Errorf("start openclaw container: %w", err)
	}

	if err := waitForContainer(ctx, l.Config.ContainerName, 30*time.Second); err != nil {
		_ = StopAndRemoveContainer(ctx, l.Config.ContainerName)
		return nil, err
	}

	gatewayURL := fmt.Sprintf("http://127.0.0.1:%d", l.Config.GatewayPort)
	if err := waitForTCP(gatewayURL, 30*time.Second); err != nil {
		_ = StopAndRemoveContainer(ctx, l.Config.ContainerName)
		return nil, err
	}

	orderedProviders := sortProvidersByPriority(cfg.Models.Providers)
	if l.Stdout != nil {
		fmt.Fprintf(l.Stdout, "✓ Container %s started\n", l.Config.ContainerName)
		fmt.Fprintf(l.Stdout, "✓ Gateway ready at %s\n\n", gatewayURL)
		fmt.Fprintln(l.Stdout, "OpenClaw is running. All API traffic flows through Tokfence.")
		fmt.Fprintln(l.Stdout, "No API keys are stored in the container.")
		fmt.Fprintf(l.Stdout, "Dashboard: %s/?token=%s\n", gatewayURL, cfg.Gateway.Auth.Token)
		fmt.Fprintln(l.Stdout, "Logs:      tokfence launch logs -f")
		fmt.Fprintln(l.Stdout, "Stop:      tokfence launch stop")
	}

	return &LaunchResult{
		ContainerID:  containerID,
		GatewayURL:   gatewayURL,
		GatewayToken: gatewayToken,
		DashboardURL: fmt.Sprintf("%s/?token=%s", gatewayURL, cfg.Gateway.Auth.Token),
		Providers:    orderedProviders,
		PrimaryModel: cfg.Agents.Defaults.Model.Primary,
		ConfigPath:   configPath,
		Status:       "running",
	}, nil
}

func (l *Launcher) Stop(ctx context.Context) error {
	return StopAndRemoveContainer(ctx, l.Config.ContainerName)
}

func (l *Launcher) Status(ctx context.Context) (*LaunchResult, error) {
	status, err := ContainerStatus(ctx, l.Config.ContainerName)
	if err != nil {
		return nil, err
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "stopped"
	}

	stateDir, err := config.ExpandPath(l.Config.StateDir)
	if err != nil {
		return nil, fmt.Errorf("expand state dir: %w", err)
	}
	result := &LaunchResult{
		Status:     status,
		ConfigPath: filepath.Join(stateDir, "openclaw.json"),
		GatewayURL: fmt.Sprintf("http://127.0.0.1:%d", l.Config.GatewayPort),
	}
	if status != "running" {
		return result, nil
	}

	raw, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return nil, fmt.Errorf("read openclaw config: %w", err)
	}

	var cfg openClawConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse openclaw config: %w", err)
	}
	result.GatewayToken = cfg.Gateway.Auth.Token
	result.DashboardURL = fmt.Sprintf("%s/?token=%s", result.GatewayURL, cfg.Gateway.Auth.Token)
	for provider := range cfg.Models.Providers {
		result.Providers = append(result.Providers, provider)
	}
	result.Providers = sortProvidersByPriority(cfg.Models.Providers)
	result.PrimaryModel = cfg.Agents.Defaults.Model.Primary

	return result, nil
}

func (l *Launcher) Logs(ctx context.Context, follow bool) error {
	return ContainerLogs(ctx, l.Config.ContainerName, follow, l.Stdout)
}

func splitPreflightErrors(errs []error) (*LaunchAlreadyRunningError, []error) {
	var running *LaunchAlreadyRunningError
	others := make([]error, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		var runningErr *LaunchAlreadyRunningError
		if errors.As(err, &runningErr) {
			running = runningErr
			continue
		}
		others = append(others, err)
	}
	return running, others
}

func waitForContainer(ctx context.Context, name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := ContainerStatus(ctx, name)
		if err != nil {
			return fmt.Errorf("docker inspect %s: %w", name, err)
		}

		switch strings.ToLower(strings.TrimSpace(status)) {
		case "running":
			return nil
		case "":
			// container not found
			return fmt.Errorf("container %q exited before start", name)
		case "created", "restarting", "starting":
			// continue waiting
		case "exited", "dead":
			return fmt.Errorf("container %s failed to start (status %q)", name, status)
		default:
			return fmt.Errorf("container %s in unexpected state %q", name, status)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("container did not start within %s", timeout)
		}
		time.Sleep(time.Second)
	}
}

func waitForTCP(rawURL string, timeout time.Duration) error {
	addr := strings.TrimPrefix(rawURL, "https://")
	addr = strings.TrimPrefix(addr, "http://")
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("gateway %s not reachable within %s", rawURL, timeout)
		}
		time.Sleep(time.Second)
	}
}

func sortProvidersByPriority(providers map[string]openClawProviderConfig) []string {
	ordered := make([]string, 0, len(providers))
	for provider := range providers {
		ordered = append(ordered, provider)
	}
	sort.Slice(ordered, func(i, j int) bool {
		pi, iOK := providerModels[ordered[i]]
		pj, jOK := providerModels[ordered[j]]
		if iOK && jOK {
			return pi.Priority < pj.Priority
		}
		if !iOK && jOK {
			return false
		}
		if iOK && !jOK {
			return true
		}
		return ordered[i] < ordered[j]
	})
	return ordered
}
