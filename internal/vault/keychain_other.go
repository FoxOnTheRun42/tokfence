//go:build !darwin

package vault

import "errors"

type KeychainVault struct{}

func NewKeychainVault() (*KeychainVault, error) {
	return nil, errors.New("keychain backend only available on darwin")
}
