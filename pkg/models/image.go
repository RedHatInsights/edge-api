package models

import (
	"errors"

	"github.com/lib/pq"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	apierror "github.com/redhatinsights/edge-api/pkg/errors"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Image is what generates a OSTree Commit.
type Image struct {
	Model
	Name                   string           `json:"Name"`
	Account                string           `json:"Account"`
	OrgID                  string           `json:"org_id" gorm:"index"`
	Distribution           string           `json:"Distribution"`
	Description            string           `json:"Description"`
	Status                 string           `json:"Status"`
	Version                int              `json:"Version" gorm:"default:1"`
	ImageType              string           `json:"ImageType"` // TODO: Remove as soon as the frontend stops using
	OutputTypes            pq.StringArray   `gorm:"type:text[]" json:"OutputTypes"`
	CommitID               uint             `json:"CommitID"`
	Commit                 *Commit          `json:"Commit"`
	InstallerID            *uint            `json:"InstallerID"`
	Installer              *Installer       `json:"Installer"`
	ImageSetID             *uint            `json:"ImageSetID" gorm:"index"` // TODO: Wipe staging database and set to not nullable
	Packages               []Package        `json:"Packages,omitempty" gorm:"many2many:images_packages;"`
	ThirdPartyRepositories []ThirdPartyRepo `json:"ThirdPartyRepositories,omitempty" gorm:"many2many:images_repos;"`
	CustomPackages         []Package        `json:"CustomPackages,omitempty" gorm:"many2many:images_custom_packages"`
	RequestID              string           `json:"request_id"` // storing for logging reference on resume
}

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
		if err := validateImageUserName(i.Installer.Username); err != nil {
			return err
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

// GetPackagesList returns the packages in a user-friendly list containing their names
func (i *Image) GetPackagesList() *[]string {

	if i.Distribution == "" {
		return nil
	}

	distributionPackage := config.DistributionsPackages[i.Distribution]

	requiredPackages := make([]string, 0, len(distributionPackage)+len(config.RequiredPackages))
	requiredPackages = append(requiredPackages, config.RequiredPackages...)
	requiredPackages = append(requiredPackages, distributionPackage...)

	l := len(requiredPackages)

	pkgs := make([]string, len(i.Packages)+l)
	for i, p := range requiredPackages {
		pkgs[i] = p
	}
	for i, p := range i.Packages {
		pkgs[i+l] = p.Name
	}
	return &pkgs
}

// GetALLPackagesList returns all the packages including custom packages containing their names
func (i *Image) GetALLPackagesList() *[]string {
	packagesList := i.GetPackagesList()
	if len(i.ThirdPartyRepositories) == 0 {
		// ignore custom packages when custom repositories list is empty
		return packagesList
	}
	var initialPackages []string
	if packagesList != nil {
		initialPackages = *packagesList
	}

	packages := make([]string, 0, len(initialPackages)+len(i.CustomPackages))
	packages = append(packages, initialPackages...)

	for _, pkg := range i.CustomPackages {
		packages = append(packages, pkg.Name)
	}
	return &packages
}

// BeforeCreate method is called before creating Images, it make sure org_id is not empty
func (i *Image) BeforeCreate(tx *gorm.DB) error {
	if i.OrgID == "" {
		log.Error("image do not have an org_id")
		return ErrOrgIDIsMandatory

	}

	return nil
}

// IsValid checks the image for conditions required to exist as an Image
func (i *Image) IsValid() (bool, error) {
	// TODO: compare this from service.CreateImage() to route ValidateRequest()
	if i.OrgID == "" {
		return false, new(apierror.OrgIDNotSetError)
	}

	if i.Name == "" {
		return false, new(apierror.ImageNameUndefinedError)
	}

	if i.Commit.Arch == "" || i.Distribution == "" {
		return false, apierror.NewBadRequest("architecture and/or distribution are not set")
	}

	return true, nil
}

// ImageExistsByName verifies image name exists in the database
func ImageExistsByName(name string, orgID string) (bool, error) {
	log.WithField("name", name).Debug("Checking image name exists in database")
	var imageFindByName *Image
	result := db.Org(orgID, "").Where("(name = ?)", name).First(&imageFindByName)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return false, nil
		}
		return false, result.Error
	}
	return imageFindByName != nil, nil
}

// ExistsByName verifies image name exists in the database
func (i *Image) ExistsByName() (bool, error) {
	return ImageExistsByName(i.Name, i.OrgID)
}
