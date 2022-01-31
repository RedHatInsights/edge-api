package models

import (
	"errors"
	"regexp"
)

// Group is a record of Edge Devices Groups
// Account is the account associated with the group
// Type is the group type and must be "static" or "dynamic"
type Group struct {
	Model
	Account string   `json:"Account" gorm:"index;<-:create"`
	Name    string   `json:"Name"`
	Type    string   `json:"Type" gorm:"default:static;<-:create"`
	Devices []Device `json:"Devices" gorm:"many2many:devices_groups;"`
}

var (
	validGroupNameRegex = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
)

const (
	// GroupNameInvalidErrorMessage is the error message returned when group name is invalid.
	GroupNameInvalidErrorMessage = "group name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// GroupNameEmptyErrorMessage is the error message returned when group Name is empty.
	GroupNameEmptyErrorMessage = "group name cannot be empty"
	// GroupAccountEmptyErrorMessage is the error message returned when group account is empty.
	GroupAccountEmptyErrorMessage = "group account can't be empty"
	// GroupTypeStatic correspond to the group type value "static".
	GroupTypeStatic = "static"
	// GroupTypeDynamic correspond to the group type value "dynamic".
	GroupTypeDynamic = "dynamic"
	// GroupTypeDefault correspond to the default group type value.
	GroupTypeDefault = GroupTypeStatic
	// GroupTypeInvalidErrorMessage is the error message returned when group type is invalid
	GroupTypeInvalidErrorMessage = "group type must be \"static\" or \"dynamic\""
)

// ValidateRequest validates the Device Group request
func (group *Group) ValidateRequest() error {
	if group.Name == "" {
		return errors.New(GroupNameEmptyErrorMessage)
	}
	if group.Account == "" {
		return errors.New(GroupAccountEmptyErrorMessage)
	}
	if !validGroupNameRegex.MatchString(group.Name) {
		return errors.New(GroupNameInvalidErrorMessage)
	}
	if group.Type != GroupTypeStatic && group.Type != GroupTypeDynamic {
		return errors.New(GroupTypeInvalidErrorMessage)
	}

	return nil
}
