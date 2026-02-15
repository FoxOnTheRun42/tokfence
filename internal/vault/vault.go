package vault

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/macfox/tokfence/internal/config"
)

var ErrKeyNotFound = errors.New("key not found")

type Vault interface {
	Get(ctx context.Context, provider string) (string, error)
	Set(ctx context.Context, provider, key string) error
	Delete(ctx context.Context, provider string) error
	List(ctx context.Context) ([]string, error)
}

type Options struct {
	Passphrase string
	FilePath   string
}

var supportedProviders = map[string]struct{}{
	"anthropic":  {},
	"openai":     {},
	"google":     {},
	"mistral":    {},
	"openrouter": {},
	"groq":       {},
}

func ValidateProvider(provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "global" {
		return nil
	}
	if _, ok := supportedProviders[provider]; !ok {
		return fmt.Errorf("unsupported provider %q", provider)
	}
	return nil
}

func Providers() []string {
	out := make([]string, 0, len(supportedProviders))
	for provider := range supportedProviders {
		out = append(out, provider)
	}
	sort.Strings(out)
	return out
}

func NewDefault(opts Options) (Vault, error) {
	if backend := strings.ToLower(strings.TrimSpace(os.Getenv("TOKFENCE_VAULT_BACKEND"))); backend == "file" {
		return NewEncryptedFileVault(opts)
	}
	if runtime.GOOS == "darwin" {
		if v, err := NewKeychainVault(); err == nil {
			return v, nil
		}
	}
	return NewEncryptedFileVault(opts)
}

func defaultFilePath() (string, error) {
	dir, err := config.EnsureSecureDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vault.enc"), nil
}
