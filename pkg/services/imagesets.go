// FIXME: golangci-lint
// nolint:govet,ineffassign,revive,staticcheck
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
	GetImageSetViewByID(imageSetID uint, imagesLimit int, imagesOffSet int, imagesDBFilter *gorm.DB) (*ImageSetIDView, error)
	GetImageSetsBuildIsoURL(orgID string, imageSetIDS []uint) (map[uint]uint, error)
	GetImagesViewData(imageSetID uint, imagesLimit int, imagesOffSet int, tx *gorm.DB) (*ImagesViewData, error)
	GetImageSetImageViewByID(imageSetID uint, imageID uint) (*ImageSetImageIDView, error)
}

// ImagesViewData is the images view data return for images view with filters , limit, offSet
type ImagesViewData struct {
	Count int64              `json:"count"`
	Data  []models.ImageView `json:"data"`
}

// ImageSetIDView is the image set details view returned for ui image-set display
type ImageSetIDView struct {
	ImageBuildIsoURL string          `json:"ImageBuildIsoURL"`
	ImageSet         models.ImageSet `json:"ImageSet"`
	LastImageDetails ImageDetail     `json:"LastImageDetails"`
}

// ImageSetImageIDView is the image set image view returned for ui image-set / version display
type ImageSetImageIDView struct {
	ImageBuildIsoURL string          `json:"ImageBuildIsoURL"`
	ImageSet         models.ImageSet `json:"ImageSet"`
	ImageDetails     ImageDetail     `json:"ImageDetails"`
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
	result := db.Org(orgID, "image_sets").First(&imageSet, imageSetID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set by id")
		return nil, new(ImageSetNotFoundError)
	}
	result = db.Org(orgID, "").Where("image_set_id = ?", imageSetID).Find(&imageSet.Images)
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

	if result := db.OrgDB(orgID, tx, "image_sets").Debug().
		Joins(`JOIN images ON image_sets.id = images.image_set_id`).
		Model(&models.ImageSet{}).Distinct("image_sets.id").Count(&count); result.Error != nil {
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
		ID        uint               `json:"ID"`
		Name      string             `json:"Name"`
		Version   int                `json:"Version"`
		UpdatedAt models.EdgeAPITime `json:"UpdatedAt"`
		Status    string             `json:"Status"`
		ImageID   uint               `json:"ImageID"`
	}

	var imageSetsRows []ImageSetRow

	if result := db.OrgDB(orgID, tx, "image_sets").Debug().Table("image_sets").Limit(limit).Offset(offset).
		Select(`image_sets.id, image_sets.name, image_sets.version, image_sets.updated_at, images.status, images.id as "image_id"`).
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

	imageSetsInstallersMap, err := s.GetImageSetsBuildIsoURL(orgID, imageSetIDS)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "OrgID": orgID}).Error(
			"error when getting image-sets view installer data",
		)
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
			ImageID:   imageSetRow.ImageID,
		}
		installerID, ok := imageSetsInstallersMap[imageSetRow.ID]
		if ok {
			imageSetView.ImageBuildIsoURL = GetStorageInstallerIsoURL(installerID)
		}
		imageSetsView = append(imageSetsView, imageSetView)

	}

	return &imageSetsView, nil
}

// GetImageSetsBuildIsoURL return a map of image-set id and the latest successfully built installer id
func (s *ImageSetsService) GetImageSetsBuildIsoURL(orgID string, imageSetIDS []uint) (map[uint]uint, error) {
	if orgID == "" {
		return nil, new(OrgIDNotSet)
	}
	if len(imageSetIDS) == 0 {
		return map[uint]uint{}, nil
	}

	// ImageSetInstaller the structure that correspond to the latest successful installer build of each image-set
	type ImageSetInstaller struct {
		ImageSetID  uint `json:"ImageSetID"`
		InstallerID uint `json:"InstallerID"`
	}
	var imageSetsInstallers []ImageSetInstaller

	if result := db.Org(orgID, "images").Table("images").
		Select(`images.image_set_id, Max(installers.id) as "installer_id"`).
		Joins("JOIN installers ON images.installer_id = installers.id").
		Where("images.status = ? AND images.image_set_id in (?)", models.ImageStatusSuccess, imageSetIDS).
		Where("(installers.image_build_iso_url != '' AND installers.image_build_iso_url IS NOT NULL)").
		Group("images.image_set_id").
		Find(&imageSetsInstallers); result.Error != nil {
		log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error(
			"error when getting image sets view installer data",
		)
		return nil, result.Error
	}

	imageSetsInstallersMap := make(map[uint]uint, len(imageSetsInstallers))
	for _, imageInstaller := range imageSetsInstallers {
		imageSetsInstallersMap[imageInstaller.ImageSetID] = imageInstaller.InstallerID
	}
	return imageSetsInstallersMap, nil
}

// GetImagesViewData return images view count and images view data of the supplied image-set
func (s *ImageSetsService) GetImagesViewData(imageSetID uint, imagesLimit int, imagesOffSet int, imagesDBFilter *gorm.DB) (*ImagesViewData, error) {
	if imagesDBFilter == nil {
		imagesDBFilter = db.DB
	}
	imageService := NewImageService(s.ctx, s.log)

	imagesDBFilter = imagesDBFilter.Where("image_set_id = ?", imageSetID)
	imagesCount, err := imageService.GetImagesViewCount(imagesDBFilter)
	if err != nil {
		return nil, err
	}
	imagesView, err := imageService.GetImagesView(imagesLimit, imagesOffSet, imagesDBFilter)
	if err != nil {
		return nil, err
	}
	return &ImagesViewData{Count: imagesCount, Data: *imagesView}, nil
}

// GetImageSetViewByID return the data related to image set, data, build iso url, last image and images view.
func (s *ImageSetsService) GetImageSetViewByID(imageSetID uint, imagesLimit int, imagesOffSet int, imagesDBFilter *gorm.DB) (*ImageSetIDView, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, new(OrgIDNotSet)
	}
	if imagesDBFilter == nil {
		imagesDBFilter = db.DB
	}

	imageService := NewImageService(s.ctx, s.log)

	var imageSetIDView ImageSetIDView

	// get the image-set
	if result := db.Org(orgID, "").First(&imageSetIDView.ImageSet, imageSetID); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("image-set not found")
			return nil, new(ImageSetNotFoundError)
		}
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("error getting image-set")
		return nil, result.Error
	}

	var lastImage models.Image
	// get the last image-set image
	if result := db.Org(orgID, "").Order("created_at DESC").
		Preload("Packages").
		Preload("CustomPackages").
		Preload("Installer").
		Preload("Commit").
		Preload("Commit.Repo").
		Preload("Commit.InstalledPackages").
		Where("image_set_id = ?", imageSetID).First(&lastImage); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("image-set last image not found")
			return nil, new(ImageNotFoundError)
		}
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("error getting image-set last-image")
		return nil, result.Error
	}

	imageInfo, err := imageService.AddPackageInfo(&lastImage)
	if err != nil {
		return nil, err
	}
	imageSetIDView.LastImageDetails = imageInfo

	// set build iso URL
	if imageSetsInstallersMap, err := s.GetImageSetsBuildIsoURL(orgID, []uint{imageSetID}); err != nil {
		return nil, err
	} else if installerID, ok := imageSetsInstallersMap[imageSetID]; ok {
		imageSetIDView.ImageBuildIsoURL = GetStorageInstallerIsoURL(installerID)
	}

	if imageSetIDView.LastImageDetails.Image.Installer != nil && imageSetIDView.LastImageDetails.Image.Installer.Status == models.ImageStatusSuccess {
		// replace the BuildIsoURL with internal path
		imageSetIDView.LastImageDetails.Image.Installer.ImageBuildISOURL = GetStorageInstallerIsoURL(imageSetIDView.LastImageDetails.Image.Installer.ID)
	}

	return &imageSetIDView, nil
}

// GetImageSetImageViewByID  return image-set image view details info, the image set data, the build iso url and the image detailsInfo
func (s *ImageSetsService) GetImageSetImageViewByID(imageSetID uint, imageID uint) (*ImageSetImageIDView, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, new(OrgIDNotSet)
	}

	imageService := NewImageService(s.ctx, s.log)

	var imageSetImageIDView ImageSetImageIDView

	// get the image-set
	if result := db.Org(orgID, "").First(&imageSetImageIDView.ImageSet, imageSetID); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("image-set not found")
			return nil, new(ImageSetNotFoundError)
		}
		return nil, result.Error
	}

	var image models.Image
	// get the image-set image
	if result := db.Org(orgID, "").Order("created_at DESC").
		Preload("Packages").
		Preload("CustomPackages").
		Preload("Commit").
		Preload("Commit.Repo").
		Preload("Commit.InstalledPackages").
		Preload("Installer").
		Where("image_set_id = ?", imageSetID).First(&image, imageID); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("image-set image not found")
			return nil, new(ImageNotFoundError)
		}
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("error getting image-set image")
		return nil, result.Error
	}

	imageInfo, err := imageService.AddPackageInfo(&image)
	if err != nil {
		return nil, err
	}
	imageSetImageIDView.ImageDetails = imageInfo

	// set build iso URL
	if imageSetsInstallersMap, err := s.GetImageSetsBuildIsoURL(orgID, []uint{imageSetID}); err != nil {
		return nil, err
	} else if installerID, ok := imageSetsInstallersMap[imageSetID]; ok {
		imageSetImageIDView.ImageBuildIsoURL = GetStorageInstallerIsoURL(installerID)
	}

	if imageSetImageIDView.ImageDetails.Image.Installer != nil {
		// replace the BuildIsoURL with
		imageSetImageIDView.ImageDetails.Image.Installer.ImageBuildISOURL = GetStorageInstallerIsoURL(imageSetImageIDView.ImageDetails.Image.Installer.ID)
	}

	return &imageSetImageIDView, nil
}
