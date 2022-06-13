package services

import "errors"

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

// OrgIDNotSet indicates the account was nil
type OrgIDNotSet struct{}

func (e *OrgIDNotSet) Error() string {
	return "Org ID is not set"
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

// ThirdPartyRepositoryAlreadyExists indicates the Third Party Repository already exists
type ThirdPartyRepositoryAlreadyExists struct{}

func (e *ThirdPartyRepositoryAlreadyExists) Error() string {
	return "custom repository already exists"
}

// ThirdPartyRepositoryNameIsEmpty indicates the Third Party Repository name is empty
type ThirdPartyRepositoryNameIsEmpty struct{}

func (e *ThirdPartyRepositoryNameIsEmpty) Error() string {
	return "custom repository name cannot be empty"
}

// ThirdPartyRepositoryURLIsEmpty indicates the Third Party Repository url is empty
type ThirdPartyRepositoryURLIsEmpty struct{}

func (e *ThirdPartyRepositoryURLIsEmpty) Error() string {
	return "custom repository URL cannot be empty"
}

// ThirdPartyRepositoryImagesExists indicates the Third Party Repository has been used in some images
type ThirdPartyRepositoryImagesExists struct{}

func (e *ThirdPartyRepositoryImagesExists) Error() string {
	return "custom repository is used by some images"
}

// ImageVersionAlreadyExists indicates the updated image version was already present
type ImageVersionAlreadyExists struct{}

func (e *ImageVersionAlreadyExists) Error() string {
	return "updated image version already exists"
}

// ImageNameAlreadyExists indicates the image with supplied name already exists
type ImageNameAlreadyExists struct{}

func (e *ImageNameAlreadyExists) Error() string {
	return "image with supplied name already exists"
}

// PackageNameDoesNotExist indicates that package name doesn't exist
type PackageNameDoesNotExist struct{}

func (e *PackageNameDoesNotExist) Error() string {
	return "package name doesn't exist"
}

// ImageNameUndefined indicates the image name is not defined
type ImageNameUndefined struct{}

func (e *ImageNameUndefined) Error() string {
	return "image name is not defined"
}

// ImageSetUnDefined indicates the image has no imageSetDefined
type ImageSetUnDefined struct{}

func (e *ImageSetUnDefined) Error() string {
	return "image-set is undefined"
}

// ImageUnDefined indicates the image is undefined in the db
type ImageUnDefined struct{}

func (e *ImageUnDefined) Error() string {
	return "image-set is undefined"
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

// DeviceHasImageUndefined indicates that device record has image not defined
type DeviceHasImageUndefined struct{}

func (e *DeviceHasImageUndefined) Error() string {
	return "device has image undefined"
}

// DeviceHasNoImageUpdate indicates that device record no image
type DeviceHasNoImageUpdate struct{}

func (e *DeviceHasNoImageUpdate) Error() string {
	return "device has no image update"
}

// DeviceHasMoreThanOneImageSet indicates that device record no image
type DeviceHasMoreThanOneImageSet struct{}

func (e *DeviceHasMoreThanOneImageSet) Error() string {
	return "device has more than one imageset"
}

// ImageHasNoImageSet indicates that device record no image
type ImageHasNoImageSet struct{}

func (e *ImageHasNoImageSet) Error() string {
	return "Image has no imageset"
}

// ErrUndefinedCommit indicate that the update transaction/image or some entity  has no commit defined.
var ErrUndefinedCommit = errors.New("entity has defined commit")

// CommitNotFound indicates commit matching the given id was not found
type CommitNotFound struct{}

func (e *CommitNotFound) Error() string {
	return "commit not found"
}

// OstreeNotFound was not found
type OstreeNotFound struct{}

func (e *OstreeNotFound) Error() string {
	return "Ostree not found"
}
