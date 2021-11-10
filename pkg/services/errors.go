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

// ThirdPartyRepositoryNotFound indicates the Third Party Repository is not found
type ThirdPartyRepositoryNotFound struct{}

func (e *ThirdPartyRepositoryNotFound) Error() string {
	return "Third Party Repository is not found"
}
