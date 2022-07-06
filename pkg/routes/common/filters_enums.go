package common

import (
	"fmt"
)

// DeviceGroupsFilter object type int
type DeviceGroupsFilter int

const (
	name DeviceGroupsFilter = iota
	createdat
	updatedat
	sortby
)

// GetFiltersArray returns the array of the acceptable filters
func GetFiltersArray() []string {
	return []string{"name", "created_at", "updated_at", "sort_by"}
}

func (dgf DeviceGroupsFilter) String() string {
	switch dgf {
	case name:
		return "name"
	case createdat:
		return "created_at"
	case updatedat:
		return "updated_at"
	case sortby:
		return "sort_by"
	default:
		return fmt.Sprintf("%d", int(dgf))
	}
}
