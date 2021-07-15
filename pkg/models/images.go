package models

import (
	"errors"
	"regexp"

	"gorm.io/gorm"
)

// Image is what generates a OSTree Commit.
type Image struct {
	gorm.Model
	Name         string     `json:"Name"`
	Account      string     `json:"Account"`
	Distribution string     `json:"Distribution"`
	Description  string     `json:"Description"`
	Status       string     `json:"Status"`
	Version      int        `json:"Version" gorm:"default:1"`
	ImageType    string     `json:"ImageType"`
	CommitID     int        `json:"CommitID"`
	Commit       *Commit    `json:"Commit"`
	InstallerID  *int       `json:"InstallerID"`
	Installer    *Installer `json:"Installer"`
}

const (
	// DistributionCantBeNilMessage is the error message when a distribution is nil
	DistributionCantBeNilMessage = "distribution can't be empty"
	// ArchitectureCantBeEmptyMessage is the error message when the architecture is empty
	ArchitectureCantBeEmptyMessage = "architecture can't be empty"
	// NameCantBeInvalidMessage is the error message when the name is invalid
	NameCantBeInvalidMessage = "name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// ImageTypeNotAccepted is the error message when an image type is not accepted
	ImageTypeNotAccepted = "this image type is not accepted"

	// ImageTypeInstaller is the installer image type on Image Builder
	ImageTypeInstaller = "rhel-edge-installer"
	// ImageTypeCommit is the installer image type on Image Builder
	ImageTypeCommit = "rhel-edge-commit"

	// ImageStatusCreated is for when a image is created
	ImageStatusCreated = "CREATED"
	// ImageStatusBuilding is for when a image is building
	ImageStatusBuilding = "BUILDING"
	// ImageStatusError is for when a image is on a error state
	ImageStatusError = "ERROR"
	// ImageStatusSuccess is for when a image is available to the user
	ImageStatusSuccess = "SUCCESS"
)

// ValidateRequest validates an Image Request
func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New(DistributionCantBeNilMessage)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`).MatchString(i.Name) {
		return errors.New(NameCantBeInvalidMessage)
	}
	if i.Commit == nil || i.Commit.Arch == "" {
		return errors.New(ArchitectureCantBeEmptyMessage)
	}
	if i.ImageType != ImageTypeCommit && i.ImageType != ImageTypeInstaller {
		return errors.New(ImageTypeNotAccepted)
	}
	return nil
}
