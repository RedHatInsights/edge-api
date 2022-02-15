package models

import (
	"errors"
	"gorm.io/gorm"
	"regexp"
)

// DeviceGroup is a record of Edge Devices Groups
// Account is the account associated with the device group
// Type is the device group type and must be "static" or "dynamic"
type DeviceGroup struct {
	Model
	Account string   `json:"Account" gorm:"index;<-:create"`
	Name    string   `json:"Name"`
	Type    string   `json:"Type" gorm:"default:static;<-:create"`
	Devices []Device `json:"Devices" gorm:"many2many:device_groups_devices;"`
}

var (
	validGroupNameRegex = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
)

const (
	// DeviceGroupNameInvalidErrorMessage is the error message returned when device group name is invalid.
	DeviceGroupNameInvalidErrorMessage = "group name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// DeviceGroupNameEmptyErrorMessage is the error message returned when device group Name is empty.
	DeviceGroupNameEmptyErrorMessage = "group name cannot be empty"
	// DeviceGroupAccountEmptyErrorMessage is the error message returned when device group account is empty.
	DeviceGroupAccountEmptyErrorMessage = "group account can't be empty"
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
	if group.Account == "" {
		return errors.New(DeviceGroupAccountEmptyErrorMessage)
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
	return tx.Where(&Device{Account: group.Account}).Delete(&group.Devices).Error
}
