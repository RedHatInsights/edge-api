package services

type DeviceNotFoundError struct{}

func (e *DeviceNotFoundError) Error() string {
	return "Device was not found"
}

type UpdateNotFoundError struct{}

func (e *UpdateNotFoundError) Error() string {
	return "Update was not found"
}

type ImageNotFoundError struct{}

func (e *ImageNotFoundError) Error() string {
	return "image is not found"
}

type AccountNotSet struct{}

func (e *AccountNotSet) Error() string {
	return "Account is not set"
}

type IDMustBeInteger struct{}

func (e *IDMustBeInteger) Error() string {
	return "ID needs to be an integer"
}
