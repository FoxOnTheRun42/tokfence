package logger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

type TokenUsage struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
}

func RequestHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:8])
}

func ExtractModelFromRequest(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	if model, ok := payload["model"].(string); ok {
		return model
	}
	return ""
}

func ExtractErrorFromBody(body []byte) (string, string) {
	if len(body) == 0 {
		return "", ""
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", ""
	}
	if errObj, ok := payload["error"].(map[string]any); ok {
		typeStr := asString(errObj["type"])
		msg := asString(errObj["message"])
		return typeStr, msg
	}
	return "", ""
}

func ExtractUsageFromResponse(provider string, body []byte) TokenUsage {
	if len(body) == 0 {
		return TokenUsage{}
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return TokenUsage{}
	}
	return extractUsageFromMap(strings.ToLower(provider), payload)
}

func ExtractUsageFromSSE(provider string, ssePayload []byte) TokenUsage {
	provider = strings.ToLower(provider)
	lines := strings.Split(string(ssePayload), "\n")
	result := TokenUsage{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		chunk := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if chunk == "" || chunk == "[DONE]" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(chunk), &payload); err != nil {
			continue
		}
		usage := extractUsageFromMap(provider, payload)
		if usage.InputTokens > 0 {
			result.InputTokens = usage.InputTokens
		}
		if usage.OutputTokens > 0 {
			result.OutputTokens = usage.OutputTokens
		}
		if usage.CacheReadTokens > 0 {
			result.CacheReadTokens = usage.CacheReadTokens
		}
		if usage.CacheCreationTokens > 0 {
			result.CacheCreationTokens = usage.CacheCreationTokens
		}
	}
	return result
}

func extractUsageFromMap(provider string, payload map[string]any) TokenUsage {
	usageRaw, ok := payload["usage"]
	if !ok {
		if message, ok := payload["message"].(map[string]any); ok {
			usageRaw = message["usage"]
		}
	}
	usageMap, ok := usageRaw.(map[string]any)
	if !ok {
		return TokenUsage{}
	}
	tokens := TokenUsage{}
	switch provider {
	case "anthropic":
		tokens.InputTokens = asInt(usageMap["input_tokens"])
		tokens.OutputTokens = asInt(usageMap["output_tokens"])
		tokens.CacheReadTokens = asInt(usageMap["cache_read_input_tokens"])
		tokens.CacheCreationTokens = asInt(usageMap["cache_creation_input_tokens"])
	default:
		tokens.InputTokens = firstNonZero(
			asInt(usageMap["input_tokens"]),
			asInt(usageMap["prompt_tokens"]),
		)
		tokens.OutputTokens = firstNonZero(
			asInt(usageMap["output_tokens"]),
			asInt(usageMap["completion_tokens"]),
		)
		tokens.CacheReadTokens = asInt(usageMap["cache_read_input_tokens"])
		tokens.CacheCreationTokens = asInt(usageMap["cache_creation_input_tokens"])
	}
	return tokens
}

func firstNonZero(values ...int) int {
	for _, v := range values {
		if v != 0 {
			return v
		}
	}
	return 0
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}
