package config

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const KeyringService = "autodl-cli"
const KeyringUser = "developer-token"

// SecretStore allows credential behavior to be tested without a desktop session.
type SecretStore interface {
	Get() (string, error)
	Set(string) error
}

type SystemStore struct{}

func (SystemStore) Get() (string, error) {
	token, err := keyring.Get(KeyringService, KeyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return token, err
}

func (SystemStore) Set(token string) error {
	return keyring.Set(KeyringService, KeyringUser, token)
}
