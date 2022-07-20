package services

import (
	"context"
	"fmt"
	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
)

// ImageSetsServiceInterface defines the interface that helps handle
// the business logic of ImageSets
type ImageSetsServiceInterface interface {
	GetImageSetsByID(imageSetID int) (*models.ImageSet, error)
	GetImageSetsViewCount(tx *gorm.DB) (int64, error)
	GetImageSetsView(limit int, offset int, tx *gorm.DB) (*[]models.ImageSetView, error)
}

// NewImageSetsService gives a instance of the main implementation of a ImageSetsServiceInterface
func NewImageSetsService(ctx context.Context, log *log.Entry) ImageSetsServiceInterface {
	return &ImageSetsService{
		Service: Service{ctx: ctx, log: log.WithField("service", "image-sets")},
	}
}

// ImageSetsService is the main implementation of a ImageSetsServiceInterface
type ImageSetsService struct {
	Service
}

// GetStorageInstallerIsoURL return the installer application storage url
func GetStorageInstallerIsoURL(installerID uint) string {
	if installerID == 0 {
		return ""
	}
	return fmt.Sprintf("/api/edge/v1/storage/isos/%d", installerID)
}

// GetImageSetsByID to get image set by id
func (s *ImageSetsService) GetImageSetsByID(imageSetID int) (*models.ImageSet, error) {
	var imageSet models.ImageSet
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err).Error("Error retrieving org_id")
		return nil, new(OrgIDNotSet)
	}
	result := db.Org(orgID, "image_sets").Debug().First(&imageSet, imageSetID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set by id")
		return nil, new(ImageSetNotFoundError)
	}
	result = db.Org(orgID, "").Debug().Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set's images")
		return nil, new(ImageNotFoundError)
	}
	return &imageSet, nil
}

// GetImageSetsViewCount get the ImageSets view records count
func (s *ImageSetsService) GetImageSetsViewCount(tx *gorm.DB) (int64, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return 0, err
	}

	if tx == nil {
		tx = db.DB
	}

	var count int64

	result := db.OrgDB(orgID, tx, "image_sets").Debug().
		Joins(`JOIN images ON image_sets.id = images.image_set_id AND Images.id = (Select Max(id) from Images where images.image_set_id = image_sets.id)`).
		Model(&models.ImageSet{}).Count(&count)

	if result.Error != nil {
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("Error getting image sets count")
		return 0, result.Error
	}

	return count, nil
}

// GetImageSetsView returns a list of ImageSets.
func (s *ImageSetsService) GetImageSetsView(limit int, offset int, tx *gorm.DB) (*[]models.ImageSetView, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		tx = db.DB
	}

	// ImageSetRow the structure for getting the main data table
	type ImageSetRow struct {
		ID          uint               `json:"ID"`
		Name        string             `json:"Name"`
		Version     int                `json:"Version"`
		UpdatedAt   models.EdgeAPITime `json:"UpdatedAt"`
		Status      string             `json:"Status"`
		InstallerID uint               `json:"InstallerID"`
	}

	var imageSetsRows []ImageSetRow

	if result := db.OrgDB(orgID, tx, "image_sets").Debug().Table("image_sets").Limit(limit).Offset(offset).
		Select("image_sets.id, image_sets.name, image_sets.version, image_sets.updated_at, images.status").
		Joins(`JOIN images ON image_sets.id = images.image_set_id AND Images.id = (Select Max(id) from Images where images.image_set_id = image_sets.id)`).
		Find(&imageSetsRows); result.Error != nil {

		log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error(
			"error when getting image sets view data",
		)
		return nil, err
	}
	if len(imageSetsRows) == 0 {
		return &[]models.ImageSetView{}, nil
	}

	// get the latest installer iso url for each image-set
	// get the image-set ids
	var imageSetIDS []uint
	for _, imageSetRow := range imageSetsRows {
		imageSetIDS = append(imageSetIDS, imageSetRow.ID)
	}

	// ImageSetInstaller the structure that correspond to the latest successful installer build of each image-set
	type ImageSetInstaller struct {
		ImageSetID  uint `json:"ImageSetID"`
		InstallerID uint `json:"InstallerID"`
	}

	var imageSetsInstallers []ImageSetInstaller
	if result := db.Org(orgID, "images").Debug().Table("images").
		Select(`images.image_set_id, Max(installers.id) as "installer_id"`).
		Joins("JOIN installers ON images.installer_id = installers.id").
		Where("images.status = ? AND images.image_set_id in (?)", models.ImageStatusSuccess, imageSetIDS).
		Where("(installers.image_build_iso_url != '' AND installers.image_build_iso_url IS NOT NULL)").
		Group("images.image_set_id").
		Find(&imageSetsInstallers); result.Error != nil {
		log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error(
			"error when getting image sets view installer data",
		)
		return nil, err
	}

	// create a map of the corresponding image-sets and installers
	imageSetsInstallersMap := make(map[uint]uint, len(imageSetsInstallers))
	for _, imageInstaller := range imageSetsInstallers {
		imageSetsInstallersMap[imageInstaller.ImageSetID] = imageInstaller.InstallerID
	}

	// create the main image-sets view
	imageSetsView := make([]models.ImageSetView, 0, len(imageSetsRows))
	for _, imageSetRow := range imageSetsRows {
		imageSetView := models.ImageSetView{
			ID:        imageSetRow.ID,
			Name:      imageSetRow.Name,
			Version:   imageSetRow.Version,
			UpdatedAt: imageSetRow.UpdatedAt,
			Status:    imageSetRow.Status,
		}
		installerID, ok := imageSetsInstallersMap[imageSetRow.ID]
		if ok {
			imageSetView.ImageBuildIsoURL = GetStorageInstallerIsoURL(installerID)
		}
		imageSetsView = append(imageSetsView, imageSetView)

	}

	return &imageSetsView, nil
}
