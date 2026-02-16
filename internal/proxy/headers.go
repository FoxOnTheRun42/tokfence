package proxy

import (
	"fmt"
	"net/http"
	"strings"
)

func StripIncomingAuth(headers http.Header) {
	blocked := []string{
		"authorization",
		"proxy-authorization",
		"x-api-key",
		"api-key",
		"token",
		"x-goog-api-key",
		"bearer",
	}

	for key := range headers {
		lower := strings.ToLower(key)
		for _, candidate := range blocked {
			if strings.Contains(lower, candidate) {
				headers.Del(key)
				break
			}
		}
	}

	for key := range headers {
		for _, value := range headers[key] {
			for _, candidate := range blocked {
				if strings.Contains(strings.ToLower(value), candidate) {
					headers.Del(key)
					break
				}
			}
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
		return fmt.Errorf("unsupported provider %q", provider)
	}
	return nil
}
