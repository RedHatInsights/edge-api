package services

import (
	"fmt"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

// DeviceServiceInterface defines the interface to handle the business logic of RHEL for Edge Devices
type DeviceServiceInterface interface {
	GetDeviceByID(deviceID uint) (*models.Device, error)
	GetDeviceByUUID(deviceUUID string) (*models.Device, error)
	GetUpdateAvailableForDevice(currentCheckSum string) ([]models.Image, error)
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

// GetUpdateAvailableForDevice returns if exists update for the current image at the device.
func (s *DeviceService) GetUpdateAvailableForDevice(currentCheckSum string) ([]models.Image, error) {
	var images []models.Image
	var currentImage models.Image
	result := db.DB.Joins("Commit").Where("OS_Tree_Commit = ?", currentCheckSum).First(&currentImage)
	log.Infof("currentImage :: %v \n", currentImage)
	if result.Error == nil {
		updates := db.DB.Where("Parent_Id = ?", currentImage.ID).Find(&images)
		fmt.Printf("\n Available Update:: %v \n", images)
		if updates.Error == nil {
			return images, nil
		} else {
			return nil, updates.Error
		}
	}
	return nil, result.Error
}
