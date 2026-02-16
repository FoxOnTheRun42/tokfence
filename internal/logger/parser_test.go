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

func TestExtractUsageFromSSEAnthropicMessageAndDelta(t *testing.T) {
	sse := []byte(
		"data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":21}}}\n\n" +
			"data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":8}}\n\n",
	)
	usage := ExtractUsageFromSSE("anthropic", sse)
	if usage.InputTokens != 21 || usage.OutputTokens != 8 {
		t.Fatalf("unexpected anthropic streaming usage: %+v", usage)
	}
}

func TestExtractUsageFromSSEOpenAIStreamOptionsUsage(t *testing.T) {
	sse := []byte(
		"data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\n" +
			"data: {\"id\":\"x\",\"object\":\"chat.completion.chunk\",\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7}}\n\n",
	)
	usage := ExtractUsageFromSSE("openai", sse)
	if usage.InputTokens != 11 || usage.OutputTokens != 7 {
		t.Fatalf("unexpected openai streaming usage: %+v", usage)
	}
}
