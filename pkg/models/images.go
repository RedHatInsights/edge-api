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
	Packages     []Package `gorm:"many2many:image_packages;"`
	Status       string
	ComposeJobID string
	CommitID     int
	Commit       *Commit
}

type Package struct {
	gorm.Model
	Name string
}

func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New("distribution can't be empty")
	}
	if i.Commit == nil || i.Commit.Arch == "" {
		return errors.New("architecture can't be empty")
	}
	if i.OutputType != "tar" {
		return errors.New("only tar architecture supported for now")
	}
	return nil
}

func (i *Image) GetPackagesList() *[]string {
	pkgs := make([]string, len(i.Packages))
	for i, p := range i.Packages {
		pkgs[i] = p.Name
	}
	return &pkgs
}
