package main

import (
	"errors"
	"log/slog"
	"sync"
)

// Password holds passwords
// TODO: bcrypt/scrypt/argon2id encrypted? accept all?
type Password string

// String returns a string representation of the Password, hiding the actual password value.
func (p Password) String() string {
	return "********"
}

// AuthStore is a struct that manages user authentication.
type AuthStore struct {
	mu    sync.Mutex
	users map[string]Password
}

// Plain authenticates a user. It implements sasl.PlainAuthenticator
func (a *AuthStore) Plain(identity, username, password string) error {
	slog.Debug("authenticating user", "identity", identity, "username", username)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.users[username] != Password(password) {
		return errors.New("invalid password")
	}

	return nil
}
