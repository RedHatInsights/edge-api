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

// TypeEnum define type for groups
type TypeEnum string

const (
	static  TypeEnum = "Static"
	dynamic TypeEnum = "Dynamic"
)

// DeviceGroupsServiceInterface defines the interface that helps handle
// the business logic of creating and getting device groups
type DeviceGroupsServiceInterface interface {
	CreateDeviceGroup(deviceGroup *models.DeviceGroup) (*models.DeviceGroup, error)
	GetDeviceGroups(account string, limit int, offset int, tx *gorm.DB) (*[]models.DeviceGroup, error)
	GetDeviceGroupsCount(account string, tx *gorm.DB) (int64, error)
	GetDeviceGroupByID(ID string) (*models.DeviceGroup, error)
	UpdateDeviceGroup(deviceGroup *models.DeviceGroup, account string, ID string) error
	AddDeviceGroupDevices(account string, deviceGroupID uint, devices []models.Device) (*[]models.Device, error)
}

// DeviceGroupsService is the main implementation of a DeviceGroupsServiceInterface
type DeviceGroupsService struct {
	Service
}

// NewDeviceGroupsService return an instance of the main implementation of a DeviceGroupsServiceInterface
func NewDeviceGroupsService(ctx context.Context, log *log.Entry) DeviceGroupsServiceInterface {
	return &DeviceGroupsService{
		Service: Service{ctx: ctx, log: log.WithField("service", "device-groups")},
	}
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

// GetDeviceGroups get the device groups objects from the database
func (s *DeviceGroupsService) GetDeviceGroups(account string, limit int, offset int, tx *gorm.DB) (*[]models.DeviceGroup, error) {

	if tx == nil {
		tx = db.DB
	}

	var deviceGroups []models.DeviceGroup

	res := tx.Limit(limit).Offset(offset).Where("account = ?", account).Find(&deviceGroups)

	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device groups")
		return nil, res.Error
	}

	return &deviceGroups, nil
}

//CreateDeviceGroup creaate a device group for an account
func (s *DeviceGroupsService) CreateDeviceGroup(deviceGroup *models.DeviceGroup) (*models.DeviceGroup, error) {
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
	result := db.DB.Where("account = ? and id = ?", account, ID).First(&deviceGroup)
	if result.Error != nil {
		return nil, new(DeviceGroupNotFound)
	}
	return &deviceGroup, nil
}

// UpdateDeviceGroup update an existent group
func (s *DeviceGroupsService) UpdateDeviceGroup(deviceGroup *models.DeviceGroup, account string, ID string) error {

	deviceGroup.Account = account
	groupDetails, err := s.GetDeviceGroupByID(ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retieving third party repository")
	}
	if groupDetails.Name != "" {
		groupDetails.Name = deviceGroup.Name
	}

	result := db.DB.Save(&groupDetails)
	if result.Error != nil {
		return result.Error
	}

	return nil
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
