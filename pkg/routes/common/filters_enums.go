package common

import (
	"fmt"
)

// DeviceGroupsFilter object type int
type DeviceGroupsFilter int

// DevicesFilter object type int
type DevicesFilter int

// ImagesFilter object type int
type ImagesFilter int

const (
	deviceGroupName DeviceGroupsFilter = iota
	deviceGroupCreatedAt
	deviceGroupUpdatedAt
	deviceGroupSortBy
)

const (
	deviceName DevicesFilter = iota
	deviceUUID
	deviceUpdateAvailable
	deviceImageID
)

const (
	imageStatus ImagesFilter = iota
	imageName
	imageDistribution
	imageCreatedAt
	imageSortBy
)

// GetDeviceGroupsFiltersArray returns the array of the acceptable filters
func GetDeviceGroupsFiltersArray() []string {
	return []string{"name", "created_at", "updated_at", "sort_by"}
}

// GetDevicesFiltersArray returns the array of the acceptable filters
func GetDevicesFiltersArray() []string {
	return []string{"name", "uuid", "update_available", "image_id"}
}

// GetImagesFiltersArray returns the array of the acceptable filters
func GetImagesFiltersArray() []string {
	return []string{"status", "name", "distribution", "created_at", "sort_by"}
}

func (dgf DeviceGroupsFilter) String() string {
	switch dgf {
	case deviceGroupName:
		return "name"
	case deviceGroupCreatedAt:
		return "created_at"
	case deviceGroupUpdatedAt:
		return "updated_at"
	case deviceGroupSortBy:
		return "sort_by"
	default:
		return fmt.Sprintf("%d", int(dgf))
	}
}

func (dgf DevicesFilter) String() string {
	switch dgf {
	case deviceName:
		return "name"
	case deviceUUID:
		return "uuid"
	case deviceUpdateAvailable:
		return "update_available"
	case deviceImageID:
		return "image_id"
	default:
		return fmt.Sprintf("%d", int(dgf))
	}
}

func (dgf ImagesFilter) String() string {
	switch dgf {
	case imageStatus:
		return "status"
	case imageName:
		return "name"
	case imageDistribution:
		return "distribution"
	case imageCreatedAt:
		return "created_at"
	case imageSortBy:
		return "sort_by"
	default:
		return fmt.Sprintf("%d", int(dgf))
	}
}
