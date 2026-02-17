package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FoxOnTheRun42/tokfence/internal/config"
)

func withFakePSOutput(t *testing.T, output string) {
	t.Helper()

	tmpDir := t.TempDir()
	psPath := filepath.Join(tmpDir, "ps")

	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\n' \"$TOKFENCE_PS_OUTPUT\"\n")
	if err := os.WriteFile(psPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TOKFENCE_PS_OUTPUT", output)
	t.Setenv("PATH", fmt.Sprintf("%s:%s", tmpDir, os.Getenv("PATH")))
}

func TestProcessCommandContainsNonce(t *testing.T) {
	t.Run("empty nonce bypasses check", func(t *testing.T) {
		withFakePSOutput(t, "/bin/does-not-matter --foo=bar")
		if !processCommandContainsNonce(1234, "") {
			t.Fatalf("expected empty nonce to pass")
		}
	})

	t.Run("matches token in environment form", func(t *testing.T) {
		withFakePSOutput(t, "/usr/bin/tokfence TOKFENCE_DAEMON_NONCE=abc123 --flag=value")
		if !processCommandContainsNonce(1234, "abc123") {
			t.Fatalf("expected process command to match nonce in env variable")
		}
	})

	t.Run("matches token in separate argument", func(t *testing.T) {
		withFakePSOutput(t, "/usr/bin/tokfence --tokfence-daemon-nonce abc123 --flag=value")
		if !processCommandContainsNonce(1234, "abc123") {
			t.Fatalf("expected process command to match nonce argument")
		}
	})

	t.Run("matches token in argument form", func(t *testing.T) {
		withFakePSOutput(t, "/usr/bin/tokfence --tokfence-daemon-nonce=abc123 --flag=value")
		if !processCommandContainsNonce(1234, "abc123") {
			t.Fatalf("expected process command to match inline nonce argument")
		}
	})

	t.Run("does not match missing token", func(t *testing.T) {
		withFakePSOutput(t, "/usr/bin/tokfence --tokfence-daemon-nonce=wrong --flag=value")
		if processCommandContainsNonce(1234, "abc123") {
			t.Fatalf("expected process command to reject mismatched nonce")
		}
	})
}

func TestVerifyDaemonProcessRejectsNonceMismatch(t *testing.T) {
	parentShell := os.Getenv("SHELL")
	if parentShell == "" {
		parentShell = "/bin/sh"
	}

	withFakePSOutput(t, parentShell+" --tokfence-daemon-nonce=wrong")
	state := daemonState{
		PID:      os.Getpid(),
		UID:      os.Getuid(),
		Binary:   parentShell,
		CmdNonce: "expected-token",
	}

	err := verifyDaemonProcess(state)
	if err == nil {
		t.Fatalf("expected nonce mismatch error")
	}
	if !strings.Contains(err.Error(), "nonce mismatch") {
		t.Fatalf("expected nonce mismatch error, got %v", err)
	}

	withFakePSOutput(t, parentShell+" --tokfence-daemon-nonce=expected-token")
	if err := verifyDaemonProcess(state); err != nil {
		t.Fatalf("expected nonce match to be accepted, got %v", err)
	}
}

type fakeVault struct {
	providers []string
}

func (f fakeVault) Get(context.Context, string) (string, error) { return "", nil }
func (f fakeVault) Set(context.Context, string, string) error   { return nil }
func (f fakeVault) Delete(context.Context, string) error        { return nil }
func (f fakeVault) List(context.Context) ([]string, error) {
	return append([]string{}, f.providers...), nil
}

func TestParseRemoteUsageTotalsOpenAIDashboard(t *testing.T) {
	payload := []byte(`{"total_usage":1234}`)
	usage, err := parseRemoteUsageTotals(payload)
	if err != nil {
		t.Fatalf("parse usage failed: %v", err)
	}
	if !usage.CostKnown {
		t.Fatalf("expected cost to be known")
	}
	if usage.CostCents != 1234 {
		t.Fatalf("cost cents = %d, want 1234", usage.CostCents)
	}
	if usage.CostUSD != "$12.34" {
		t.Fatalf("cost usd = %q, want $12.34", usage.CostUSD)
	}
}

func TestParseRemoteUsageTotalsTokenAndCostSum(t *testing.T) {
	payload := []byte(`{
		"data":[
			{"input_tokens":100,"output_tokens":20,"cost_usd":0.5,"requests":2},
			{"input_tokens":70,"completion_tokens":10,"cost_usd":0.3,"requests":1}
		]
	}`)
	usage, err := parseRemoteUsageTotals(payload)
	if err != nil {
		t.Fatalf("parse usage failed: %v", err)
	}
	if usage.InputTokens != 170 {
		t.Fatalf("input tokens = %d, want 170", usage.InputTokens)
	}
	if usage.OutputTokens != 30 {
		t.Fatalf("output tokens = %d, want 30", usage.OutputTokens)
	}
	if usage.RequestCount != 3 {
		t.Fatalf("request count = %d, want 3", usage.RequestCount)
	}
	if usage.CostCents != 80 {
		t.Fatalf("cost cents = %d, want 80", usage.CostCents)
	}
}

func TestParseCustomUsageEndpointFlags(t *testing.T) {
	endpoints, err := parseCustomUsageEndpointFlags([]string{
		"openai=https://example.com/usage",
		"anthropic=https://api.anthropic.com/v1/usage",
	})
	if err != nil {
		t.Fatalf("parse custom usage endpoints failed: %v", err)
	}
	if endpoints["openai"] != "https://example.com/usage" {
		t.Fatalf("openai endpoint mismatch: %q", endpoints["openai"])
	}
	if endpoints["anthropic"] != "https://api.anthropic.com/v1/usage" {
		t.Fatalf("anthropic endpoint mismatch: %q", endpoints["anthropic"])
	}
}

func TestResolveWatchProvidersFromVault(t *testing.T) {
	cfg := config.Default()
	providers, err := resolveWatchProviders(nil, cfg, fakeVault{providers: []string{"openai", "anthropic"}})
	if err != nil {
		t.Fatalf("resolve providers failed: %v", err)
	}
	if len(providers) != 2 {
		t.Fatalf("provider count = %d, want 2", len(providers))
	}
	if providers[0] != "anthropic" || providers[1] != "openai" {
		t.Fatalf("unexpected provider order: %v", providers)
	}
}
