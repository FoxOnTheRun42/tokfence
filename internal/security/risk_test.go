package security

import "testing"

func TestRiskMachineEscalationMonotonic(t *testing.T) {
	machine := NewRiskMachine(RiskDefaults{InitialState: "GREEN"})

	if machine.State() != RiskGreen {
		t.Fatalf("initial state = %s, want GREEN", machine.State())
	}

	machine.Escalate(RiskEventSecretLeak)
	if machine.State() != RiskYellow {
		t.Fatalf("after secret leak = %s, want YELLOW", machine.State())
	}

	machine.Escalate(RiskEventCanaryLeak)
	if machine.State() != RiskRed {
		t.Fatalf("after canary leak = %s, want RED", machine.State())
	}

	machine.Escalate(RiskEventEndpoint)
	if machine.State() != RiskRed {
		t.Fatalf("after endpoint during RED = %s, want RED", machine.State())
	}
}

func TestRiskMachineSafeRoutePolicies(t *testing.T) {
	machine := NewRiskMachine(RiskDefaults{InitialState: "GREEN"})
	cases := []struct {
		state  RiskState
		method string
		path   string
		scope  string
		want   bool
	}{
		{RiskGreen, "POST", "/v1/messages", "proxy", true},
		{RiskYellow, "POST", "/v1/messages", "proxy", false},
		{RiskYellow, "GET", "/v1/models", "safe", true},
		{RiskOrange, "GET", "/v1/models", "proxy", true},
		{RiskOrange, "POST", "/v1/messages", "safe", false},
		{RiskRed, "GET", "/v1/models", "proxy", false},
	}
	for _, tc := range cases {
		if got := machine.IsRequestAllowedForState(tc.state, tc.scope, tc.method, tc.path); got != tc.want {
			t.Fatalf("state=%s method=%s path=%s scope=%s got=%v want=%v", tc.state, tc.method, tc.path, tc.scope, got, tc.want)
		}
	}
}
