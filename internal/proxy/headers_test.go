package proxy

import (
	"net/http"
	"testing"
)

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
