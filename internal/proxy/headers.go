package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

var blockedAuthHeaders = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"api-key":             {},
	"token":               {},
	"x-goog-api-key":      {},
	"bearer":              {},
}

func StripIncomingAuth(headers http.Header) {
	for key := range headers {
		normalized := strings.ToLower(strings.TrimSpace(http.CanonicalHeaderKey(key)))
		if _, blocked := blockedAuthHeaders[normalized]; blocked {
			headers.Del(key)
		}
	}
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
		headers.Set("Authorization", "Bearer "+key)
	}
	return nil
}
