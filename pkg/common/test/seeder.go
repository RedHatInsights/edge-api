package test

import (
	"fmt"
	"reflect"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
)

var orgID = common.DefaultOrgID

type Seeder struct {
	imageSetID        uint
	imageID           uint
	ostreeHashCommit  string
	version           int
	installedPackages []models.InstalledPackage
}

func NewSeeder() *Seeder {
	return &Seeder{}
}

func clearSetup(v interface{}) {
	p := reflect.ValueOf(v).Elem()
	p.Set(reflect.Zero(p.Type()))
}

func (s *Seeder) WithImageSetID(imageSetID uint) *Seeder {
	s.imageSetID = imageSetID
	return s
}

func (s *Seeder) WithImageID(imageID uint) *Seeder {
	s.imageID = imageID
	return s
}

func (s *Seeder) WithOstreeCommit(ostreeHashCommit string) *Seeder {
	s.ostreeHashCommit = ostreeHashCommit
	return s
}

func (s *Seeder) WithInstalledPackages(installedPackages []models.InstalledPackage) *Seeder {
	s.installedPackages = installedPackages
	return s
}

func (s *Seeder) WithVersion(version int) *Seeder {
	s.version = version
	return s
}

func (s *Seeder) CreateImage() (*models.Image, *models.ImageSet) {
	var imageSet *models.ImageSet

	if s.ostreeHashCommit == "" {
		s.ostreeHashCommit = faker.UUIDHyphenated()
	}

	if s.version == 0 {
		s.version = 1
	}

	if s.installedPackages == nil {
		s.installedPackages = []models.InstalledPackage{
			{
				Name:    "ansible",
				Version: "1.0.0",
			},
			{
				Name:    "yum",
				Version: "2:6.0-1",
			},
		}
	}

	if s.imageSetID == 0 {
		imageSet = &models.ImageSet{
			Name:    fmt.Sprintf("image-test-%s", faker.UUIDHyphenated()),
			Version: 1,
			OrgID:   orgID,
		}
		db.DB.Create(imageSet)
	} else {
		db.DB.Where("id = ?", s.imageSetID).First(&imageSet)
	}

	image := &models.Image{
		Commit: &models.Commit{
			OSTreeCommit:      s.ostreeHashCommit,
			InstalledPackages: s.installedPackages,
			OrgID:             orgID,
		},
		Version:    s.version,
		Status:     models.ImageStatusSuccess,
		ImageSetID: &imageSet.ID,
		OrgID:      orgID,
	}

	db.DB.Create(image.Commit)
	db.DB.Create(image)

	clearSetup(s)

	return image, imageSet
}

func (s *Seeder) CreateDevice() *models.Device {
	var image *models.Image

	if s.imageID == 0 {
		image, _ = s.CreateImage()
	} else {
		image = &models.Image{
			Model: models.Model{ID: s.imageID},
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

	clearSetup(s)

	return device
}
