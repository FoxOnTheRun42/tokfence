package security

import "testing"

func TestCanaryLeakEscalatesRiskToRed(t *testing.T) {
	machine := NewRiskMachine(RiskDefaults{InitialState: "GREEN"})
	if machine.State() != RiskGreen {
		t.Fatalf("initial state = %s, want GREEN", machine.State())
	}

	canary := "tokfence-canary-4f2c8ab8a"
	if DetectCanaryLeak([]byte("normal response"), canary) {
		t.Fatalf("expected canary not found in normal response")
	}
	if !DetectCanaryLeak([]byte(canary), canary) {
		t.Fatalf("expected canary found in response")
	}

	machine.Escalate(RiskEventCanaryLeak)
	if machine.State() != RiskRed {
		t.Fatalf("state after canary leak = %s, want RED", machine.State())
	}
}
