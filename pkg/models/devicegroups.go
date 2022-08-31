package models

import (
	"errors"
	"regexp"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// DeviceGroup is a record of Edge Devices Groups
// Account is the account associated with the device group
// Type is the device group type and must be "static" or "dynamic"
type DeviceGroup struct {
	Model
	Account     string   `json:"Account" gorm:"index;<-:create"`
	OrgID       string   `json:"org_id" gorm:"index"`
	Name        string   `json:"Name"`
	Type        string   `json:"Type" gorm:"default:static;<-:create"`
	Devices     []Device `faker:"-" json:"Devices" gorm:"many2many:device_groups_devices;"`
	ValidUpdate bool     `json:"ValidUpdate" gorm:"-:all"`
}

//DeviceGroupListDetail is a record of Edge Devices Groups with images and status information
type DeviceGroupListDetail struct {
	DeviceGroup     DeviceGroup        `json:"DeviceGroup"`
	DeviceImageInfo *[]DeviceImageInfo `json:"DevicesImageInfo"`
}

//DeviceImageInfo is a record of group with the current images running on the device
type DeviceImageInfo struct {
	Name            string
	Version         int
	Distribution    string
	CreatedAt       EdgeAPITime
	PackageDiff     PackageDiff
	UpdateAvailable bool
	CommitID        uint
}

// DeviceGroupDetails is a record of Device Groups and DeviceDetails
type DeviceGroupDetails struct {
	DeviceGroup   *DeviceGroup       `json:"DeviceGroup"`
	DeviceDetails *DeviceDetailsList `json:"Devices"`
}

// DeviceGroupDetailsView is a record of Device Groups and DeviceView
type DeviceGroupDetailsView struct {
	DeviceGroup   *DeviceGroup   `json:"DeviceGroup"`
	DeviceDetails DeviceViewList `json:"DevicesView"`
}

var (
	validGroupNameRegex = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
)

const (
	// DeviceGroupNameInvalidErrorMessage is the error message returned when device group name is invalid.
	DeviceGroupNameInvalidErrorMessage = "group name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// DeviceGroupNameEmptyErrorMessage is the error message returned when device group Name is empty.
	DeviceGroupNameEmptyErrorMessage = "group name cannot be empty"
	// DeviceGroupOrgIDEmptyErrorMessage is the error message returned when device group orgID is empty.
	DeviceGroupOrgIDEmptyErrorMessage = "group orgID can't be empty"
	// DeviceGroupTypeStatic correspond to the device group type value "static".
	DeviceGroupTypeStatic = "static"
	// DeviceGroupTypeDynamic correspond to the device group type value "dynamic".
	DeviceGroupTypeDynamic = "dynamic"
	// DeviceGroupTypeDefault correspond to the default device group type value.
	DeviceGroupTypeDefault = DeviceGroupTypeStatic
	// DeviceGroupTypeInvalidErrorMessage is the error message returned when device group type is invalid
	DeviceGroupTypeInvalidErrorMessage = "group type must be \"static\" or \"dynamic\""
)

// ValidateRequest validates the DeviceGroup request
func (group *DeviceGroup) ValidateRequest() error {
	if group.Name == "" {
		return errors.New(DeviceGroupNameEmptyErrorMessage)
	}
	if group.OrgID == "" {
		return errors.New(DeviceGroupOrgIDEmptyErrorMessage)
	}
	if !validGroupNameRegex.MatchString(group.Name) {
		return errors.New(DeviceGroupNameInvalidErrorMessage)
	}
	if group.Type != DeviceGroupTypeStatic && group.Type != DeviceGroupTypeDynamic {
		return errors.New(DeviceGroupTypeInvalidErrorMessage)
	}

	return nil
}

// BeforeDelete is called before deleting a device group, delete the device group devices first
func (group *DeviceGroup) BeforeDelete(tx *gorm.DB) error {
	return tx.Model(group).Association("Devices").Delete(&group.Devices)
}

// BeforeCreate method is called before creating device group, it make sure org_id is not empty
func (group *DeviceGroup) BeforeCreate(tx *gorm.DB) error {
	if group.OrgID == "" {
		log.Error("device-group do not have an org_id")
		return ErrOrgIDIsMandatory
	}

	return nil
}
