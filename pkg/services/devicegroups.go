// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package services

import (
	"context"

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
	GetDeviceGroups(orgID string, limit int, offset int, tx *gorm.DB) (*[]models.DeviceGroupListDetail, error)
	GetDeviceGroupsCount(orgID string, tx *gorm.DB) (int64, error)
	GetDeviceGroupByID(ID string) (*models.DeviceGroup, error)
	GetDeviceGroupDetailsByID(ID string) (*models.DeviceGroupDetails, error)
	DeleteDeviceGroupByID(ID string) error
	UpdateDeviceGroup(deviceGroup *models.DeviceGroup, orgID string, ID string) error
	GetDeviceGroupDeviceByID(orgID string, deviceGroupID uint, deviceID uint) (*models.Device, error)
	AddDeviceGroupDevices(orgID string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error)
	DeleteDeviceGroupDevices(orgID string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error)
	GetDeviceImageInfo(setOfImages map[int]models.DeviceImageInfo, orgID string) error
	DeviceGroupNameExists(orgID string, name string) (bool, error)
}

// DeviceGroupsService is the main implementation of a DeviceGroupsServiceInterface
type DeviceGroupsService struct {
	Service
	DeviceService DeviceServiceInterface
	UpdateService UpdateServiceInterface
}

// NewDeviceGroupsService return an instance of the main implementation of a DeviceGroupsServiceInterface
func NewDeviceGroupsService(ctx context.Context, log log.FieldLogger) DeviceGroupsServiceInterface {
	return &DeviceGroupsService{
		Service:       Service{ctx: ctx, log: log.WithField("service", "device-groups")},
		DeviceService: NewDeviceService(ctx, log),
		UpdateService: NewUpdateService(ctx, log),
	}
}

// DeviceGroupNameExists check if a device group exists by (orgID) and name
func (s *DeviceGroupsService) DeviceGroupNameExists(orgID string, name string) (bool, error) {
	if (orgID == "") || name == "" {
		return false, new(DeviceGroupMandatoryFieldsUndefined)
	}
	var deviceGroupsCount int64
	result := db.Orgx(s.ctx, orgID, "").Model(&models.DeviceGroup{}).Where("name = ?", name).Count(&deviceGroupsCount)
	if result.Error != nil {
		return false, result.Error
	}
	return deviceGroupsCount > 0, nil
}

// GetDeviceGroupsCount get the device groups by orgID records count from the database
func (s *DeviceGroupsService) GetDeviceGroupsCount(orgID string, tx *gorm.DB) (int64, error) {

	if tx == nil {
		tx = db.DBx(s.ctx)
	}

	var count int64
	res := db.OrgDB(orgID, tx, "").Model(&models.DeviceGroup{}).Count(&count)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device group count")
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
	result := db.DBx(s.ctx).Delete(&deviceGroup)
	if result.Error != nil {
		sLog.WithField("error", result.Error.Error()).Error("Error deleting device group")
		return result.Error
	}
	return nil
}

// GetDeviceGroups get the device groups objects from the database
func (s *DeviceGroupsService) GetDeviceGroups(orgID string, limit int, offset int, tx *gorm.DB) (*[]models.DeviceGroupListDetail, error) {

	if tx == nil {
		tx = db.DBx(s.ctx)
	}

	var deviceGroups []models.DeviceGroup

	res := db.OrgDB(orgID, tx, "").Limit(limit).Offset(offset).
		Preload("Devices").
		Find(&deviceGroups)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups")
		return nil, res.Error
	}

	// Getting all devices for all groups
	setOfDevices := make(map[int]models.Device)
	for _, group := range deviceGroups {
		for _, device := range group.Devices {
			setOfDevices[int(device.ID)] = device
		}
	}

	// built set of imageInfo
	setOfImages := make(map[int]models.DeviceImageInfo)
	for _, device := range setOfDevices {
		if int(device.ImageID) > 0 {
			setOfImages[int(device.ImageID)] = models.DeviceImageInfo{}
		}
	}

	// Getting image info to related images
	err := s.GetDeviceImageInfo(setOfImages, orgID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting device image info")
		return nil, res.Error
	}

	// Concat info
	var deviceGroupListDetail []models.DeviceGroupListDetail
	for _, group := range deviceGroups {
		imgInfo := make(map[int]models.DeviceImageInfo)
		for _, device := range group.Devices {
			if int(device.ImageID) > 0 {
				imgInfo[int(device.ImageID)] = setOfImages[int(device.ImageID)]
			}
		}
		var info []models.DeviceImageInfo
		imgAdded := make(map[string]bool)
		for i := range imgInfo {
			if _, ok := imgAdded[imgInfo[i].Name]; !ok {
				info = append(info, imgInfo[i])
				imgAdded[imgInfo[i].Name] = true
			}
		}
		group.ValidUpdate, err = s.UpdateService.ValidateUpdateDeviceGroup(orgID, group.ID)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error validating device group update")
		}
		deviceGroupListDetail = append(deviceGroupListDetail,
			models.DeviceGroupListDetail{DeviceGroup: group,
				DeviceImageInfo: &info})
	}

	return &deviceGroupListDetail, nil
}

// GetDeviceImageInfo returns the image related to the groups
func (s *DeviceGroupsService) GetDeviceImageInfo(images map[int]models.DeviceImageInfo, orgID string) error {
	for imageID := range images {
		if imageID > 0 {

			var updAvailable bool
			var deviceImage models.Image
			var deviceImageSet models.ImageSet
			var CommitID uint
			if result := db.Orgx(s.ctx, orgID, "").First(&deviceImage, imageID); result.Error != nil {
				return result.Error
			}

			// should be changed to get the deviceInfo once we have the data correctly on DB
			if result := db.Orgx(s.ctx, orgID, "").Preload("Images").
				First(&deviceImageSet, deviceImage.ImageSetID).Order("ID desc"); result.Error != nil {
				return result.Error
			}

			latestImage := &deviceImageSet.Images[len(deviceImageSet.Images)-1]
			latestImageID := latestImage.ID

			if int(latestImageID) > imageID && latestImage.CommitID > 0 {

				updAvailable = true
				CommitID = deviceImageSet.Images[len(deviceImageSet.Images)-1].CommitID

				// loading commit and packages to calculate diff
				if err := db.DBx(s.ctx).First(&deviceImage.Commit, deviceImage.CommitID).Error; err != nil {
					s.log.WithField("error", err.Error()).Error("Error when getting Commit for CurrentImage")
					return err
				}
				if err := db.DBx(s.ctx).Model(&deviceImage.Commit).Association("InstalledPackages").Find(&deviceImage.Commit.InstalledPackages); err != nil {
					s.log.WithField("error", err.Error()).Error("Error when getting InstalledPackages for CurrentImage")
					return err
				}

				if err := db.DBx(s.ctx).First(&latestImage.Commit, latestImage.CommitID).Error; err != nil {
					s.log.WithField("error", err.Error()).Error("Error when getting Commit for LatestImage")
					return err
				}
				if err := db.DBx(s.ctx).Model(&latestImage.Commit).Association("InstalledPackages").Find(&latestImage.Commit.InstalledPackages); err != nil {
					s.log.WithField("error", err.Error()).Error("Error when getting InstalledPackages for LatestImage")
					return err
				}

			}

			images[imageID] = models.DeviceImageInfo{
				Name:            deviceImage.Name,
				Version:         deviceImage.Version,
				Distribution:    deviceImage.Distribution,
				CreatedAt:       deviceImage.CreatedAt,
				UpdateAvailable: updAvailable,
				CommitID:        CommitID,
			}
		}
	}
	return nil
}

// CreateDeviceGroup create a device group for an ID
func (s *DeviceGroupsService) CreateDeviceGroup(deviceGroup *models.DeviceGroup) (*models.DeviceGroup, error) {
	deviceGroupExists, err := s.DeviceGroupNameExists(deviceGroup.OrgID, deviceGroup.Name)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error when checking if device group exists")
		return nil, err
	}
	if deviceGroupExists {
		return nil, new(DeviceGroupAlreadyExists)
	}
	group := &models.DeviceGroup{
		Name:  deviceGroup.Name,
		Type:  string(static),
		OrgID: deviceGroup.OrgID,
	}
	result := db.DBx(s.ctx).Create(&group)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error creating device group")
		return nil, result.Error
	}

	return group, nil
}

// GetDeviceGroupByID gets the device group by ID from the database
func (s *DeviceGroupsService) GetDeviceGroupByID(ID string) (*models.DeviceGroup, error) {
	var deviceGroup models.DeviceGroup

	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error orgID")
		return nil, err
	}

	result := db.Orgx(s.ctx, orgID, "").Where("id = ?", ID).Preload("Devices").First(&deviceGroup)
	if result.Error != nil {
		return nil, new(DeviceGroupNotFound)
	}

	deviceGroup.ValidUpdate, err = s.UpdateService.ValidateUpdateDeviceGroup(orgID, deviceGroup.ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error validating device group update")
	}

	return &deviceGroup, nil
}

// GetDeviceGroupDetailsByID gets the device group details by ID from the database
func (s *DeviceGroupsService) GetDeviceGroupDetailsByID(ID string) (*models.DeviceGroupDetails, error) {
	var deviceGroupDetails models.DeviceGroupDetails

	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error orgID")
		return nil, err
	}

	result := db.Orgx(s.ctx, orgID, "").Where("id = ?", ID).Preload("Devices").First(&deviceGroupDetails.DeviceGroup)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Device details query error")
		return nil, new(DeviceGroupNotFound)
	}

	if len(deviceGroupDetails.DeviceGroup.Devices) > 0 {
		var devices models.DeviceDetailsList
		for _, device := range deviceGroupDetails.DeviceGroup.Devices {
			param := new(inventory.Params)
			param.HostnameOrID = device.UUID
			inventoryDevice, err := s.DeviceService.GetDevices(param)
			if err != nil {
				s.log.WithField("error", err.Error()).Error("Inventory error")
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
func (s *DeviceGroupsService) UpdateDeviceGroup(deviceGroup *models.DeviceGroup, orgID string, ID string) error {
	deviceGroup.OrgID = orgID
	groupDetails, err := s.GetDeviceGroupByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving device group")
	}
	if groupDetails.Name != "" {
		groupDetails.Name = deviceGroup.Name
		deviceGroupExists, err := s.DeviceGroupNameExists(groupDetails.OrgID, groupDetails.Name)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error when checking if device group exists")
			return err
		}
		if deviceGroupExists {
			return new(DeviceGroupAlreadyExists)
		}
	}

	result := db.DBx(s.ctx).Omit("Devices").Save(&groupDetails)
	if result.Error != nil {
		return result.Error
	}

	return nil
}

// GetDeviceGroupDeviceByID return the device of a device group by its ID
func (s *DeviceGroupsService) GetDeviceGroupDeviceByID(orgID string, deviceGroupID uint, deviceID uint) (*models.Device, error) {
	if (orgID == "") || deviceGroupID == 0 {
		s.log.Debug("deviceGroupID must be defined")
		return nil, new(DeviceGroupMandatoryFieldsUndefined)
	}

	if deviceID == 0 {
		return nil, new(DeviceGroupDeviceNotSupplied)
	}

	// get the device group
	var deviceGroup models.DeviceGroup
	if res := db.Orgx(s.ctx, orgID, "").First(&deviceGroup, deviceGroupID); res.Error != nil {
		return nil, res.Error
	}

	// we need to be sure that all the devices we want to remove already exists and belong to device group
	var device models.Device
	if err := db.DBx(s.ctx).Model(&deviceGroup).Association("Devices").Find(&device, deviceID); err != nil {
		return nil, err
	}

	return &device, nil
}

// AddDeviceGroupDevices add devices to device group
func (s *DeviceGroupsService) AddDeviceGroupDevices(orgID string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error) {
	if (orgID == "") || deviceGroupID == 0 {
		s.log.Debug("deviceGroupID must be defined")
		return nil, new(DeviceGroupMandatoryFieldsUndefined)
	}

	if len(devices) == 0 {
		return nil, new(DeviceGroupDevicesNotSupplied)
	}

	// get the device group
	var deviceGroup models.DeviceGroup
	if res := db.Orgx(s.ctx, orgID, "").First(&deviceGroup, deviceGroupID); res.Error != nil {
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

	// we need to be sure that all the devices we want to add already exists and have the same ID as the current device group ID
	var devicesToAdd []models.Device
	if res := db.Orgx(s.ctx, orgID, "").Find(&devicesToAdd, devicesIDsToAdd); res.Error != nil {
		return nil, res.Error
	}

	missingDevicesCount := len(devicesIDsToAdd) - len(devicesToAdd)
	if missingDevicesCount != 0 {
		s.log.WithField("missingDevicesCount", missingDevicesCount).Debug("Some devices were not found on the ID")
		return nil, new(DeviceGroupOrgIDDevicesNotFound)
	}

	s.log.WithFields(log.Fields{"deviceCount": len(devicesToAdd), "deviceGroupID": deviceGroup.ID}).Debug("Adding devices to device group")
	if err := db.DBx(s.ctx).Model(&deviceGroup).Omit("Devices.*").Association("Devices").Append(devicesToAdd); err != nil {
		return nil, err
	}

	return &devicesToAdd, nil
}

// DeleteDeviceGroupDevices delete devices from device-group
func (s *DeviceGroupsService) DeleteDeviceGroupDevices(orgID string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error) {
	if (orgID == "") || deviceGroupID == 0 {
		s.log.Debug("org_id and deviceGroupID must be defined")
		return nil, new(DeviceGroupMandatoryFieldsUndefined)
	}

	if len(devices) == 0 {
		return nil, new(DeviceGroupDevicesNotSupplied)
	}

	// get the device group
	var deviceGroup models.DeviceGroup
	if res := db.Orgx(s.ctx, orgID, "").First(&deviceGroup, deviceGroupID); res.Error != nil {
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
	if err := db.DBx(s.ctx).Model(&deviceGroup).Association("Devices").Find(&devicesToRemove, devicesIDsToRemove); err != nil {
		return nil, err
	}

	missingDevicesCount := len(devicesIDsToRemove) - len(devicesToRemove)
	if len(devicesToRemove) == 0 || missingDevicesCount != 0 {
		s.log.WithField("missingDevicesCount", missingDevicesCount).Debug("Some devices not found in the device group")
		return nil, new(DeviceGroupDevicesNotFound)
	}

	s.log.WithFields(log.Fields{"deviceCount": len(devicesToRemove), "deviceGroupID": deviceGroup.ID}).Debug("Removing devices from device group")
	if err := db.DBx(s.ctx).Model(&deviceGroup).Association("Devices").Delete(devicesToRemove); err != nil {
		return nil, err
	}

	return &devicesToRemove, nil
}
