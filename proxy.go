package main

import (
	"io"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
)

// SMTPTarget represents a target server to proxy messages to.
type SMTPTarget struct {
	configStore ConfigLoader
}

// Send sends an email message to the specified recipient using the SMTP target configuration.
// The from parameter specifies the email sender, opts provides additional mail options,
// to specifies the email recipient, rcpt provides additional recipient options, and body
// is the email message body reader.
// If the SMTP target configuration includes a username and password, the function will
// authenticate with the SMTP server using SASL login.
// The function returns an error if any part of the email sending process fails.
func (s *SMTPTarget) Send(e Envelope) error {
	proxy := s.configStore.Load().Proxy
	client, err := smtp.DialStartTLS(proxy.Addr(), nil)
	if err != nil {
		return err
	}

	// Authenticate if credentials are provided
	if proxy.Username != "" && proxy.Password != "" {
		err = client.Auth(sasl.NewLoginClient(proxy.Username, string(proxy.Password)))
		if err != nil {
			return err
		}
	}

	// Set the sender and recipient
	err = client.Mail(e.From, e.Opts)
	if err != nil {
		return err
	}
	err = client.Rcpt(e.To, e.Rcpt)
	if err != nil {
		return err
	}

	// Send the email body
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = io.Copy(w, e.Body)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}
