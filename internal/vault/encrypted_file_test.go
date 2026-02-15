package vault

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEncryptedFileVaultRoundTrip(t *testing.T) {
	t.Setenv("TOKFENCE_VAULT_PASSPHRASE", "test-passphrase")
	path := filepath.Join(t.TempDir(), "vault.enc")
	v, err := NewEncryptedFileVault(Options{FilePath: path})
	if err != nil {
		t.Fatalf("NewEncryptedFileVault() error = %v", err)
	}
	ctx := context.Background()
	if err := v.Set(ctx, "openai", "sk-test"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, err := v.Get(ctx, "openai")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != "sk-test" {
		t.Fatalf("Get() = %s, want sk-test", got)
	}
	list, err := v.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0] != "openai" {
		t.Fatalf("List() = %#v, want [openai]", list)
	}
	if err := v.Delete(ctx, "openai"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = v.Get(ctx, "openai")
	if err == nil {
		t.Fatalf("expected key to be deleted")
	}
}
