package services_test

import (
	"fmt"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/services"
)

func TestDeviceNotFoundError(t *testing.T) {
	testCases := []struct {
		expectedErr error
		expectedMsg	string
	}{
		{new(services.DeviceNotFoundError), services.DeviceNotFoundErrorMsg},
		{new(services.UpdateNotFoundError), services.UpdateNotFoundErrorMsg},
		{new(services.ImageNotFoundError), services.ImageNotFoundErrorMsg},
		{new(services.ImageSetNotFoundError), services.ImageSetNotFoundErrorMsg},
		{new(services.AccountOrOrgIDNotSet), services.AccountOrOrgIDNotSetMsg},
		{new(services.AccountNotSet), services.AccountNotSetMsg},
		{new(services.OrgIDNotSet), services.OrgIDNotSetMsg},
		{new(services.IDMustBeInteger), services.IDMustBeIntegerMsg},
		{new(services.ThirdPartyRepositoryNotFound), services.ThirdPartyRepositoryNotFoundMsg},
		{new(services.ThirdPartyRepositoryAlreadyExists), services.ThirdPartyRepositoryAlreadyExistsMsg},
		{new(services.ThirdPartyRepositoryNameIsEmpty), services.ThirdPartyRepositoryNameIsEmptyMsg},
		{new(services.ThirdPartyRepositoryURLIsEmpty), services.ThirdPartyRepositoryURLIsEmptyMsg},
		{new(services.ThirdPartyRepositoryInfoIsInvalid), services.ThirdPartyRepositoryInfoIsInvalidMsg},
		{new(services.InvalidURLForCustomRepo), services.InvalidURLForCustomRepoMsg},
		{new(services.ThirdPartyRepositoryImagesExists), services.ThirdPartyRepositoryImagesExistsMsg},
		{new(services.ImageVersionAlreadyExists), services.ImageVersionAlreadyExistsMsg},
		{new(services.ImageNameAlreadyExists), services.ImageNameAlreadyExistsMsg},
		{new(services.PackageNameDoesNotExist), services.PackageNameDoesNotExistMsg},
		{new(services.ImageNameUndefined), services.ImageNameUndefinedMsg},
		{new(services.ImageSetUnDefined), services.ImageSetUnDefinedMsg},
		{new(services.ImageUnDefined), services.ImageUnDefinedMsg},
		{new(services.DeviceGroupNotFound), services.DeviceGroupNotFoundMsg},
		{new(services.ImageSetAlreadyExists), services.ImageSetAlreadyExistsMsg},
		{new(services.ImageNotInErrorState), services.ImageNotInErrorStateMsg},
		{new(services.DeviceGroupOrgIDDevicesNotFound), services.DeviceGroupOrgIDDevicesNotFoundMsg},
		{new(services.DeviceGroupDevicesNotFound), services.DeviceGroupDevicesNotFoundMsg},
		{new(services.DeviceGroupAccountOrIDUndefined), services.DeviceGroupAccountOrIDUndefinedMsg},
		{new(services.DeviceGroupDevicesNotSupplied), services.DeviceGroupDevicesNotSuppliedMsg},
		{new(services.DeviceGroupDeviceNotSupplied), services.DeviceGroupDeviceNotSuppliedMsg},
		{new(services.DeviceGroupAlreadyExists), services.DeviceGroupAlreadyExistsMsg},
		{new(services.DeviceGroupAccountOrNameUndefined), services.DeviceGroupAccountOrNameUndefinedMsg},
		{new(services.DeviceGroupMandatoryFieldsUndefined), services.DeviceGroupMandatoryFieldsUndefinedMsg},
		{new(services.DeviceHasImageUndefined), services.DeviceHasImageUndefinedMsg},
		{new(services.DeviceHasNoImageUpdate), services.DeviceHasNoImageUpdateMsg},
		{new(services.DevicesHasMoreThanOneImageSet), services.DevicesHasMoreThanOneImageSetMsg},
		{new(services.ImageHasNoImageSet), services.ImageHasNoImageSetMsg},
		{new(services.CommitNotFound), services.CommitNotFoundMsg},
		{new(services.CommitNotValid), services.CommitNotValidMsg},
		{new(services.OstreeNotFound), services.OstreeNotFoundMsg},
		{new(services.EntitiesImageSetsMismatch), services.EntitiesImageSetsMismatchMsg},
		{new(services.CommitImageNotFound), services.CommitImageNotFoundMsg},
		{new(services.SomeDevicesDoesNotExists), services.SomeDevicesDoesNotExistsMsg},
		{new(services.KafkaAllBrokersDown), services.KafkaAllBrokersDownMsg},
		{new(services.DBCommitError), services.DBCommitErrorMsg},
	}


	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Testing for %T", tc.expectedErr), func(t *testing.T) {
			errMsg := tc.expectedErr.Error()
			if errMsg != tc.expectedMsg {
				t.Errorf("Got error message '%s', Expected '%s'.", errMsg, tc.expectedMsg)
			}	
		})
	}
}