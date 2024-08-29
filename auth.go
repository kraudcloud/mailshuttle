package main

import (
	"errors"
	"log/slog"
)

// AuthStore is a struct that manages user authentication.
type AuthStore struct {
	configStore ConfigLoader
}

// Plain authenticates a user. It implements sasl.PlainAuthenticator
func (a *AuthStore) Plain(identity, username, password string) error {
	slog.Debug("authenticating user", "identity", identity, "username", username)
	if a.configStore.Load().Auth.Plain[username] != Password(password) {
		return errors.New("invalid password")
	}

	return nil
}
