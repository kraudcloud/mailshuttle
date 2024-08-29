package main

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"time"
	"unsafe"

	"github.com/emersion/go-smtp"
)

var (
	configPath   = cmp.Or(os.Getenv("CONFIG_PATH"), path.Join(cmp.Or(configDir, "/etc"), "mailshuttle/config.yaml"))
	configDir, _ = os.UserConfigDir()
	dumpConfig   = os.Getenv("DUMP_CONFIG") != ""
)

// Filter is a function that filters messages based on the sender and recipient.
type Filter func(from, to string) error

func main() {
	slog.Info("opening config file", "CONFIG_PATH", configPath)
	f, err := os.Open(configPath)
	if err != nil {
		slog.Error("failed to open config file", "err", err)
		os.Exit(2)
	}
	defer f.Close()

	c, err := ParseConfig(f)
	if err != nil {
		slog.Error("failed to parse config file", "err", err)
		os.Exit(1)
	}

	if dumpConfig {
		if err := DumpConfig(os.Stdout, c); err != nil {
			slog.Error("failed to dump config file", "err", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	slog.SetLogLoggerLevel(slog.Level(c.LogLevel))

	b := &Backend{
		authStore: &AuthStore{users: c.Auth.Plain},
		proxy: Proxy{
			filters: c.Filter,
			target:  &SMTPTarget{config: c.Proxy},
			saver:   NewSaver(c.Server.DataPath),
		},
	}

	if err := Serve(c.Server, b); err != nil {
		slog.Error("failed to serve", "err", err)
		os.Exit(1)
	}
}

func Serve(sc ServerConfig, b smtp.Backend) error {
	l, err := net.Listen("tcp", sc.String())
	if err != nil {
		slog.Error("failed to listen", "err", err)
		return err
	}

	s := smtp.NewServer(b)
	s.AllowInsecureAuth = true // have a tls ingress on the server.

	slog.Info("smtp server started", "addr", l.Addr())
	err = s.Serve(l)
	if err != nil {
		slog.Error("smtp encountered an error", "err", err)
		return err
	}

	return nil
}

// ReaderStringer is a struct that wraps an io.Reader and provides a String() method to read the entire contents of the reader.
type ReaderStringer struct {
	reader io.Reader
}

// String returns the entire contents of the underlying io.Reader as a string.
// This is a convenience method that reads the entire reader into memory.
// It is not recommended to use this method for large readers, as it will consume a lot of memory.
func (r ReaderStringer) String() string {
	buf, _ := io.ReadAll(r.reader)
	return unsafe.String(&buf[0], len(buf))
}

// NewReader creates a new ReaderStringer that wraps the provided io.Reader.
func NewReader(r io.Reader) ReaderStringer {
	return ReaderStringer{
		reader: r,
	}
}

// Proxy is a struct that proxies SMTP messages through a target SMTP server, applying filters to the messages.
// The Proxy struct has two fields:
type Proxy struct {
	filters FilterConfig
	target  interface {
		Send(e Envelope) error
	}
	saver interface {
		Save(e Envelope, name string) error
	}
}

// Envelope represents an SMTP message envelope, containing the sender, recipients, and message body.
type Envelope struct {
	// From and To fields represent the sender and recipient addresses, respectively.
	From string
	To   string

	// Opts and Rcpt fields contain options for the SMTP message handled by the server.
	Opts *smtp.MailOptions
	Rcpt *smtp.RcptOptions

	// Body holds the mail entire's contents (multipart)
	Body io.Reader
}

func (p Proxy) Do(e Envelope) error {
	l := slog.With("from", e.From, "to", e.To)

	buf, err := io.ReadAll(io.LimitReader(e.Body, int64(p.filters.MaxMessageSize)))
	if err != nil {
		l.Error("failed to read message", "err", err)
		return err
	}

	e.Body = bytes.NewReader(buf)

	name := fmt.Sprintf("%s.eml", time.Now().UTC().Format(time.RFC3339))
	l.Debug("saving", "name", name)
	if err := p.saver.Save(e, name); err != nil {
		l.Error("failed to save message", "err", err)
		return err
	}

	e.Body = bytes.NewReader(buf)
	l.Debug("filtering")
	for _, r := range p.filters.To {
		if r.MatchString(e.To) {
			l.Info("dropping email", "rule", r)
			return nil // just drop the email
		}
	}

	l.Info("proxying")
	return p.target.Send(e)
}