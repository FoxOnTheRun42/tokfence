package security

import (
	"bytes"
	"regexp"
	"strings"
)

var (
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(^|[^a-z0-9_])sk-[a-z0-9-]{16,}\b`),
		regexp.MustCompile(`(?i)(^|[^a-z0-9_])gsk_[a-z0-9-]{32,}\b`),
		regexp.MustCompile(`(?i)\bAIza[0-9A-Za-z_-]{35}\b`),
		regexp.MustCompile(`(?i)\bxox[baprs]-[0-9]{10,}-[0-9a-zA-Z_-]{10,}\b`),
		regexp.MustCompile(`(?i)\bapi[_-]?key["']?\s*[:=]\s*["']?[a-zA-Z0-9_-]{16,}\b`),
	}

	// disallowedEndpointPatterns matches known high-risk provider management or file endpoints.
	disallowedEndpointPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)/v1/files\b`),
		regexp.MustCompile(`(?i)/v1/fine(_|-)?tuning\b`),
		regexp.MustCompile(`(?i)/v1/admin\b`),
		regexp.MustCompile(`(?i)/v1/assistants\b`),
		regexp.MustCompile(`(?i)/v1/billing\b`),
		regexp.MustCompile(`(?i)/v1/keys\b`),
	}

	systemOverridePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(system_override|override)\b`),
		regexp.MustCompile(`(?i)\bsudo\b`),
		regexp.MustCompile(`(?i)\brun[_-]command\b`),
		regexp.MustCompile(`(?i)\bexec\b`),
	}
)

func DetectSecretReference(input string) bool {
	normalized := strings.TrimSpace(input)
	if normalized == "" {
		return false
	}
	for _, p := range secretPatterns {
		if p.MatchString(normalized) {
			return true
		}
	}
	return false
}

func DetectDisallowedEndpoint(req string) bool {
	normalized := strings.ToLower(strings.TrimSpace(req))
	if normalized == "" {
		return false
	}
	for _, p := range disallowedEndpointPatterns {
		if p.MatchString(normalized) {
			return true
		}
	}
	return false
}

func DetectSystemOverride(input string) bool {
	normalized := strings.TrimSpace(input)
	if normalized == "" {
		return false
	}
	for _, p := range systemOverridePatterns {
		if p.MatchString(normalized) {
			return true
		}
	}
	return false
}

func DetectCanaryLeak(output []byte, canary string) bool {
	if len(canary) == 0 || len(output) == 0 {
		return false
	}
	return bytes.Contains(output, []byte(canary))
}
