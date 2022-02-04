package services

import (
	"context"

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

// GetDeviceGroupByID gets the Third Party Repository by ID from the database
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
