package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

type encryptedPayload struct {
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type plainVault struct {
	Providers map[string]string `json:"providers"`
}

type EncryptedFileVault struct {
	mu         sync.Mutex
	filePath   string
	passphrase []byte
}

func NewEncryptedFileVault(opts Options) (*EncryptedFileVault, error) {
	filePath := strings.TrimSpace(opts.FilePath)
	if filePath == "" {
		var err error
		filePath, err = defaultFilePath()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o700); err != nil {
		return nil, fmt.Errorf("create vault dir: %w", err)
	}
	if err := os.Chmod(filepath.Dir(filePath), 0o700); err != nil {
		return nil, fmt.Errorf("set vault dir perms: %w", err)
	}
	pass := strings.TrimSpace(opts.Passphrase)
	if pass == "" {
		pass = strings.TrimSpace(os.Getenv("TOKFENCE_VAULT_PASSPHRASE"))
	}
	if pass == "" {
		return nil, errors.New("TOKFENCE_VAULT_PASSPHRASE is required for encrypted file vault")
	}
	v := &EncryptedFileVault{
		filePath:   filePath,
		passphrase: []byte(pass),
	}
	return v, nil
}

func (v *EncryptedFileVault) Get(_ context.Context, provider string) (string, error) {
	if err := ValidateProvider(provider); err != nil {
		return "", err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	state, err := v.loadUnlocked()
	if err != nil {
		return "", err
	}
	key, ok := state.Providers[provider]
	if !ok || key == "" {
		return "", ErrKeyNotFound
	}
	return key, nil
}

func (v *EncryptedFileVault) Set(_ context.Context, provider, key string) error {
	if err := ValidateProvider(provider); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" {
		return errors.New("key is empty")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	state, err := v.loadUnlocked()
	if err != nil {
		return err
	}
	state.Providers[provider] = key
	return v.saveUnlocked(state)
}

func (v *EncryptedFileVault) Delete(_ context.Context, provider string) error {
	if err := ValidateProvider(provider); err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	state, err := v.loadUnlocked()
	if err != nil {
		return err
	}
	delete(state.Providers, provider)
	return v.saveUnlocked(state)
}

func (v *EncryptedFileVault) List(_ context.Context) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	state, err := v.loadUnlocked()
	if err != nil {
		return nil, err
	}
	providers := make([]string, 0, len(state.Providers))
	for provider := range state.Providers {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers, nil
}

func (v *EncryptedFileVault) loadUnlocked() (plainVault, error) {
	if _, err := os.Stat(v.filePath); err != nil {
		if os.IsNotExist(err) {
			return plainVault{Providers: map[string]string{}}, nil
		}
		return plainVault{}, fmt.Errorf("stat vault: %w", err)
	}
	data, err := os.ReadFile(v.filePath)
	if err != nil {
		return plainVault{}, fmt.Errorf("read vault: %w", err)
	}
	if len(data) == 0 {
		return plainVault{Providers: map[string]string{}}, nil
	}
	var payload encryptedPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return plainVault{}, fmt.Errorf("decode vault payload: %w", err)
	}
	salt, err := base64.StdEncoding.DecodeString(payload.Salt)
	if err != nil {
		return plainVault{}, fmt.Errorf("decode salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(payload.Nonce)
	if err != nil {
		return plainVault{}, fmt.Errorf("decode nonce: %w", err)
	}
	cipherText, err := base64.StdEncoding.DecodeString(payload.Ciphertext)
	if err != nil {
		return plainVault{}, fmt.Errorf("decode ciphertext: %w", err)
	}
	plain, err := decrypt(v.passphrase, salt, nonce, cipherText)
	if err != nil {
		return plainVault{}, fmt.Errorf("decrypt vault: %w", err)
	}
	var state plainVault
	if err := json.Unmarshal(plain, &state); err != nil {
		return plainVault{}, fmt.Errorf("decode vault json: %w", err)
	}
	if state.Providers == nil {
		state.Providers = map[string]string{}
	}
	return state, nil
}

func (v *EncryptedFileVault) saveUnlocked(state plainVault) error {
	if state.Providers == nil {
		state.Providers = map[string]string{}
	}
	plain, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal vault json: %w", err)
	}
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("create salt: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("create nonce: %w", err)
	}
	cipherText, err := encrypt(v.passphrase, salt, nonce, plain)
	if err != nil {
		return fmt.Errorf("encrypt vault: %w", err)
	}
	payload := encryptedPayload{
		Salt:       base64.StdEncoding.EncodeToString(salt),
		Nonce:      base64.StdEncoding.EncodeToString(nonce),
		Ciphertext: base64.StdEncoding.EncodeToString(cipherText),
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	if err := os.WriteFile(v.filePath, out, 0o600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}
	if err := os.Chmod(v.filePath, 0o600); err != nil {
		return fmt.Errorf("set vault file perms: %w", err)
	}
	return nil
}

func deriveKey(passphrase, salt []byte) []byte {
	return argon2.IDKey(passphrase, salt, 1, 64*1024, 4, 32)
}

func encrypt(passphrase, salt, nonce, plain []byte) ([]byte, error) {
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plain, nil), nil
}

func decrypt(passphrase, salt, nonce, cipherText []byte) ([]byte, error) {
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return nil, err
	}
	return plain, nil
}
