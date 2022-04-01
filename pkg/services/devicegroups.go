package services

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	log "github.com/sirupsen/logrus"
)

// TypeEnum define type for groups
type TypeEnum string

const (
	static  TypeEnum = models.DeviceGroupTypeStatic
	dynamic TypeEnum = models.DeviceGroupTypeDynamic
)

// DeviceGroupsServiceInterface defines the interface that helps handle
// the business logic of creating and getting device groups
type DeviceGroupsServiceInterface interface {
	CreateDeviceGroup(deviceGroup *models.DeviceGroup) (*models.DeviceGroup, error)
	GetDeviceGroups(account string, limit int, offset int, tx *gorm.DB) (*[]models.DeviceGroupListDetail, error)
	GetDeviceGroupsCount(account string, tx *gorm.DB) (int64, error)
	GetDeviceGroupByID(ID string) (*models.DeviceGroup, error)
	GetDeviceGroupDetailsByID(ID string) (*models.DeviceGroupDetails, error)
	DeleteDeviceGroupByID(ID string) error
	UpdateDeviceGroup(deviceGroup *models.DeviceGroup, account string, ID string) error
	GetDeviceGroupDeviceByID(account string, deviceGroupID uint, deviceID uint) (*models.Device, error)
	AddDeviceGroupDevices(account string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error)
	DeleteDeviceGroupDevices(account string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error)
}

// DeviceGroupsService is the main implementation of a DeviceGroupsServiceInterface
type DeviceGroupsService struct {
	Service
	DeviceService DeviceServiceInterface
}

// NewDeviceGroupsService return an instance of the main implementation of a DeviceGroupsServiceInterface
func NewDeviceGroupsService(ctx context.Context, log *log.Entry) DeviceGroupsServiceInterface {
	return &DeviceGroupsService{
		Service:       Service{ctx: ctx, log: log.WithField("service", "device-groups")},
		DeviceService: NewDeviceService(ctx, log),
	}
}

// deviceGroupNameExists check if a device group exists by account and name
func deviceGroupNameExists(account string, name string) (bool, error) {
	if account == "" || name == "" {
		return false, new(DeviceGroupAccountOrNameUndefined)
	}
	var deviceGroupsCount int64
	result := db.DB.Model(&models.DeviceGroup{}).Where(models.DeviceGroup{Account: account, Name: name}).Count(&deviceGroupsCount)
	if result.Error != nil {
		return false, result.Error
	}
	return deviceGroupsCount > 0, nil
}

// GetDeviceGroupsCount get the device groups account records count from the database
func (s *DeviceGroupsService) GetDeviceGroupsCount(account string, tx *gorm.DB) (int64, error) {

	if tx == nil {
		tx = db.DB
	}

	var count int64

	res := tx.Model(&models.DeviceGroup{}).Where("account = ?", account).Count(&count)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups count")
		return 0, res.Error
	}

	return count, nil
}

// DeleteDeviceGroupByID deletes the device group by ID from the database
func (s *DeviceGroupsService) DeleteDeviceGroupByID(ID string) error {
	sLog := s.log.WithField("device_group_id", ID)
	sLog.Info("Deleting device group")
	deviceGroup, err := s.GetDeviceGroupByID(ID) // get the device group
	if err != nil {
		sLog.WithField("error", err.Error()).Error("Error getting device group")
		return err
	}
	// delete the device group
	result := db.DB.Delete(&deviceGroup)
	if result.Error != nil {
		sLog.WithField("error", result.Error.Error()).Error("Error deleting device group")
		return result.Error
	}
	return nil
}

// GetDeviceGroups get the device groups objects from the database
func (s *DeviceGroupsService) GetDeviceGroups(account string, limit int, offset int, tx *gorm.DB) (*[]models.DeviceGroupListDetail, error) {

	if tx == nil {
		tx = db.DB
	}

	var deviceGroups []models.DeviceGroup

	res := tx.Limit(limit).Offset(offset).Where("account = ?", account).
		Preload("Devices").
		Find(&deviceGroups)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups")
		return nil, res.Error
	}

	//Getting all devices for all groups
	setOfDevices := make(map[int]models.Device)
	for _, group := range deviceGroups {
		for _, device := range group.Devices {
			setOfDevices[int(device.ID)] = device
		}
	}

	//built set of imageInfo
	setOfImages := make(map[int]models.DeviceImageInfo)
	for _, device := range setOfDevices {
		setOfImages[int(device.ImageID)] = models.DeviceImageInfo{}
	}

	//Getting image info to related images
	err := GetDeviceImageInfo(setOfImages, account)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting device image info")
		return nil, res.Error
	}

	//Concat info
	var deviceGroupListDetail []models.DeviceGroupListDetail
	for _, group := range deviceGroups {
		// var imageInfo []models.DeviceImageInfo
		imgInfo := make(map[int]models.DeviceImageInfo)
		for _, device := range group.Devices {
			imgInfo[int(device.ImageID)] = setOfImages[int(device.ImageID)]
		}
		var info []models.DeviceImageInfo
		for i := range imgInfo {
			info = append(info, imgInfo[i])
		}
		deviceGroupListDetail = append(deviceGroupListDetail,
			models.DeviceGroupListDetail{DeviceGroup: group,
				DeviceImageInfo: &info})
	}

	return &deviceGroupListDetail, nil
}

// GetDeviceImageInfo returns the image related to the groups
func GetDeviceImageInfo(images map[int]models.DeviceImageInfo, account string) error {
	for imageId := range images {
		var updAvailable bool
		var deviceImage models.Image
		var deviceImageSet models.ImageSet
		var CommitID uint
		if result := db.DB.Where(models.Image{Account: account}).
			First(&deviceImage, imageId); result.Error != nil {
			return result.Error
		}

		//should be changed to get the deviceInfo once we have the data correctly on DB
		if result := db.DB.Where(models.ImageSet{Account: account}).Preload("Images").
			First(&deviceImageSet, deviceImage.ImageSetID).Order("ID desc"); result.Error != nil {
			return result.Error
		}

		latestImage := &deviceImageSet.Images[len(deviceImageSet.Images)-1]
		latestImageID := latestImage.ID

		if int(latestImageID) > imageId {
			updAvailable = true
			CommitID = deviceImageSet.Images[len(deviceImageSet.Images)-1].CommitID
		}

		images[imageId] = models.DeviceImageInfo{
			Name:            deviceImage.Name,
			UpdateAvailable: updAvailable,
			CommitID:        CommitID}

	}
	return nil
}

//CreateDeviceGroup create a device group for an account
func (s *DeviceGroupsService) CreateDeviceGroup(deviceGroup *models.DeviceGroup) (*models.DeviceGroup, error) {
	deviceGroupExists, err := deviceGroupNameExists(deviceGroup.Account, deviceGroup.Name)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error when checking if device group exists")
		return nil, err
	}
	if deviceGroupExists {
		return nil, new(DeviceGroupAlreadyExists)
	}
	group := &models.DeviceGroup{
		Name:    deviceGroup.Name,
		Type:    string(static),
		Account: deviceGroup.Account,
	}
	result := db.DB.Create(&group)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error creating device group")
		return nil, result.Error
	}

	return group, nil
}

// GetDeviceGroupByID gets the device group by ID from the database
func (s *DeviceGroupsService) GetDeviceGroupByID(ID string) (*models.DeviceGroup, error) {
	var deviceGroup models.DeviceGroup
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return nil, new(AccountNotSet)
	}
	result := db.DB.Where("account = ? and id = ?", account, ID).Preload("Devices").First(&deviceGroup)
	if result.Error != nil {
		return nil, new(DeviceGroupNotFound)
	}
	return &deviceGroup, nil
}

// GetDeviceGroupDetailsByID gets the device group details by ID from the database
func (s *DeviceGroupsService) GetDeviceGroupDetailsByID(ID string) (*models.DeviceGroupDetails, error) {
	var deviceGroupDetails models.DeviceGroupDetails
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error account")
		return nil, err
	}
	result := db.DB.Where("account = ? and id = ?", account, ID).Preload("Devices").First(&deviceGroupDetails.DeviceGroup)
	if result.Error != nil {
		s.log.WithField("error", err.Error()).Error("Device details query error")
		return nil, new(DeviceGroupNotFound)
	}

	if len(deviceGroupDetails.DeviceGroup.Devices) > 0 {
		var devices models.DeviceDetailsList
		for _, device := range deviceGroupDetails.DeviceGroup.Devices {
			param := new(inventory.Params)
			param.HostnameOrID = device.UUID
			inventoryDevice, err := s.DeviceService.GetDevices(param)
			if err != nil {
				s.log.WithField("error", err.Error()).Error("Invetory error")
				return nil, err
			}
			if len(inventoryDevice.Devices) > 0 {
				devices.Total = devices.Total + 1
				devices.Count = devices.Count + 1
				devices.Devices = append(devices.Devices, inventoryDevice.Devices...)
			}
		}

		deviceGroupDetails.DeviceDetails = &devices
	}

	return &deviceGroupDetails, nil
}

// UpdateDeviceGroup update an existent group
func (s *DeviceGroupsService) UpdateDeviceGroup(deviceGroup *models.DeviceGroup, account string, ID string) error {
	deviceGroup.Account = account
	groupDetails, err := s.GetDeviceGroupByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving device group")
	}
	if groupDetails.Name != "" {
		groupDetails.Name = deviceGroup.Name
		deviceGroupExists, err := deviceGroupNameExists(groupDetails.Account, groupDetails.Name)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error when checking if device group exists")
			return err
		}
		if deviceGroupExists {
			return new(DeviceGroupAlreadyExists)
		}
	}

	result := db.DB.Save(&groupDetails)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetDeviceGroupDeviceByID return the device of a device group by its ID
func (s *DeviceGroupsService) GetDeviceGroupDeviceByID(account string, deviceGroupID uint, deviceID uint) (*models.Device, error) {
	if account == "" || deviceGroupID == 0 {
		s.log.Debug("account and deviceGroupID must be defined")
		return nil, new(DeviceGroupAccountOrIDUndefined)
	}

	if deviceID == 0 {
		return nil, new(DeviceGroupDeviceNotSupplied)
	}

	// get the device group
	var deviceGroup models.DeviceGroup
	if res := db.DB.Where(models.DeviceGroup{Account: account}).First(&deviceGroup, deviceGroupID); res.Error != nil {
		return nil, res.Error
	}

	// we need to be sure that all the devices we want to remove already exists and belong to device group
	var device models.Device
	if err := db.DB.Model(&deviceGroup).Association("Devices").Find(&device, deviceID); err != nil {
		return nil, err
	}

	return &device, nil
}

// AddDeviceGroupDevices add devices to device group
func (s *DeviceGroupsService) AddDeviceGroupDevices(account string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error) {
	if account == "" || deviceGroupID == 0 {
		s.log.Debug("account and deviceGroupID must be defined")
		return nil, new(DeviceGroupAccountOrIDUndefined)
	}

	if len(devices) == 0 {
		return nil, new(DeviceGroupDevicesNotSupplied)
	}

	// get the device group
	var deviceGroup models.DeviceGroup
	if res := db.DB.Where(models.DeviceGroup{Account: account}).First(&deviceGroup, deviceGroupID); res.Error != nil {
		return nil, res.Error
	}

	// get the device ids needed to be added to device group and remove duplicates and undefined
	mapDeviceIDS := make(map[uint]bool, len(devices))
	devicesIDsToAdd := make([]uint, 0, len(devices))
	for _, device := range devices {
		if _, ok := mapDeviceIDS[device.ID]; !ok && device.ID != 0 {
			mapDeviceIDS[device.ID] = true
			devicesIDsToAdd = append(devicesIDsToAdd, device.ID)
		}
	}

	// we need to be sure that all the devices we want to add already exists and have the same account as the current device group account
	var devicesToAdd []models.Device
	if res := db.DB.Where(models.Device{Account: account}).Find(&devicesToAdd, devicesIDsToAdd); res.Error != nil {
		return nil, res.Error
	}

	missingDevicesCount := len(devicesIDsToAdd) - len(devicesToAdd)
	if missingDevicesCount != 0 {
		s.log.Debug(fmt.Sprintf("devices where not found among the device group account: %d", missingDevicesCount))
		return nil, new(DeviceGroupAccountDevicesNotFound)
	}

	s.log.Debug(fmt.Sprintf("adding %d devices to device group id: %d", len(devicesToAdd), deviceGroup.ID))
	if err := db.DB.Model(&deviceGroup).Association("Devices").Append(devicesToAdd); err != nil {
		return nil, err
	}

	return &devicesToAdd, nil
}

// DeleteDeviceGroupDevices delete devices from device-group
func (s *DeviceGroupsService) DeleteDeviceGroupDevices(account string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error) {
	if account == "" || deviceGroupID == 0 {
		s.log.Debug("account and deviceGroupID must be defined")
		return nil, new(DeviceGroupAccountOrIDUndefined)
	}

	if len(devices) == 0 {
		return nil, new(DeviceGroupDevicesNotSupplied)
	}

	// get the device group
	var deviceGroup models.DeviceGroup
	if res := db.DB.Where(models.DeviceGroup{Account: account}).First(&deviceGroup, deviceGroupID); res.Error != nil {
		return nil, res.Error
	}

	// get the device ids needed to be deleted from device group, remove duplicates and undefined
	mapDeviceIDS := make(map[uint]bool, len(devices))
	devicesIDsToRemove := make([]uint, 0, len(devices))
	for _, device := range devices {
		if _, ok := mapDeviceIDS[device.ID]; !ok && device.ID != 0 {
			mapDeviceIDS[device.ID] = true
			devicesIDsToRemove = append(devicesIDsToRemove, device.ID)
		}
	}

	// we need to be sure that all the devices we want to remove already exists and belong to device group
	var devicesToRemove []models.Device
	if err := db.DB.Model(&deviceGroup).Association("Devices").Find(&devicesToRemove, devicesIDsToRemove); err != nil {
		return nil, err
	}

	missingDevicesCount := len(devicesIDsToRemove) - len(devicesToRemove)
	if len(devicesToRemove) == 0 || missingDevicesCount != 0 {
		s.log.Debug(fmt.Sprintf("devices not found in the device group: %d", missingDevicesCount))
		return nil, new(DeviceGroupDevicesNotFound)
	}

	s.log.Debug(fmt.Sprintf("removing %d devices from device group id: %d", len(devicesToRemove), deviceGroup.ID))
	if err := db.DB.Model(&deviceGroup).Association("Devices").Delete(devicesToRemove); err != nil {
		return nil, err
	}

	return &devicesToRemove, nil
}
