package main

import (
	"io"
	"os"
	"path"
)

// EmailSaver saves email to the configured target.
type EmailSaver struct {
	path string
}

// NewSaver creates a new EmailSaver instance that saves emails to the specified path.
func NewSaver(path string) EmailSaver {
	err := os.MkdirAll(path, 0o700)
	if err != nil {
		panic("Failed to create directory " + path + ": " + err.Error())
	}

	return EmailSaver{
		path: path,
	}
}

func (es EmailSaver) Save(e Envelope, name string) error {
	f, err := os.Create(path.Join(es.path, name))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, e.Body)
	return err
}
