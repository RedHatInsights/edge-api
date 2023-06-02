// nolint:gocritic,govet,revive
package services

import (
	"context"
	"fmt"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ImageSetsServiceInterface defines the interface that helps handle
// the business logic of ImageSets
type ImageSetsServiceInterface interface {
	GetImageSetsByID(imageSetID int) (*models.ImageSet, error)
	GetImageSetsViewCount(tx *gorm.DB) (int64, error)
	GetImageSetsView(limit int, offset int, tx *gorm.DB) (*[]models.ImageSetView, error)
	GetImageSetViewByID(imageSetID uint) (*ImageSetIDView, error)
	GetImageSetsBuildIsoURL(orgID string, imageSetIDS []uint) (map[uint]uint, error)
	GetImagesViewData(imageSetID uint, imagesLimit int, imagesOffSet int, tx *gorm.DB) (*ImagesViewData, error)
	GetImageSetImageViewByID(imageSetID uint, imageID uint) (*ImageSetImageIDView, error)
	GetDeviceIdsByImageSetID(imageSetID uint) (int, []string, error)
	DeleteImageSet(imageSetID uint) error
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

// NewImageSetsService gives an instance of the main implementation of a ImageSetsServiceInterface
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

	// create a sub query of the latest images and their corresponding image sets
	latestImagesSubQuery := db.Org(orgID, "").Model(&models.Image{}).Select("image_set_id", "max(id) as image_id").Group("image_set_id")
	if result := db.OrgDB(orgID, tx, "image_sets").Table("(?) as latest_images", latestImagesSubQuery).
		Joins("JOIN images on images.id = latest_images.image_id").
		Joins("JOIN image_sets on image_sets.id = latest_images.image_set_id").
		Where("image_sets.deleted_at IS NULL").
		Count(&count); result.Error != nil {
		log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error(
			"error when getting image sets view data",
		)
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
		ID           uint               `json:"ID"`
		Name         string             `json:"Name"`
		Version      int                `json:"Version"`
		Distribution string             `json:"Distribution"`
		OutputTypes  pq.StringArray     `gorm:"type:text[]" json:"OutputTypes"`
		Status       string             `json:"Status"`
		ImageID      uint               `json:"ImageID"`
		UpdatedAt    models.EdgeAPITime `json:"UpdatedAt"`
	}

	var imageSetsRows []ImageSetRow

	// create a sub query of the latest images and their corresponding image sets
	latestImagesSubQuery := db.Org(orgID, "").Model(&models.Image{}).Select("image_set_id", "max(id) as image_id").Group("image_set_id")
	if result := db.OrgDB(orgID, tx, "image_sets").Debug().Table("(?) as latest_images", latestImagesSubQuery).Limit(limit).Offset(offset).
		Joins("JOIN images on images.id = latest_images.image_id").
		Joins("JOIN image_sets on image_sets.id = latest_images.image_set_id").
		Select("image_sets.id, image_sets.name, images.version,images.distribution, images.output_types, images.status, images.id as image_id, images.updated_at").
		Where("image_sets.deleted_at IS NULL").
		Find(&imageSetsRows); result.Error != nil {
		log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error(
			"error when getting image sets view data",
		)
		return nil, result.Error
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
			ID:           imageSetRow.ID,
			Name:         imageSetRow.Name,
			Version:      imageSetRow.Version,
			Distribution: imageSetRow.Distribution,
			OutputTypes:  imageSetRow.OutputTypes,
			UpdatedAt:    imageSetRow.UpdatedAt,
			Status:       imageSetRow.Status,
			ImageID:      imageSetRow.ImageID,
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
		Where("installers.status =  ?", models.ImageStatusSuccess).
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

// preloadImageSetImageData preload the image related data one by one instead of doing a general gorm preload.
func (s *ImageSetsService) preloadImageSetImageData(image *models.Image) error {
	if err := db.DB.Model(&image).Association("Packages").Find(&image.Packages); err != nil {
		s.log.WithField("error", err.Error()).Error("error occurred when preloading image.Packages data")
		return err
	}
	if err := db.DB.Model(&image).Association("CustomPackages").Find(&image.CustomPackages); err != nil {
		s.log.WithField("error", err.Error()).Error("error occurred when preloading image.CustomPackages data")
		return err
	}
	if image.InstallerID != nil {
		if res := db.DB.First(&image.Installer, *image.InstallerID); res.Error != nil {
			s.log.WithField("error", res.Error.Error()).Error("error occurred when preloading image.Installer data")
			return res.Error
		}
	}
	if res := db.DB.First(&image.Commit, image.CommitID); res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("error occurred when preloading image.Commit data")
		return res.Error
	}
	if image.Commit.RepoID != nil {
		if res := db.DB.First(&image.Commit.Repo, *image.Commit.RepoID); res.Error != nil {
			s.log.WithField("error", res.Error.Error()).Error("error occurred when preloading image.Commit.Repo data")
			return res.Error
		}
	}
	if err := db.DB.Model(image.Commit).Association("InstalledPackages").Find(&image.Commit.InstalledPackages); err != nil {
		s.log.WithField("error", err.Error()).Error("error occurred when preloading image.Commit.InstalledPackages data")
		return err
	}
	return nil
}

// GetImageSetViewByID return the data related to image set, data, build iso url, last image and images view.
func (s *ImageSetsService) GetImageSetViewByID(imageSetID uint) (*ImageSetIDView, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, new(OrgIDNotSet)
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
		Where("image_set_id = ?", imageSetID).First(&lastImage); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("image-set last image not found")
			return nil, new(ImageNotFoundError)
		}
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("error getting image-set last-image")
		return nil, result.Error
	}

	// seems gorm will preload all the data from all the images of the image set , that may affect performance if the image-set
	// has too many images, loading the data one by one is the only way to ensure we are loading only the needed one
	if err := s.preloadImageSetImageData(&lastImage); err != nil {
		s.log.WithField("error", err.Error()).Error("error occurred when preloading image data")
		return nil, err
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
		Where("image_set_id = ?", imageSetID).First(&image, imageID); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("image-set image not found")
			return nil, new(ImageNotFoundError)
		}
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("error getting image-set image")
		return nil, result.Error
	}

	// seems gorm will preload all the data from all the images of the image set , that may affect performance if the image-set
	// has too many images, loading the data one by one is the only way to ensure we are loading only the needed one
	if err := s.preloadImageSetImageData(&image); err != nil {
		s.log.WithField("error", err.Error()).Error("error occurred when preloading image data")
		return nil, err
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

func (s *ImageSetsService) GetDeviceIdsByImageSetID(imageSetID uint) (int, []string, error) {
	type DeviceInfo struct {
		DeviceID   uint   `json:"device_id"`
		DeviceUUID string `json:"device_uuid"`
	}
	var deviceInfos []DeviceInfo

	if result := db.DB.Table("image_sets").
		Where("image_sets.id = ? AND devices.deleted_at is NULL", imageSetID).
		Select(`image_sets.id, images.id as "image_id", devices.id as "device_id", devices.uuid as "device_uuid"`).
		Joins(`JOIN images ON image_sets.id = images.image_set_id`).
		Joins(`JOIN devices on devices.image_id = images.id`).
		Find(&deviceInfos); result.Error != nil {
		s.log.WithFields(log.Fields{"error": result.Error.Error()}).Error("Error getting devices by image set id")
		return 0, nil, result.Error
	}

	DeviceUUIDs := make([]string, 0, len(deviceInfos))
	for _, one := range deviceInfos {
		DeviceUUIDs = append(DeviceUUIDs, one.DeviceUUID)
	}

	return len(DeviceUUIDs), DeviceUUIDs, nil
}

func (s *ImageSetsService) DeleteImageSet(imageSetID uint) error {
	count, ids, err := s.GetDeviceIdsByImageSetID(imageSetID)
	if err != nil {
		s.log.WithFields(log.Fields{"error": err}).Error("Error getting devices by image set id")
		return err
	}
	if count != 0 || len(ids) != 0 {
		s.log.WithFields(log.Fields{"error": "Image Set in use"}).Error("Error, unable to delete image set in use")
		err := new(ImageSetInUse)
		return err
	}

	var imageSet models.ImageSet
	result := db.DB.First(&imageSet, imageSetID)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error getting image set by id")
		return result.Error
	}

	if err := db.DB.Where("image_set_id = ?", imageSetID).Delete(&models.Image{}).Error; err != nil {
		s.log.WithFields(log.Fields{"ImageSet_id": imageSetID, "error": err.Error}).Error("an error occurred when deleting images")
		return result.Error
	}

	if result := db.DB.Delete(&imageSet); result.Error != nil {
		s.log.WithFields(
			log.Fields{"ImageSet_id": imageSetID, "error": result.Error},
		).Error("Error when deleting image set")
		return result.Error
	}

	return nil
}
