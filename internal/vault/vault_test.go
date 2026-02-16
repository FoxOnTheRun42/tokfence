package vault

import "testing"

func TestValidateProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  bool
	}{
		{name: "known provider", provider: "openai", wantErr: false},
		{name: "custom provider", provider: "deepseek", wantErr: false},
		{name: "custom provider with dash", provider: "my-provider_1", wantErr: false},
		{name: "upper-case normalizes", provider: "OpenAI", wantErr: false},
		{name: "empty", provider: "", wantErr: true},
		{name: "invalid chars", provider: "bad provider", wantErr: true},
		{name: "invalid prefix", provider: "-provider", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProvider(tt.provider)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateProvider(%q) expected error", tt.provider)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateProvider(%q) unexpected error: %v", tt.provider, err)
			}
		})
	}
}
