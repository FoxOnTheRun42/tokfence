package logger

import "testing"

func TestExtractUsageFromResponseOpenAI(t *testing.T) {
	body := []byte(`{"usage":{"prompt_tokens":10,"completion_tokens":5}}`)
	usage := ExtractUsageFromResponse("openai", body)
	if usage.InputTokens != 10 || usage.OutputTokens != 5 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestExtractUsageFromSSEAnthropic(t *testing.T) {
	sse := []byte("data: {\"type\":\"message_stop\",\"usage\":{\"input_tokens\":12,\"output_tokens\":34}}\n\n")
	usage := ExtractUsageFromSSE("anthropic", sse)
	if usage.InputTokens != 12 || usage.OutputTokens != 34 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}
