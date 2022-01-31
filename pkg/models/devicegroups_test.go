package models

import (
	"errors"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/db"
)

func TestGroupValidateRequest(t *testing.T) {
	testScenarios := []struct {
		name     string
		group    *DeviceGroup
		expected error
	}{
		{name: "Empty name", group: &DeviceGroup{Account: "111111", Type: "static"}, expected: errors.New(DeviceGroupNameEmptyErrorMessage)},
		{name: "Invalid type", group: &DeviceGroup{Name: "test_group", Account: "111111", Type: "invalid type"}, expected: errors.New(DeviceGroupTypeInvalidErrorMessage)},
		{name: "Invalid name", group: &DeviceGroup{Name: "** test group", Account: "111111", Type: DeviceGroupTypeDefault}, expected: errors.New(DeviceGroupNameInvalidErrorMessage)},
		{name: "Empty account", group: &DeviceGroup{Name: "test_group", Type: "static"}, expected: errors.New(DeviceGroupAccountEmptyErrorMessage)},
		{name: "Valid DeviceGroup", group: &DeviceGroup{Name: "test_group", Account: "111111", Type: DeviceGroupTypeDefault}, expected: nil},
	}

	for _, testScenario := range testScenarios {
		err := testScenario.group.ValidateRequest()
		if err == nil && testScenario.expected != nil {
			t.Errorf("Test %q was supposed to fail but passed successfully", testScenario.name)
		}
		if err != nil && testScenario.expected == nil {
			t.Errorf("Test %q was supposed to pass but failed: %s", testScenario.name, err)
		}
		if err != nil && testScenario.expected != nil && err.Error() != testScenario.expected.Error() {
			t.Errorf("Test %q: expected to fail on %q but got %q", testScenario.name, testScenario.expected, err)
		}
	}
}

func TestGroupCreateUpdateConstraint(t *testing.T) {
	groupInitialAccount := "111111"
	groupInitialName := "test_group"
	groupInitialType := DeviceGroupTypeDynamic

	groupNewAccount := "222222"
	groupNewType := DeviceGroupTypeStatic
	groupNewName := "new_test_group"

	group := DeviceGroup{Name: groupInitialName, Account: groupInitialAccount, Type: groupInitialType}

	err := group.ValidateRequest()
	if err != nil {
		t.Errorf("Failed to pass validation, Error: %q", err)
	}

	result := db.DB.Create(&group)
	if result.Error != nil {
		t.Errorf("Failed to create DeviceGroup: %q", result.Error)
	}

	var savedGroup DeviceGroup
	result = db.DB.First(&savedGroup, group.ID)
	if result.Error != nil {
		t.Errorf("Failed to retreive the created DeviceGroup: %q", result.Error)
	}

	savedGroup.Account = groupNewAccount
	savedGroup.Type = groupNewType
	savedGroup.Name = groupNewName

	result = db.DB.Save(&savedGroup)
	if result.Error != nil {
		t.Errorf("Failed to save the created DeviceGroup: %q", result.Error)
	}

	var updatedGroup DeviceGroup
	result = db.DB.First(&updatedGroup, group.ID)
	if result.Error != nil {
		t.Errorf("Failed to retreive the updated DeviceGroup: %q", result.Error)
	}
	// The group Account should not be updated
	if updatedGroup.Account != groupInitialAccount {
		t.Errorf("The group Account has been updated expected: %q  but found %q", groupInitialAccount, updatedGroup.Account)
	}
	// The group Type should not be updated
	if updatedGroup.Type != groupInitialType {
		t.Errorf("The group Type has been updated expected: %q  but found %q", groupInitialAccount, updatedGroup.Type)
	}
	// The DeviceGroup Name has to be updated
	if updatedGroup.Name != groupNewName {
		t.Errorf("Failed to update group name expected: %q but found: %q", groupNewName, updatedGroup.Name)
	}
}
