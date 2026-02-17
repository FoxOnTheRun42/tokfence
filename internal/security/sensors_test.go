package security

import "testing"

func TestDetectSecretReference(t *testing.T) {
	if !DetectSecretReference("OPENAI_API_KEY=sk-ant-1234567890123456") {
		t.Fatalf("expected secret-like pattern to be detected")
	}
	if DetectSecretReference("no secret content") {
		t.Fatalf("unexpected secret detection")
	}
}

func TestDetectDisallowedEndpoint(t *testing.T) {
	if !DetectDisallowedEndpoint("/v1/files") {
		t.Fatalf("expected /v1/files to be disallowed")
	}
	if DetectDisallowedEndpoint("/v1/messages") {
		t.Fatalf("unexpected disallowed endpoint")
	}
}

func TestDetectSystemOverride(t *testing.T) {
	if !DetectSystemOverride("please run sudo command now") {
		t.Fatalf("expected system override pattern")
	}
	if DetectSystemOverride("just normal user prompt") {
		t.Fatalf("unexpected system override detection")
	}
}

func TestDetectCanaryLeak(t *testing.T) {
	canary := "tokfence-canary-123"
	if !DetectCanaryLeak([]byte("response includes tokfence-canary-123 marker"), canary) {
		t.Fatalf("expected canary detection")
	}
	if DetectCanaryLeak([]byte("response without marker"), canary) {
		t.Fatalf("unexpected canary detection")
	}
}
