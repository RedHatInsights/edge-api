package seeder

import (
	"fmt"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

var orgID = common.DefaultOrgID

// Seeder options
func Images() *Image {
	return &Image{
		version:          1,
		ostreeHashCommit: faker.UUIDHyphenated(),
		installedPackages: []models.InstalledPackage{
			{
				Name:    "ansible",
				Version: "1.0.0",
			},
			{
				Name:    "yum",
				Version: "2:6.0-1",
			},
		},
		repoURL: faker.URL(),
	}
}

func Devices() *Device {
	return &Device{}
}

// Image seeder
type Image struct {
	imageSetID        uint
	ostreeHashCommit  string
	version           int
	installedPackages []models.InstalledPackage
	repoURL           string
}

func (i *Image) WithImageSetID(imageSetID uint) *Image {
	i.imageSetID = imageSetID
	return i
}

func (i *Image) WithOstreeCommit(ostreeHashCommit string) *Image {
	i.ostreeHashCommit = ostreeHashCommit
	return i
}

func (i *Image) WithInstalledPackages(installedPackages []models.InstalledPackage) *Image {
	i.installedPackages = installedPackages
	return i
}

func (i *Image) WithVersion(version int) *Image {
	i.version = version
	return i
}

func (i *Image) WithRepoURL(repoURL string) *Image {
	i.repoURL = repoURL
	return i
}

func (i *Image) Create() (*models.Image, *models.ImageSet) {
	var imageSet *models.ImageSet

	if i.imageSetID == 0 {
		imageSet = &models.ImageSet{
			Name:    fmt.Sprintf("image-test-%s", faker.UUIDHyphenated()),
			Version: 1,
			OrgID:   orgID,
		}
		db.DB.Create(imageSet)
	} else {
		db.DB.Where("id = ?", i.imageSetID).First(&imageSet)
	}

	image := &models.Image{
		Name: imageSet.Name,
		Commit: &models.Commit{
			Arch:              "x86_64",
			OSTreeCommit:      i.ostreeHashCommit,
			InstalledPackages: i.installedPackages,
			OrgID:             orgID,
			Repo: &models.Repo{
				URL:    i.repoURL,
				Status: models.RepoStatusSuccess,
			},
		},
		Version:      i.version,
		Status:       models.ImageStatusSuccess,
		ImageSetID:   &imageSet.ID,
		OrgID:        orgID,
		Account:      common.DefaultAccount,
		Distribution: "rhel-91",
		OutputTypes:  []string{models.ImageTypeCommit},
	}

	db.DB.Create(image.Commit)
	db.DB.Create(image)

	return image, imageSet
}

// Device seeder
type Device struct {
	imageID uint
}

func (d *Device) WithImageID(imageID uint) *Device {
	d.imageID = imageID
	return d
}

func (d *Device) Create() *models.Device {
	var image *models.Image

	if d.imageID == 0 {
		image, _ = Images().Create()
	} else {
		image = &models.Image{
			Model: models.Model{ID: d.imageID},
		}
	}

	device := &models.Device{
		UUID:            faker.UUIDHyphenated(),
		RHCClientID:     faker.UUIDHyphenated(),
		UpdateAvailable: false,
		ImageID:         image.ID,
		OrgID:           orgID,
	}

	db.DB.Create(&device)

	return device
}
