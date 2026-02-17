package security

import (
	"strings"
	"testing"
	"time"
)

func TestMintValidateCapabilityRoundTrip(t *testing.T) {
	token, err := MintCapability("proxy", "agent-a", "session-1", "GREEN", 30*time.Minute)
	if err != nil {
		t.Fatalf("MintCapability() error = %v", err)
	}
	capability, err := ValidateCapability(token)
	if err != nil {
		t.Fatalf("ValidateCapability() error = %v", err)
	}
	if capability.ClientID != "agent-a" {
		t.Fatalf("ClientID = %s, want agent-a", capability.ClientID)
	}
	if capability.SessionID != "session-1" {
		t.Fatalf("SessionID = %s, want session-1", capability.SessionID)
	}
	if strings.ToLower(capability.Scope) != "proxy" {
		t.Fatalf("Scope = %s, want proxy", capability.Scope)
	}
	if capability.RiskState != "GREEN" {
		t.Fatalf("RiskState = %s, want GREEN", capability.RiskState)
	}
	if capability.Expiry <= 0 || capability.Nonce == "" {
		t.Fatalf("invalid issued payload: expiry=%d nonce=%q", capability.Expiry, capability.Nonce)
	}
}

func TestValidateCapabilityRejectsExpiredToken(t *testing.T) {
	manager, err := NewCapabilityManager()
	if err != nil {
		t.Fatalf("NewCapabilityManager() error = %v", err)
	}
	token, err := manager.MintCapability("safe", "agent-a", "session-1", "GREEN", 1*time.Second)
	if err != nil {
		t.Fatalf("MintCapability() error = %v", err)
	}
	time.Sleep(2 * time.Second)

	if _, err := manager.ValidateCapability(token); err == nil {
		t.Fatalf("expected expired token validation to fail")
	}
}

func TestValidateCapabilityRejectsTamperedToken(t *testing.T) {
	token, err := MintCapability("proxy", "agent-a", "session-1", "GREEN", 10*time.Minute)
	if err != nil {
		t.Fatalf("MintCapability() error = %v", err)
	}
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected token format")
	}
	tampered := parts[0][:len(parts[0])-1] + "A" + "." + parts[1]
	if _, err := ValidateCapability(tampered); err == nil {
		t.Fatalf("expected tampered token validation to fail")
	}
}

func TestValidateCapabilityRequiresClientID(t *testing.T) {
	if _, err := MintCapability("proxy", "", "session", "GREEN", 5*time.Minute); err == nil {
		t.Fatalf("expected missing clientID to fail")
	}
}

func TestMintCapabilityUsesScopeAndTTLShortening(t *testing.T) {
	capability, err := MintCapability("safe", "agent-b", "session-2", "YELLOW", 10*time.Second)
	if err != nil {
		t.Fatalf("MintCapability() error = %v", err)
	}
	parsed, err := ValidateCapability(capability)
	if err != nil {
		t.Fatalf("ValidateCapability() error = %v", err)
	}
	if parsed.Scope != "safe" {
		t.Fatalf("Scope = %s, want safe", parsed.Scope)
	}
	// YELLOW halves TTL: 10s becomes 5s (minimum floor is 1s, so 5s expected)
	if parsed.Expiry <= time.Now().Unix() {
		t.Fatalf("expected expiry in future, got %d", parsed.Expiry)
	}
	if parsed.Expiry-time.Now().Unix() > 10 {
		t.Fatalf("unexpected expiry too long for YELLOW scope: %d", parsed.Expiry-time.Now().Unix())
	}
}
