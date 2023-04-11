// FIXME: golangci-lint
// nolint:revive
package services

import "errors"

const DeviceNotFoundErrorMsg = "Device was not found"
const UpdateNotFoundErrorMsg = "Update was not found"
const ImageNotFoundErrorMsg = "image was not found"
const ImageSetNotFoundErrorMsg = "image-set was not found"
const AccountOrOrgIDNotSetMsg = "Account or orgID is not set"
const AccountNotSetMsg = "Account is not set"
const OrgIDNotSetMsg = "Org ID is not set"
const IDMustBeIntegerMsg = "ID needs to be an integer"
const ThirdPartyRepositoryNotFoundMsg = "third party repository was not found"
const ThirdPartyRepositoryAlreadyExistsMsg = "custom repository already exists"
const ThirdPartyRepositoryWithURLAlreadyExistsMsg = "custom repository with url already exists"
const ThirdPartyRepositoryNameIsEmptyMsg = "custom repository name cannot be empty"
const ThirdPartyRepositoryURLIsEmptyMsg = "custom repository URL cannot be empty"
const ThirdPartyRepositoryInfoIsInvalidMsg = "custom repository info is invalid"
const InvalidURLForCustomRepoMsg = "invalid URL"
const ThirdPartyRepositoryImagesExistsMsg = "custom repository is used by some images"
const ImageVersionAlreadyExistsMsg = "updated image version already exists"
const ImageNameAlreadyExistsMsg = "image with supplied name already exists"
const PackageNameDoesNotExistMsg = "package name doesn't exist"
const ImageNameUndefinedMsg = "image name is not defined"
const ImageSetUnDefinedMsg = "image-set is undefined"
const ImageUnDefinedMsg = "image-set is undefined"
const DeviceGroupNotFoundMsg = "device group was not found"
const ImageSetAlreadyExistsMsg = "image set already exists"
const ImageNotInErrorStateMsg = "image is not in error state"
const ImageOnlyLatestCanModifyMsg = "only the latest updated image can be modified"
const DeviceGroupOrgIDDevicesNotFoundMsg = "devices not found among the device group orgID"
const DeviceGroupDevicesNotFoundMsg = "devices not found in device group"
const DeviceGroupAccountOrIDUndefinedMsg = "account or deviceGroupID undefined"
const DeviceGroupDevicesNotSuppliedMsg = "devices must be supplied to be added to or removed from device group"
const DeviceGroupDeviceNotSuppliedMsg = "device-group device must be supplied"
const DeviceGroupAlreadyExistsMsg = "device group already exists"
const DeviceGroupAccountOrNameUndefinedMsg = "device group account or name are undefined"
const DeviceGroupMandatoryFieldsUndefinedMsg = "device group mandatory field are undefined"
const DeviceHasImageUndefinedMsg = "device has image undefined"
const DeviceHasNoImageUpdateMsg = "device has no image update"
const DevicesHasMoreThanOneImageSetMsg = "device has more than one image-set"
const ImageHasNoImageSetMsg = "Image has no image-set"
const ImageCommitNotFoundMsg = "Image commit not found"
const ImageNameChangeIsProhibitedMsg = "image name change is prohibited in the current context"
const CommitNotFoundMsg = "Commit was not found"
const CommitNotValidMsg = "is not valid for update"
const OstreeNotFoundMsg = "Ostree not found"
const EntitiesImageSetsMismatchMsg = "does not belong to the same image-set as devices images"
const CommitImageNotFoundMsg = "Commit image was not found"
const SomeDevicesDoesNotExistsMsg = "image-set not found for all devices"
const KafkaAllBrokersDownMsg = "Cannot connect to any Kafka brokers"
const DBCommitErrorMsg = "Error searching for ImageSet of Device Images"
const KafkaProducerInstanceUndefinedMsg = "kafka producer instance is undefined"

// DeviceNotFoundError indicates the device was not found
type DeviceNotFoundError struct{}

func (e *DeviceNotFoundError) Error() string {
	return DeviceNotFoundErrorMsg
}

// UpdateNotFoundError indicates the update was not found
type UpdateNotFoundError struct{}

func (e *UpdateNotFoundError) Error() string {
	return UpdateNotFoundErrorMsg
}

// ImageNotFoundError indicates the image was not found
type ImageNotFoundError struct{}

func (e *ImageNotFoundError) Error() string {
	return ImageNotFoundErrorMsg
}

// ImageOnlyLatestCanModify indicates only the latest image can be modified
type ImageOnlyLatestCanModify struct{}

func (e *ImageOnlyLatestCanModify) Error() string {
	return ImageOnlyLatestCanModifyMsg
}

// ImageSetNotFoundError indicates the image-set was not found
type ImageSetNotFoundError struct{}

func (e *ImageSetNotFoundError) Error() string {
	return ImageSetNotFoundErrorMsg
}

// AccountOrOrgIDNotSet indicates the account or orgID was nil
type AccountOrOrgIDNotSet struct{}

func (e *AccountOrOrgIDNotSet) Error() string {
	return AccountOrOrgIDNotSetMsg
}

// AccountNotSet indicates the account was nil
type AccountNotSet struct{}

func (e *AccountNotSet) Error() string {
	return AccountNotSetMsg
}

// OrgIDNotSet indicates the account was nil
type OrgIDNotSet struct{}

func (e *OrgIDNotSet) Error() string {
	return OrgIDNotSetMsg
}

// IDMustBeInteger indicates the ID is required to be an integer value
type IDMustBeInteger struct{}

func (e *IDMustBeInteger) Error() string {
	return IDMustBeIntegerMsg
}

// ThirdPartyRepositoryNotFound indicates the Third Party Repository was not found
type ThirdPartyRepositoryNotFound struct{}

func (e *ThirdPartyRepositoryNotFound) Error() string {
	return ThirdPartyRepositoryNotFoundMsg
}

// ThirdPartyRepositoryAlreadyExists indicates the Third Party Repository already exists
type ThirdPartyRepositoryAlreadyExists struct{}

func (e *ThirdPartyRepositoryAlreadyExists) Error() string {
	return ThirdPartyRepositoryAlreadyExistsMsg
}

// ThirdPartyRepositoryWithURLAlreadyExists indicates the Third Party Repository already exists with the requested url
type ThirdPartyRepositoryWithURLAlreadyExists struct{}

func (e *ThirdPartyRepositoryWithURLAlreadyExists) Error() string {
	return ThirdPartyRepositoryWithURLAlreadyExistsMsg
}

// ThirdPartyRepositoryNameIsEmpty indicates the Third Party Repository name is empty
type ThirdPartyRepositoryNameIsEmpty struct{}

func (e *ThirdPartyRepositoryNameIsEmpty) Error() string {
	return ThirdPartyRepositoryNameIsEmptyMsg
}

// ThirdPartyRepositoryURLIsEmpty indicates the Third Party Repository url is empty
type ThirdPartyRepositoryURLIsEmpty struct{}

func (e *ThirdPartyRepositoryURLIsEmpty) Error() string {
	return ThirdPartyRepositoryURLIsEmptyMsg
}

// ThirdPartyRepositoryInfoIsInvalid indicates the Third Party Repository info is not valid
type ThirdPartyRepositoryInfoIsInvalid struct{}

func (e *ThirdPartyRepositoryInfoIsInvalid) Error() string {
	return ThirdPartyRepositoryInfoIsInvalidMsg
}

// InvalidURLForCustomRepo indicates the Third Party Repository url is invalid
type InvalidURLForCustomRepo struct{}

func (e *InvalidURLForCustomRepo) Error() string {
	return InvalidURLForCustomRepoMsg
}

// ThirdPartyRepositoryImagesExists indicates the Third Party Repository has been used in some images
type ThirdPartyRepositoryImagesExists struct{}

func (e *ThirdPartyRepositoryImagesExists) Error() string {
	return ThirdPartyRepositoryImagesExistsMsg
}

// ImageVersionAlreadyExists indicates the updated image version was already present
type ImageVersionAlreadyExists struct{}

func (e *ImageVersionAlreadyExists) Error() string {
	return ImageVersionAlreadyExistsMsg
}

// ImageNameAlreadyExists indicates the image with supplied name already exists
type ImageNameAlreadyExists struct{}

func (e *ImageNameAlreadyExists) Error() string {
	return ImageNameAlreadyExistsMsg
}

// PackageNameDoesNotExist indicates that package name doesn't exist
type PackageNameDoesNotExist struct{}

func (e *PackageNameDoesNotExist) Error() string {
	return PackageNameDoesNotExistMsg
}

// ImageNameUndefined indicates the image name is not defined
type ImageNameUndefined struct{}

func (e *ImageNameUndefined) Error() string {
	return ImageNameUndefinedMsg
}

// ImageSetUnDefined indicates the image has no imageSetDefined
type ImageSetUnDefined struct{}

func (e *ImageSetUnDefined) Error() string {
	return ImageSetUnDefinedMsg
}

// ImageUnDefined indicates the image is undefined in the db
type ImageUnDefined struct{}

func (e *ImageUnDefined) Error() string {
	return ImageUnDefinedMsg
}

// DeviceGroupNotFound indicates the Third Party Repository was not found
type DeviceGroupNotFound struct{}

func (e *DeviceGroupNotFound) Error() string {
	return DeviceGroupNotFoundMsg
}

// ImageSetAlreadyExists indicates the ImageSet attempting to be created already exists
type ImageSetAlreadyExists struct{}

func (e *ImageSetAlreadyExists) Error() string {
	return ImageSetAlreadyExistsMsg
}

// ImageNotInErrorState indicates unable to delete an image
type ImageNotInErrorState struct{}

func (e *ImageNotInErrorState) Error() string {
	return ImageNotInErrorStateMsg
}

// ImageNameChangeIsProhibited indicates that the image name was about to change, but this is not allowed
// mainly this happens when updating an image
type ImageNameChangeIsProhibited struct{}

func (e *ImageNameChangeIsProhibited) Error() string {
	return ImageNameChangeIsProhibitedMsg
}

// ImageSetInUse indicates unable to delete an image set
type ImageSetInUse struct{}

func (e *ImageSetInUse) Error() string {
	return "image set is in use"
}

// DeviceGroupOrgIDDevicesNotFound indicates that devices not found among the device group OrgID
type DeviceGroupOrgIDDevicesNotFound struct{}

func (e *DeviceGroupOrgIDDevicesNotFound) Error() string {
	return DeviceGroupOrgIDDevicesNotFoundMsg

}

// DeviceGroupDevicesNotFound indicates that devices not found in the device group collection
type DeviceGroupDevicesNotFound struct{}

func (e *DeviceGroupDevicesNotFound) Error() string {
	return DeviceGroupDevicesNotFoundMsg
}

// DeviceGroupAccountOrIDUndefined indicates that device group account or ID was not supplied
type DeviceGroupAccountOrIDUndefined struct{}

func (e *DeviceGroupAccountOrIDUndefined) Error() string {
	return DeviceGroupAccountOrIDUndefinedMsg
}

// DeviceGroupDevicesNotSupplied indicates that device group devices was not supplied
type DeviceGroupDevicesNotSupplied struct{}

func (e *DeviceGroupDevicesNotSupplied) Error() string {
	return DeviceGroupDevicesNotSuppliedMsg
}

// DeviceGroupDeviceNotSupplied indicates that device group device was not supplied
type DeviceGroupDeviceNotSupplied struct{}

func (e *DeviceGroupDeviceNotSupplied) Error() string {
	return DeviceGroupDeviceNotSuppliedMsg
}

// DeviceGroupAlreadyExists indicates that device group already exists
type DeviceGroupAlreadyExists struct{}

func (e *DeviceGroupAlreadyExists) Error() string {
	return DeviceGroupAlreadyExistsMsg
}

// DeviceGroupAccountOrNameUndefined indicates that device group account or name are undefined
type DeviceGroupAccountOrNameUndefined struct{}

func (e *DeviceGroupAccountOrNameUndefined) Error() string {
	return DeviceGroupAccountOrNameUndefinedMsg
}

// DeviceGroupMandatoryFieldsUndefined indicates that device group mandatory field are undefined
type DeviceGroupMandatoryFieldsUndefined struct{}

func (e *DeviceGroupMandatoryFieldsUndefined) Error() string {
	return DeviceGroupMandatoryFieldsUndefinedMsg
}

// DeviceHasImageUndefined indicates that device record has image not defined
type DeviceHasImageUndefined struct{}

func (e *DeviceHasImageUndefined) Error() string {
	return DeviceHasImageUndefinedMsg
}

// DeviceHasNoImageUpdate indicates that device record no image
type DeviceHasNoImageUpdate struct{}

func (e *DeviceHasNoImageUpdate) Error() string {
	return DeviceHasNoImageUpdateMsg
}

// DevicesHasMoreThanOneImageSet indicates that device record no image
type DevicesHasMoreThanOneImageSet struct{}

func (e *DevicesHasMoreThanOneImageSet) Error() string {
	return DevicesHasMoreThanOneImageSetMsg
}

// ImageHasNoImageSet indicates that device record no image
type ImageHasNoImageSet struct{}

func (e *ImageHasNoImageSet) Error() string {
	return ImageHasNoImageSetMsg
}

// ErrUndefinedCommit indicate that the update transaction/image or some entity  has no commit defined.
var ErrUndefinedCommit = errors.New("entity has defined commit")

// CommitNotFound indicates commit matching the given id was not found
type CommitNotFound struct{}

func (e *CommitNotFound) Error() string {
	return CommitNotFoundMsg
}

// CommitNotValid indicates commit matching the given id was not found
type CommitNotValid struct{}

func (e *CommitNotValid) Error() string {
	return CommitNotValidMsg
}

// OstreeNotFound was not found
type OstreeNotFound struct{}

func (e *OstreeNotFound) Error() string {
	return OstreeNotFoundMsg
}

// EntitiesImageSetsMismatch indicates the CommitID does not belong to the same ImageSet as of Device's Image
type EntitiesImageSetsMismatch struct{}

func (e *EntitiesImageSetsMismatch) Error() string {
	return EntitiesImageSetsMismatchMsg
}

// CommitImageNotFound indicates the Commit Image is not found
type CommitImageNotFound struct{}

func (e *CommitImageNotFound) Error() string {
	return CommitImageNotFoundMsg
}

// SomeDevicesDoesNotExists indicates that device record no image
type SomeDevicesDoesNotExists struct{}

func (e *SomeDevicesDoesNotExists) Error() string {
	return SomeDevicesDoesNotExistsMsg
}

// ErrOrgIDMismatch returned when the context orgID is different from an entity OrgID
var ErrOrgIDMismatch = errors.New("context org_id and entity org_id mismatch")

// KafkaAllBrokersDown indicates that the error has occured due to kafka broker issue
type KafkaAllBrokersDown struct{}

func (e *KafkaAllBrokersDown) Error() string {
	return KafkaAllBrokersDownMsg
}

// KafkaProducerInstanceUndefined indicates that we were not able to get a kafka producer instance
type KafkaProducerInstanceUndefined struct{}

func (e *KafkaProducerInstanceUndefined) Error() string {
	return KafkaProducerInstanceUndefinedMsg
}

// DBCommitError indicates a dbError during search
type DBCommitError struct{}

func (e *DBCommitError) Error() string {
	return DBCommitErrorMsg
}

// ImageCommitNotFound occurs when the image commit cannot be found
type ImageCommitNotFound struct{}

func (e *ImageCommitNotFound) Error() string {
	return ImageCommitNotFoundMsg
}
