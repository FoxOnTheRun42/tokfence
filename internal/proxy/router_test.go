package proxy

import (
	"testing"

	"github.com/FoxOnTheRun42/tokfence/internal/config"
)

func TestResolveRoute(t *testing.T) {
	cfg := config.Default()
	route, err := ResolveRoute(cfg, "/openai/v1/chat/completions", "a=1")
	if err != nil {
		t.Fatalf("ResolveRoute() error = %v", err)
	}
	if route.Provider != "openai" {
		t.Fatalf("provider = %s, want openai", route.Provider)
	}
	got := route.ForwardedURL.String()
	want := "https://api.openai.com/v1/chat/completions?a=1"
	if got != want {
		t.Fatalf("forwarded URL = %s, want %s", got, want)
	}
}

func TestResolveRouteOpenRouterBasePath(t *testing.T) {
	cfg := config.Default()
	route, err := ResolveRoute(cfg, "/openrouter/v1/chat/completions", "")
	if err != nil {
		t.Fatalf("ResolveRoute() error = %v", err)
	}
	got := route.ForwardedURL.String()
	want := "https://openrouter.ai/api/v1/chat/completions"
	if got != want {
		t.Fatalf("forwarded URL = %s, want %s", got, want)
	}
}
