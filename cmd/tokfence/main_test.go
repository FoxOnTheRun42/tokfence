package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
