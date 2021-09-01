package services

import (
	"sort"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// DeviceServiceInterface defines the interface to handle the business logic of RHEL for Edge Devices
type DeviceServiceInterface interface {
	GetDeviceByID(deviceID uint) (*models.Device, error)
	GetDeviceByUUID(deviceUUID string) (*models.Device, error)
	GetUpdateAvailableForDevice(currentCheckSum string) ([]ImageUpdateAvailable, error)
}

// NewDeviceService gives a instance of the main implementation of DeviceServiceInterface
func NewDeviceService() DeviceServiceInterface {
	return &DeviceService{}
}

// DeviceService is the main implementation of a DeviceServiceInterface
type DeviceService struct{}

// GetDeviceByID receives DeviceID uint and get a *models.Device back
func (s *DeviceService) GetDeviceByID(deviceID uint) (*models.Device, error) {
	log.Debugf("GetDeviceByID::deviceID: %#v", deviceID)
	var device models.Device
	result := db.DB.First(&device, deviceID)
	log.Debugf("GetDeviceByID::result: %#v", result)
	log.Debugf("GetDeviceByID::device: %#v", device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

// GetDeviceByUUID receives UUID string and get a *models.Device back
func (s *DeviceService) GetDeviceByUUID(deviceUUID string) (*models.Device, error) {
	log.Debugf("GetDeviceByUUID::deviceUUID: %#v", deviceUUID)
	var device models.Device
	result := db.DB.Where("uuid = ?", deviceUUID).First(&device)
	log.Debugf("GetDeviceByUUID::result: %#v", result)
	log.Debugf("GetDeviceByUUID::device: %#v", device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

type ImageUpdateAvailable struct {
	Image       models.Image
	PackageDiff DeltaDiff
}
type DeltaDiff struct {
	Added   []models.Package
	Removed []models.Package
}

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func (s *DeviceService) GetUpdateAvailableForDevice(currentCheckSum string) ([]ImageUpdateAvailable, error) {
	var images []models.Image
	var currentImage models.Image

	result := db.DB.Joins("Commit").Where("OS_Tree_Commit = ?", currentCheckSum).First(&currentImage)
	err := db.DB.Model(&currentImage.Commit).Association("Packages").Find(&currentImage.Commit.Packages)
	if err != nil {
		return nil, errors.NewInternalServerError()
	}

	if result.Error == nil {
		updates := db.DB.Where("Parent_Id = ?", currentImage.ID).Joins("Commit").Find(&images)
		if updates.Error == nil {
			var imageDiff []ImageUpdateAvailable
			for _, upd := range images {
				db.DB.First(&upd.Commit, upd.CommitID)
				db.DB.Model(&upd.Commit).Association("Packages").Find(&upd.Commit.Packages)
				var delta ImageUpdateAvailable
				diff := GetDiffOnUpdate(currentImage, upd)
				delta.Image = upd
				delta.PackageDiff = diff
				imageDiff = append(imageDiff, delta)
			}

			return imageDiff, nil
		} else {
			return nil, updates.Error
		}
	}
	return nil, result.Error
}

func GetDiffOnUpdate(currentImage models.Image, updatedImage models.Image) DeltaDiff {
	initialCommit := currentImage.Commit.Packages
	updateCommit := updatedImage.Commit.Packages
	var initString []string
	for _, str := range initialCommit {
		initString = append(initString, str.Name)
	}
	var added []models.Package
	for _, pkg := range updateCommit {
		if !contains(initString, pkg.Name) {
			added = append(added, pkg)
		}
	}
	var updateString []string
	for _, str := range updateCommit {
		updateString = append(updateString, str.Name)
	}
	var removed []models.Package
	for _, pkg := range initialCommit {
		if !contains(updateString, pkg.Name) {
			removed = append(removed, pkg)
		}
	}
	var results DeltaDiff
	results.Added = added
	results.Removed = removed
	return results
}

func contains(s []string, searchterm string) bool {
	i := sort.SearchStrings(s, searchterm)
	return i < len(s) && s[i] == searchterm
}
