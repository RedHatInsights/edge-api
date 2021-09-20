package models

import (
	"errors"
	"regexp"

	"github.com/lib/pq"
)

type ImageSet struct {
	Model
	Name    string  `json:"Name"`
	Version int     `json:"Version" gorm:"default:1"`
	Account string  `json:"Account"`
	Images  []Image `json:"Images"`
}

// Image is what generates a OSTree Commit.
type Image struct {
	Model
	Name         string         `json:"Name"`
	Account      string         `json:"Account"`
	Distribution string         `json:"Distribution"`
	Description  string         `json:"Description"`
	Status       string         `json:"Status"`
	Version      int            `json:"Version" gorm:"default:1"`
	ImageType    string         `json:"ImageType"` // TODO: Remove as soon as the frontend stops using
	OutputTypes  pq.StringArray `gorm:"type:text[]" json:"OutputTypes"`
	CommitID     uint           `json:"CommitID"`
	Commit       *Commit        `json:"Commit"`
	InstallerID  *uint          `json:"InstallerID"`
	Installer    *Installer     `json:"Installer"`
	ParentId     *uint          `gorm:"foreignKey:Image" json:"ParentId"`
	ImageSetID   *uint          `json:"ImageSetID"` // TODO: Wipe staging database and set to not nullable
	ID           uint           `gorm:"primarykey" json:"ID"`
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
	// NoOutputTypes is the error message when the output types list is empty
	NoOutputTypes = "an output type is required"

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

	// MissingInstaller is the error message for not passing an installer in the request
	MissingInstaller = "installer info must be provided"
	// MissingUsernameError is the error message for not passing username in the request
	MissingUsernameError = "username must be provided"
	// MissingSSHKeyError is the error message when SSH Key is not given
	MissingSSHKeyError = "SSH key must be provided"
	// InvalidSSHKeyError is the error message for not supported or invalid ssh key format
	InvalidSSHKeyError = "SSH Key supports RSA or DSS or ED25519 or ECDSA-SHA2 algorithms"
)

var (
	validSSHPrefix     = regexp.MustCompile(`^(ssh-(rsa|dss|ed25519)|ecdsa-sha2-nistp(256|384|521)) \S+`)
	validImageName     = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
	acceptedImageTypes = map[string]interface{}{ImageTypeCommit: nil, ImageTypeInstaller: nil}
)

// ValidateRequest validates an Image Request
func (i *Image) ValidateRequest() error {
	if i.Distribution == "" {
		return errors.New(DistributionCantBeNilMessage)
	}
	if !validImageName.MatchString(i.Name) {
		return errors.New(NameCantBeInvalidMessage)
	}
	if i.Commit == nil || i.Commit.Arch == "" {
		return errors.New(ArchitectureCantBeEmptyMessage)
	}
	if len(i.OutputTypes) == 0 {
		return errors.New(NoOutputTypes)
	}
	for _, out := range i.OutputTypes {
		if _, ok := acceptedImageTypes[out]; !ok {
			return errors.New(ImageTypeNotAccepted)
		}
	}
	// Installer checks
	if i.HasOutputType(ImageTypeInstaller) {
		if i.Installer == nil {
			return errors.New(MissingInstaller)
		}
		if i.Installer.Username == "" {
			return errors.New(MissingUsernameError)
		}
		if i.Installer.SSHKey == "" {
			return errors.New(MissingSSHKeyError)
		}
		if !validSSHPrefix.MatchString(i.Installer.SSHKey) {
			return errors.New(InvalidSSHKeyError)
		}

	}
	return nil
}

// HasOutputType checks if an image has an specific output type
func (i *Image) HasOutputType(imageType string) bool {
	for _, out := range i.OutputTypes {
		if out == imageType {
			return true
		}
	}
	return false
}
