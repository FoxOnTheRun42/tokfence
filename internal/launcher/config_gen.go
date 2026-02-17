package launcher

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/FoxOnTheRun42/tokfence/internal/config"
)

const openclawVersion = "2026.2.14"
const dummyAPIKey = "tokfence-managed"
const openclawGatewayPort = 18789

var providerModels = map[string]struct {
	ModelID  string
	Priority int
}{
	"anthropic":  {"anthropic/claude-sonnet-4-5", 1},
	"openai":     {"openai/gpt-5.1", 2},
	"google":     {"google/gemini-2.5-flash", 3},
	"openrouter": {"openrouter/auto", 4},
	"groq":       {"groq/llama-4-scout-17b-16e-instruct", 5},
	"mistral":    {"mistral/mistral-large-latest", 6},
}

type openClawConfig struct {
	Meta     openClawMeta     `json:"meta"`
	Wizard   openClawWizard   `json:"wizard"`
	Gateway  openClawGateway  `json:"gateway"`
	Models   openClawModels   `json:"models"`
	Agents   openClawAgents   `json:"agents"`
	Messages openClawMessages `json:"messages"`
	Commands openClawCommands `json:"commands"`
}

type openClawMeta struct {
	LastTouchedVersion string `json:"lastTouchedVersion"`
	LastTouchedAt      string `json:"lastTouchedAt"`
}

type openClawWizard struct {
	LastRunAt      string `json:"lastRunAt"`
	LastRunVersion string `json:"lastRunVersion"`
	LastRunCommand string `json:"lastRunCommand"`
	LastRunMode    string `json:"lastRunMode"`
}

type openClawGateway struct {
	Port      int                    `json:"port"`
	Mode      string                 `json:"mode"`
	Bind      string                 `json:"bind"`
	Auth      openClawGatewayAuth    `json:"auth"`
	ControlUI openClawControlUI      `json:"controlUi"`
}

type openClawControlUI struct {
	Enabled           bool `json:"enabled"`
	AllowInsecureAuth bool `json:"allowInsecureAuth"`
}

type openClawGatewayAuth struct {
	Mode  string `json:"mode"`
	Token string `json:"token"`
}

type openClawModels struct {
	Mode      string                            `json:"mode"`
	Providers map[string]openClawProviderConfig `json:"providers"`
}

type openClawProviderConfig struct {
	BaseUrl string   `json:"baseUrl"`
	APIKey  string   `json:"apiKey"`
	Models  []string `json:"models"`
}

type openClawAgents struct {
	Defaults openClawAgentDefaults `json:"defaults"`
}

type openClawAgentDefaults struct {
	Model     openClawAgentModel `json:"model"`
	Workspace string             `json:"workspace"`
}

type openClawAgentModel struct {
	Primary   string   `json:"primary"`
	Fallbacks []string `json:"fallbacks"`
}

type openClawMessages struct {
	AckReactionScope string `json:"ackReactionScope"`
}

type openClawCommands struct {
	Native       string `json:"native"`
	NativeSkills string `json:"nativeSkills"`
}

func GenerateOpenClawConfig(vaultProviders []string, tokCfg config.Config) ([]byte, string, error) {
	supported, err := resolveSupportedVaultProviders(vaultProviders, tokCfg)
	if err != nil {
		return nil, "", err
	}
	if len(supported) == 0 {
		return nil, "", errors.New("no supported providers found in vault")
	}

	token, err := randomToken()
	if err != nil {
		return nil, "", err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	cfg := openClawConfig{
		Meta: openClawMeta{
			LastTouchedVersion: openclawVersion,
			LastTouchedAt:      now,
		},
		Wizard: openClawWizard{
			LastRunAt:      now,
			LastRunVersion: openclawVersion,
			LastRunCommand: "onboard",
			LastRunMode:    "local",
		},
		Gateway: openClawGateway{
			Port: openclawGatewayPort,
			Mode: "local",
			Bind: "loopback",
			Auth: openClawGatewayAuth{
				Mode:  "token",
				Token: token,
			},
			ControlUI: openClawControlUI{
				Enabled:           true,
				AllowInsecureAuth: true,
			},
		},
		Models: openClawModels{
			Mode:      "merge",
			Providers: map[string]openClawProviderConfig{},
		},
		Agents: openClawAgents{
			Defaults: openClawAgentDefaults{
				Model:     openClawAgentModel{},
				Workspace: "/home/node/.openclaw/workspace",
			},
		},
		Messages: openClawMessages{
			AckReactionScope: "group-mentions",
		},
		Commands: openClawCommands{
			Native:       "auto",
			NativeSkills: "auto",
		},
	}

	primaryProvider := supported[0]
	primaryModel, fallbackModels := modelPriority(primaryProvider, supported)
	cfg.Agents.Defaults.Model.Primary = primaryModel
	cfg.Agents.Defaults.Model.Fallbacks = append([]string{}, fallbackModels...)

	for _, provider := range supported {
		cfg.Models.Providers[provider] = openClawProviderConfig{
			BaseUrl: fmt.Sprintf("http://%s:%d/%s", proxyHost(), tokCfg.Daemon.Port, provider),
			APIKey:  dummyAPIKey,
			Models:  []string{},
		}
	}

	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, "", err
	}

	var validate map[string]any
	if err := json.Unmarshal(encoded, &validate); err != nil {
		return nil, "", fmt.Errorf("validate generated config: %w", err)
	}

	return encoded, token, nil
}

func proxyHost() string {
	if runtime.GOOS == "linux" {
		return "172.17.0.1"
	}
	return "host.docker.internal"
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func resolveSupportedVaultProviders(vaultProviders []string, tokCfg config.Config) ([]string, error) {
	seen := map[string]struct{}{}
	filtered := make([]string, 0, len(vaultProviders))
	for _, raw := range vaultProviders {
		provider := strings.ToLower(strings.TrimSpace(raw))
		if provider == "" {
			continue
		}
		if _, ok := providerModels[provider]; !ok {
			continue
		}
		if _, ok := tokCfg.Providers[provider]; !ok {
			continue
		}
		if _, ok := seen[provider]; ok {
			continue
		}
		filtered = append(filtered, provider)
		seen[provider] = struct{}{}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return providerModels[filtered[i]].Priority < providerModels[filtered[j]].Priority
	})
	return filtered, nil
}

func modelPriority(primary string, providers []string) (string, []string) {
	primaryModel := ""
	fallbacks := make([]string, 0, len(providers))
	for _, provider := range providers {
		model := providerModels[provider].ModelID
		if provider == primary {
			primaryModel = model
			continue
		}
		fallbacks = append(fallbacks, model)
	}
	return primaryModel, fallbacks
}
