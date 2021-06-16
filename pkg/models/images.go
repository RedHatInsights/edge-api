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
	Status       string
	ComposeJobID string
	CommitID     int
	Commit       *Commit
}

const (
	// Errors
	DistributionCantBeNilMessage   = "distribution can't be empty"
	ArchitectureCantBeEmptyMessage = "architecture can't be empty"
	OnlyTarAcceptedMessage         = "only tar architecture supported for now"

	// Status
	ImageStatusCreated  = "CREATED"
	ImageStatusBuilding = "BUILDING"
	ImageStatusError    = "ERROR"
	ImageStatusSuccess  = "SUCCESS"
)

func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New(DistributionCantBeNilMessage)
	}
	if i.Commit == nil || i.Commit.Arch == "" {
		return errors.New(ArchitectureCantBeEmptyMessage)
	}
	if i.OutputType != "tar" {
		return errors.New(OnlyTarAcceptedMessage)
	}
	return nil
}
