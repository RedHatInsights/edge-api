package models

import (
	"errors"

	"gorm.io/gorm"
)

// Image is what generates a OSTree Commit.
type Image struct {
	gorm.Model
	Name         string
	Account      string
	Distribution string
	Description  string
	ImageType    string
	Status       string
	ComposeJobID string
	CommitID     int
	Version      int `gorm:"default:1"`
	Commit       *Commit
}

const (
	// Errors
	DistributionCantBeNilMessage   = "distribution can't be empty"
	ArchitectureCantBeEmptyMessage = "architecture can't be empty"
	ImageTypeNotAccepted           = "Image type must be rhel-edge-installer or rhel-edge-commit"

	// ImageTypes
	ImageTypeInstaller = "rhel-edge-installer"
	ImageTypeCommit    = "rhel-edge-commit"

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
	if i.ImageType != ImageTypeCommit && i.ImageType != ImageTypeInstaller {
		return errors.New(ImageTypeNotAccepted)
	}
	return nil
}
