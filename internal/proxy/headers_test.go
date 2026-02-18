package proxy

import (
	"net/http"
	"testing"
)

func TestStripIncomingAuthIsHeaderNameOnlyAndCaseInsensitive(t *testing.T) {
	headers := make(http.Header)
	headers.Set("Authorization", "Bearer upstream-token")
	headers.Set("proxy-authorization", "Bearer another")
	headers.Set("X-API-Key", "agent-key")
	headers.Set("Api-Key", "agent-key-2")
	headers.Set("X-Goog-API-Key", "google-key")
	headers.Set("token", "value")
	headers.Set("BEARER", "value")

	StripIncomingAuth(headers)

	if got := headers.Get("Authorization"); got != "" {
		t.Fatalf("Authorization should be stripped, got %q", got)
	}
	if got := headers.Get("proxy-authorization"); got != "" {
		t.Fatalf("proxy-authorization should be stripped, got %q", got)
	}
	if got := headers.Get("X-API-Key"); got != "" {
		t.Fatalf("X-API-Key should be stripped, got %q", got)
	}
	if got := headers.Get("X-Goog-API-Key"); got != "" {
		t.Fatalf("X-Goog-API-Key should be stripped, got %q", got)
	}
	if got := headers.Get("token"); got != "" {
		t.Fatalf("token should be stripped, got %q", got)
	}
}

func TestStripIncomingAuthKeepsFalsePositivesAndBearerValue(t *testing.T) {
	headers := make(http.Header)
	headers.Set("X-Custom-AuthToken", "contains bearer token")
	headers.Set("X-Custom-Auth", "Bearer token-value")

	StripIncomingAuth(headers)

	if got := headers.Get("X-Custom-AuthToken"); got != "contains bearer token" {
		t.Fatalf("X-Custom-AuthToken should not be stripped, got %q", got)
	}
	if got := headers.Get("X-Custom-Auth"); got != "Bearer token-value" {
		t.Fatalf("X-Custom-Auth should not be stripped by value content, got %q", got)
	}
}

func TestApplyProviderAuthUnknownProviderUsesBearer(t *testing.T) {
	headers := make(http.Header)
	if err := ApplyProviderAuth(headers, "deepseek", "sk-test"); err != nil {
		t.Fatalf("ApplyProviderAuth() error = %v", err)
	}
	if got := headers.Get("Authorization"); got != "Bearer sk-test" {
		t.Fatalf("Authorization = %q, want %q", got, "Bearer sk-test")
	}
}

func TestApplyProviderAuthAnthropicUsesXAPIKey(t *testing.T) {
	headers := make(http.Header)
	if err := ApplyProviderAuth(headers, "anthropic", "sk-ant-test"); err != nil {
		t.Fatalf("ApplyProviderAuth() error = %v", err)
	}
	if got := headers.Get("x-api-key"); got != "sk-ant-test" {
		t.Fatalf("x-api-key = %q, want %q", got, "sk-ant-test")
	}
	if got := headers.Get("Authorization"); got != "" {
		t.Fatalf("Authorization = %q, want empty", got)
	}
}
