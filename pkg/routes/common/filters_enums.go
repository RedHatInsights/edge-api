package common

import (
	"fmt"
)

type DeviceGroupsFilter int

const (
	name DeviceGroupsFilter = iota
	created_at
	updated_at
)

// GetArray gets array
func GetArray() []string {
	return []string{"name", "created_at", "updated_at"}
}

func (dgf DeviceGroupsFilter) String() string {
	switch dgf {
	case name:
		return "name"
	case created_at:
		return "created_at"
	case updated_at:
		return "updated_at"
	default:
		return fmt.Sprintf("%d", int(dgf))
	}
}
