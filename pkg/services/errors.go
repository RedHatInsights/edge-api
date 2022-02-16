package services

// DeviceNotFoundError indicates the device was not found
type DeviceNotFoundError struct{}

func (e *DeviceNotFoundError) Error() string {
	return "Device was not found"
}

// UpdateNotFoundError indicates the update was not found
type UpdateNotFoundError struct{}

func (e *UpdateNotFoundError) Error() string {
	return "Update was not found"
}

// ImageNotFoundError indicates the image was not found
type ImageNotFoundError struct{}

func (e *ImageNotFoundError) Error() string {
	return "image is not found"
}

// AccountNotSet indicates the account was nil
type AccountNotSet struct{}

func (e *AccountNotSet) Error() string {
	return "Account is not set"
}

// IDMustBeInteger indicates the ID is required to be an integer value
type IDMustBeInteger struct{}

func (e *IDMustBeInteger) Error() string {
	return "ID needs to be an integer"
}

// ThirdPartyRepositoryNotFound indicates the Third Party Repository was not found
type ThirdPartyRepositoryNotFound struct{}

func (e *ThirdPartyRepositoryNotFound) Error() string {
	return "third party repository was not found"
}

// ImageVersionAlreadyExists indicates the updated image version was already present
type ImageVersionAlreadyExists struct{}

func (e *ImageVersionAlreadyExists) Error() string {
	return "updated image version already exists"
}

// DeviceGroupNotFound indicates the Third Party Repository was not found
type DeviceGroupNotFound struct{}

func (e *DeviceGroupNotFound) Error() string {
	return "device group was not found"
}

// ImageSetAlreadyExists indicates the ImageSet attempting to be created already exists
type ImageSetAlreadyExists struct{}

func (e *ImageSetAlreadyExists) Error() string {
	return "image set already exists"
}

// DeviceGroupAccountDevicesNotFound indicates that devices not found amonf the device group account
type DeviceGroupAccountDevicesNotFound struct{}

func (e *DeviceGroupAccountDevicesNotFound) Error() string {
	return "devices not found among the device group account"

}

// DeviceGroupDevicesNotFound indicates that devices not found in the device group collection
type DeviceGroupDevicesNotFound struct{}

func (e *DeviceGroupDevicesNotFound) Error() string {
	return "devices not found in device group"
}

// DeviceGroupAccountOrIDUndefined indicates that device group account or ID was not supplied
type DeviceGroupAccountOrIDUndefined struct{}

func (e *DeviceGroupAccountOrIDUndefined) Error() string {
	return "account or deviceGroupID undefined"
}

// DeviceGroupDevicesNotSupplied indicates that device group devices was not supplied
type DeviceGroupDevicesNotSupplied struct{}

func (e *DeviceGroupDevicesNotSupplied) Error() string {
	return "devices must be supplied to be added to or removed from device group"
}

// DeviceGroupDeviceNotSupplied indicates that device group device was not supplied
type DeviceGroupDeviceNotSupplied struct{}

func (e *DeviceGroupDeviceNotSupplied) Error() string {
	return "device-group device must be supplied"
}

// DeviceGroupAlreadyExists indicates that device group already exists
type DeviceGroupAlreadyExists struct{}

func (e *DeviceGroupAlreadyExists) Error() string {
	return "device group already exists"
}

// DeviceGroupAccountOrNameUndefined indicates that device group account or name are undefined
type DeviceGroupAccountOrNameUndefined struct{}

func (e *DeviceGroupAccountOrNameUndefined) Error() string {
	return "device group account or name are undefined"
}
