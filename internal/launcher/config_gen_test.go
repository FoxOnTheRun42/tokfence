package launcher

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/FoxOnTheRun42/tokfence/internal/config"
)

func TestGenerateConfig_SingleProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = map[string]config.ProviderConfig{
		"anthropic": cfg.Providers["anthropic"],
	}

	out, token, err := GenerateOpenClawConfig([]string{"anthropic"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	if len(token) != 48 {
		t.Fatalf("gateway token should be 48 chars, got %d", len(token))
	}
	for _, c := range token {
		if c < '0' || (c > '9' && c < 'a') || c > 'f' {
			t.Fatalf("gateway token is not lowercase hex: %q", token)
		}
	}

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}

	models, err := nestedMap(parsed, "models")
	if err != nil {
		t.Fatalf("models node missing: %v", err)
	}
	providersNode, ok := models["providers"].(map[string]any)
	if !ok {
		t.Fatalf("models.providers not map[string]any")
	}
	if _, ok := providersNode["anthropic"]; !ok {
		t.Fatalf("missing anthropic provider config")
	}
	anthro := providersNode["anthropic"].(map[string]any)
	if anthro["apiKey"] != dummyAPIKey {
		t.Fatalf("apiKey = %v, want %q", anthro["apiKey"], dummyAPIKey)
	}
	if strings.Contains(token, "-") {
		t.Fatalf("token has dashes: %q", token)
	}

	agents, _ := nestedMap(parsed, "agents")
	defaults, _ := nestedMap(agents, "defaults")
	model, _ := nestedMap(defaults, "model")
	if got, ok := model["primary"].(string); !ok || got == "" {
		t.Fatalf("missing primary model")
	} else if got != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("primary=%q, want anthropic model", got)
	}
	if fallbacks, ok := model["fallbacks"].([]any); !ok || len(fallbacks) != 0 {
		t.Fatalf("fallbacks=%v expected 0", fallbacks)
	}

	baseURL, ok := anthro["baseUrl"].(string)
	if !ok {
		t.Fatalf("baseUrl missing")
	}
	host := proxyHost()
	if !strings.Contains(baseURL, host) {
		t.Fatalf("baseUrl=%q missing host %q", baseURL, host)
	}
	if !strings.Contains(baseURL, ":9471/anthropic") {
		t.Fatalf("baseUrl=%q does not include proxy port and provider", baseURL)
	}
}

func TestGenerateConfig_MultipleProviders(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"openai", "anthropic", "groq"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}

	models, err := nestedMap(parsed, "agents")
	if err != nil {
		t.Fatalf("agents missing: %v", err)
	}
	defaults, _ := nestedMap(models, "defaults")
	model, _ := nestedMap(defaults, "model")
	if got := model["primary"]; got != "anthropic/claude-sonnet-4-5" {
		t.Fatalf("primary=%v, want anthropic", got)
	}

	fallbacks, ok := model["fallbacks"].([]any)
	if !ok {
		t.Fatalf("fallbacks not array")
	}
	if len(fallbacks) != 2 {
		t.Fatalf("fallback len=%d want 2", len(fallbacks))
	}
	if got := fallbacks[0]; got != "openai/gpt-5.1" {
		t.Fatalf("fallback0=%v want openai/gpt-5.1", got)
	}
	if got := fallbacks[1]; got != "groq/llama-4-scout-17b-16e-instruct" {
		t.Fatalf("fallback1=%v want groq model", got)
	}
}

func TestGenerateConfig_UnknownProviderIgnored(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"anthropic", "some-custom-thing"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}
	models, _ := nestedMap(parsed, "models")
	providers, ok := models["providers"].(map[string]any)
	if !ok {
		t.Fatalf("models.providers not map")
	}
	if _, ok := providers["anthropic"]; !ok {
		t.Fatalf("anthropic missing")
	}
	if _, ok := providers["some-custom-thing"]; ok {
		t.Fatalf("custom provider should be ignored")
	}
}

func TestGenerateConfig_WizardBypass(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"anthropic"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}
	wizard, err := nestedMap(parsed, "wizard")
	if err != nil {
		t.Fatalf("wizard missing: %v", err)
	}
	if wizard["lastRunCommand"] != "onboard" {
		t.Fatalf("wizard.lastRunCommand=%v want onboard", wizard["lastRunCommand"])
	}
	if _, ok := wizard["lastRunAt"].(string); !ok || wizard["lastRunAt"] == "" {
		t.Fatalf("wizard.lastRunAt empty")
	}
}

func TestGenerateConfig_GatewayToken(t *testing.T) {
	cfg := config.Default()
	_, token1, err := GenerateOpenClawConfig([]string{"anthropic"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	_, token2, err := GenerateOpenClawConfig([]string{"anthropic"}, cfg)
	if err != nil {
		t.Fatalf("generate config 2: %v", err)
	}
	if len(token1) != 48 || len(token2) != 48 {
		t.Fatalf("token length invalid: %q %q", token1, token2)
	}
	if token1 == token2 {
		t.Fatalf("expected different gateway tokens")
	}
}

func TestGenerateConfig_NoDoubleSlashInBaseUrl(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"anthropic", "openai"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}
	models, _ := nestedMap(parsed, "models")
	providers := models["providers"].(map[string]any)
	for raw, item := range providers {
		providerConfig := item.(map[string]any)
		base := providerConfig["baseUrl"].(string)
		s := strings.ToLower(raw)
		if _, ok := providerConfig["baseUrl"].(string); !ok {
			t.Fatalf("%s: baseUrl missing", raw)
		}
		head := strings.SplitN(base, "://", 2)
		if len(head) == 2 && strings.Contains(head[1], "//") {
			t.Fatalf("baseUrl %q contains double slash path", base)
		}
		if strings.Contains(base, "//"+s) {
			t.Fatalf("baseUrl %q contains //%s", base, s)
		}
	}
}

func TestGenerateConfig_LinuxHost(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"anthropic"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}
	models, _ := nestedMap(parsed, "models")
	providers := models["providers"].(map[string]any)
	cfgProvider, ok := providers["anthropic"].(map[string]any)
	if !ok {
		t.Fatalf("anthropic missing")
	}
	baseURL, ok := cfgProvider["baseUrl"].(string)
	if !ok {
		t.Fatalf("baseUrl missing")
	}

	expected := "host.docker.internal"
	if runtime.GOOS == "linux" {
		expected = "172.17.0.1"
	}
	if !strings.Contains(baseURL, expected) {
		t.Fatalf("baseUrl %q expected host %q", baseURL, expected)
	}
}

func TestGenerateConfig_ControlUIEnabled(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"anthropic"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}
	gw, err := nestedMap(parsed, "gateway")
	if err != nil {
		t.Fatalf("gateway missing: %v", err)
	}
	controlUI, err := nestedMap(gw, "controlUi")
	if err != nil {
		t.Fatalf("gateway.controlUi missing: %v", err)
	}
	if enabled, ok := controlUI["enabled"].(bool); !ok || !enabled {
		t.Fatalf("controlUi.enabled=%v, want true", controlUI["enabled"])
	}
	if insecure, ok := controlUI["allowInsecureAuth"].(bool); !ok || !insecure {
		t.Fatalf("controlUi.allowInsecureAuth=%v, want true", controlUI["allowInsecureAuth"])
	}
}

func TestGenerateConfig_DummyKeyNeverReal(t *testing.T) {
	cfg := config.Default()
	out, _, err := GenerateOpenClawConfig([]string{"anthropic", "openai", "google"}, cfg)
	if err != nil {
		t.Fatalf("generate config: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("valid json: %v", err)
	}

	models, _ := nestedMap(parsed, "models")
	if err := inspectStringsForLeakPatterns(models, []string{"sk-ant-", "sk-", "gsk_", "AIza"}, func(value string, pattern string) {
		t.Fatalf("found disallowed pattern %q in generated JSON: %q", pattern, value)
	}); err != nil {
		t.Fatalf("inspect: %v", err)
	}
}

func nestedMap(v map[string]any, key string) (map[string]any, error) {
	raw, ok := v[key]
	if !ok {
		return nil, &testErr{msg: "missing key " + key}
	}
	out, ok := raw.(map[string]any)
	if !ok {
		return nil, &testErr{msg: "wrong type for key " + key}
	}
	return out, nil
}

func inspectStringsForLeakPatterns(v any, patterns []string, onMatch func(string, string)) error {
	switch node := v.(type) {
	case map[string]any:
		for _, child := range node {
			if err := inspectStringsForLeakPatterns(child, patterns, onMatch); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := inspectStringsForLeakPatterns(child, patterns, onMatch); err != nil {
				return err
			}
		}
	case string:
		for _, pattern := range patterns {
			if strings.Contains(node, pattern) {
				onMatch(node, pattern)
			}
		}
	case nil:
		return nil
	default:
		// only traverse complex types and strings
	}
	return nil
}

type testErr struct {
	msg string
}

func (e *testErr) Error() string { return e.msg }
