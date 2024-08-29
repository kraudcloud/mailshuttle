package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// Proxier is a function that proxies the message from the sender to the
// intended recipient
type Proxier interface {
	Do(Envelope) error
}

// The Backend implements SMTP server methods.
type Backend struct {
	authStore *AuthStore
	proxy     Proxier
}

// NewSession is called after client greeting (EHLO, HELO).
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{
		authStore: bkd.authStore,
		proxy:     bkd.proxy,
	}, nil
}

// A Session is returned after successful login.
type Session struct {
	authStore *AuthStore
	proxy     Proxier

	log    *slog.Logger
	authed bool
	Envelope
}

// AuthMechanisms returns a slice of available auth mechanisms; only PLAIN is
// supported in this example.
func (s *Session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

// Auth is the handler for supported authenticators.
func (s *Session) Auth(mech string) (_ sasl.Server, err error) {
	switch mech {
	case sasl.Plain:
		return sasl.NewPlainServer(func(identity, username, password string) error {
			if err := s.authStore.Plain(identity, username, password); err != nil {
				return err
			}
			s.log = slog.Default().With("user", username)
			s.authed = true
			return nil
		}), nil
	}

	return nil, errors.New("unsupported auth mechanism")
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	if !s.authed {
		return fmt.Errorf("not authenticated")
	}

	s.log = s.log.With("from", from)
	s.Envelope.From = from
	s.Envelope.Opts = opts
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.log = s.log.With("to", to)
	s.Envelope.To = to
	s.Envelope.Rcpt = opts
	return nil
}

func (s *Session) Data(body io.Reader) error {
	s.Envelope.Body = body
	err := s.proxy.Do(s.Envelope)
	if err != nil {
		s.log.Error("error proxying message", "err", err)
	}

	return nil
}

func (s *Session) Reset() {
	s.Envelope = Envelope{}
}

func (s *Session) Logout() error {
	return nil // todo figure out
}
