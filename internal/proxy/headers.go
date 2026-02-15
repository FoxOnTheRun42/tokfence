package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

func StripIncomingAuth(headers http.Header) {
	headers.Del("Authorization")
	headers.Del("X-API-Key")
	headers.Del("x-api-key")
	headers.Del("X-Goog-Api-Key")
	headers.Del("x-goog-api-key")
}

func ApplyProviderAuth(headers http.Header, provider, key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("missing API key for provider %q", provider)
	}
	switch provider {
	case "anthropic":
		headers.Set("x-api-key", key)
		headers.Set("anthropic-version", "2023-06-01")
	case "openai", "mistral", "groq", "openrouter":
		headers.Set("Authorization", "Bearer "+key)
	case "google":
		headers.Set("x-goog-api-key", key)
	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
	return nil
}
