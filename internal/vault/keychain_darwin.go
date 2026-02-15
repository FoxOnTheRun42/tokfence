//go:build darwin

package vault

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type KeychainVault struct{}

func NewKeychainVault() (*KeychainVault, error) {
	if _, err := exec.LookPath("security"); err != nil {
		return nil, fmt.Errorf("keychain security binary not found: %w", err)
	}
	return &KeychainVault{}, nil
}

func (v *KeychainVault) service(provider string) string {
	return "tokfence:" + provider
}

func (v *KeychainVault) Get(ctx context.Context, provider string) (string, error) {
	if err := ValidateProvider(provider); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "security", "find-generic-password", "-a", "tokfence", "-s", v.service(provider), "-w")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(strings.ToLower(stderr.String()), "could not be found") {
			return "", ErrKeyNotFound
		}
		return "", fmt.Errorf("read keychain key: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (v *KeychainVault) Set(ctx context.Context, provider, key string) error {
	if err := ValidateProvider(provider); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("key is empty")
	}
	cmd := exec.CommandContext(ctx, "security", "add-generic-password", "-a", "tokfence", "-s", v.service(provider), "-w", key, "-U")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("store keychain key: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (v *KeychainVault) Delete(ctx context.Context, provider string) error {
	if err := ValidateProvider(provider); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "security", "delete-generic-password", "-a", "tokfence", "-s", v.service(provider))
	if out, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(strings.ToLower(string(out)), "could not be found") {
			return nil
		}
		return fmt.Errorf("delete keychain key: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (v *KeychainVault) List(ctx context.Context) ([]string, error) {
	providers := Providers()
	enabled := make([]string, 0, len(providers))
	for _, provider := range providers {
		_, err := v.Get(ctx, provider)
		if err == nil {
			enabled = append(enabled, provider)
			continue
		}
		if !errors.Is(err, ErrKeyNotFound) {
			return nil, err
		}
	}
	sort.Strings(enabled)
	return enabled, nil
}
