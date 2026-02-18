package security

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
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
	for _, normalized := range normalizedAndExpandedInputs(input) {
		for _, p := range secretPatterns {
			if p.MatchString(normalized) {
				return true
			}
		}
	}
	return false
}

func DetectDisallowedEndpoint(req string) bool {
	normalizedCandidates := normalizedAndExpandedInputs(req)
	for _, normalized := range normalizedCandidates {
		lowered := strings.ToLower(strings.TrimSpace(normalized))
		if lowered == "" {
			continue
		}
		for _, p := range disallowedEndpointPatterns {
			if p.MatchString(lowered) {
				return true
			}
		}
	}
	return false
}

func DetectSystemOverride(input string) bool {
	for _, normalized := range normalizedAndExpandedInputs(input) {
		for _, p := range systemOverridePatterns {
			if p.MatchString(normalized) {
				return true
			}
		}
	}
	return false
}

func DetectCanaryLeak(output []byte, canary string) bool {
	if len(canary) == 0 || len(output) == 0 {
		return false
	}
	if bytes.Contains(output, []byte(canary)) {
		return true
	}

	return outputContainsObfuscatedCanary(string(output), canary)
}

func normalizedAndExpandedInputs(input string) []string {
	normalized := strings.TrimSpace(input)
	if normalized == "" {
		return nil
	}

	seen := map[string]struct{}{}
	candidates := []string{}
	addCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if !utf8.ValidString(value) {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}
	addCandidate(normalized)

	if decoded, err := urlUnescapeOnce(normalized); err == nil && decoded != normalized {
		addCandidate(decoded)
	}

	if decoded, err := strconvUnquote(normalized); err == nil && decoded != normalized {
		addCandidate(decoded)
	}
	if decoded, err := base64UnescapeCandidate(normalized); err == nil {
		addCandidate(decoded)
	}
	if decoded, err := urlUnescapeOnce(normalized); err == nil {
		if decoded2, err := strconvUnquote(decoded); err == nil && decoded2 != decoded {
			addCandidate(decoded2)
		}
		if decoded3, err := base64UnescapeCandidate(decoded); err == nil {
			addCandidate(decoded3)
		}
	}
	if decodedParts := base64UnescapeFromParts(normalized); len(decodedParts) > 0 {
		for _, decoded := range decodedParts {
			addCandidate(decoded)
		}
	}
	return candidates
}

func urlUnescapeOnce(raw string) (string, error) {
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		return "", err
	}
	return decoded, nil
}

func strconvUnquote(raw string) (string, error) {
	quoted, err := strconv.Unquote(raw)
	if err != nil {
		return "", err
	}
	return quoted, nil
}

func base64UnescapeCandidate(raw string) (string, error) {
	if len(raw) < 16 {
		return "", fmt.Errorf("too short")
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	decodedString := strings.TrimSpace(string(decoded))
	if !utf8.ValidString(decodedString) {
		return "", fmt.Errorf("invalid utf8")
	}
	if len(decodedString) == 0 {
		return "", fmt.Errorf("empty payload")
	}
	return decodedString, nil
}

func base64UnescapeFromParts(raw string) []string {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return nil
	}
	for _, field := range fields {
		if len(field) < 16 {
			continue
		}
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(field)))
		n, err := base64.StdEncoding.Decode(decoded, []byte(field))
		if err != nil {
			continue
		}
		content := string(decoded[:n])
		if len(content) == 0 || !utf8.ValidString(content) {
			continue
		}
		return append([]string{}, content)
	}
	return nil
}

func outputContainsObfuscatedCanary(output string, canary string) bool {
	for _, normalized := range normalizedAndExpandedInputs(output) {
		if strings.Contains(normalized, canary) {
			return true
		}
	}
	return false
}
