package proxy

import (
	"fmt"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/macfox/tokfence/internal/config"
)

type Route struct {
	Provider     string
	ForwardedURL *url.URL
	Path         string
}

func KnownProviders(cfg config.Config) []string {
	providers := make([]string, 0, len(cfg.Providers))
	for name := range cfg.Providers {
		providers = append(providers, name)
	}
	sort.Strings(providers)
	return providers
}

func ParseProviderPath(requestPath string) (provider, forwardPath string, err error) {
	trimmed := strings.TrimPrefix(requestPath, "/")
	if trimmed == "" {
		return "", "", fmt.Errorf("missing provider in path")
	}
	parts := strings.SplitN(trimmed, "/", 2)
	provider = strings.TrimSpace(parts[0])
	if provider == "" {
		return "", "", fmt.Errorf("missing provider in path")
	}
	if len(parts) == 1 || parts[1] == "" {
		return provider, "/", nil
	}
	return provider, "/" + parts[1], nil
}

func ResolveRoute(cfg config.Config, requestPath, rawQuery string) (Route, error) {
	provider, forwardPath, err := ParseProviderPath(requestPath)
	if err != nil {
		return Route{}, err
	}
	providerCfg, ok := cfg.Providers[provider]
	if !ok {
		return Route{}, fmt.Errorf("unsupported provider %q", provider)
	}
	baseURL, err := url.Parse(providerCfg.Upstream)
	if err != nil {
		return Route{}, fmt.Errorf("parse upstream for %s: %w", provider, err)
	}
	joinedPath := joinURLPath(baseURL.Path, forwardPath)
	baseURL.Path = joinedPath
	baseURL.RawPath = joinedPath
	baseURL.RawQuery = rawQuery
	return Route{
		Provider:     provider,
		ForwardedURL: baseURL,
		Path:         forwardPath,
	}, nil
}

func joinURLPath(base, suffix string) string {
	if suffix == "" {
		suffix = "/"
	}
	if !strings.HasPrefix(suffix, "/") {
		suffix = "/" + suffix
	}
	if base == "" || base == "/" {
		return suffix
	}
	cleanBase := strings.TrimSuffix(base, "/")
	cleanSuffix := strings.TrimPrefix(suffix, "/")
	joined := path.Join(cleanBase, cleanSuffix)
	if strings.HasSuffix(suffix, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}
	return joined
}
