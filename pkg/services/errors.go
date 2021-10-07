package services

type DeviceNotFoundError struct {
	error
}

func (e *DeviceNotFoundError) Error() string {
	return "Device was not found"
}

type UpdateNotFoundError struct {
	error
}

func (e *UpdateNotFoundError) Error() string {
	return "Update was not found"
}

type ImageNotFoundError struct {
	error
}

func (e *ImageNotFoundError) Error() string {
	return "image is not found"
}

type AccountNotSet struct {
	error
}

func (e *AccountNotSet) Error() string {
	return "Account is not set"
}

type IDMustBeInteger struct {
	error
}

func (e *IDMustBeInteger) Error() string {
	return "ID needs to be an integer"
}
