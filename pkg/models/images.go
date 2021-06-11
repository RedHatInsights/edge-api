package models

import (
	"errors"

	"gorm.io/gorm"
)

// An Image is what generates a OSTree Commit.
//
// swagger:model image
type Image struct {
	gorm.Model
	Name         string
	Account      string
	Distribution string // rhel-8
	Description  string
	OutputType   string
	Packages     []string
	Status       string
	ComposeJobID string
	commit       *Commit
}

func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New("distribution can't be empty")
	}
	if i.commit.Arch == "" {
		return errors.New("architecture can't be empty")
	}
	if i.OutputType != "tar" {
		return errors.New("only tar architecture supported for now")
	}
	return nil
}
