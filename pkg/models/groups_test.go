package models

import (
	"errors"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/db"
)

func TestGroupValidateRequest(t *testing.T) {
	testScenarios := []struct {
		name     string
		group    *Group
		expected error
	}{
		{name: "Empty name", group: &Group{Account: "111111", Type: "static"}, expected: errors.New(GroupNameEmptyErrorMessage)},
		{name: "Invalid type", group: &Group{Name: "test_group", Account: "111111", Type: "invalid type"}, expected: errors.New(GroupTypeInvalidErrorMessage)},
		{name: "Invalid name", group: &Group{Name: "** test group", Account: "111111", Type: GroupTypeDefault}, expected: errors.New(GroupNameInvalidErrorMessage)},
		{name: "Empty account", group: &Group{Name: "test_group", Type: "static"}, expected: errors.New(GroupAccountEmptyErrorMessage)},
		{name: "Valid Group", group: &Group{Name: "test_group", Account: "111111", Type: GroupTypeDefault}, expected: nil},
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
	groupInitialType := GroupTypeDynamic

	groupNewAccount := "222222"
	groupNewType := GroupTypeStatic
	groupNewName := "new_test_group"

	group := Group{Name: groupInitialName, Account: groupInitialAccount, Type: groupInitialType}

	err := group.ValidateRequest()
	if err != nil {
		t.Errorf("Failed to pass validation, Error: %q", err)
	}

	result := db.DB.Create(&group)
	if result.Error != nil {
		t.Errorf("Failed to create Group: %q", result.Error)
	}

	var savedGroup Group
	result = db.DB.First(&savedGroup, group.ID)
	if result.Error != nil {
		t.Errorf("Failed to retreive the created Group: %q", result.Error)
	}

	savedGroup.Account = groupNewAccount
	savedGroup.Type = groupNewType
	savedGroup.Name = groupNewName

	result = db.DB.Save(&savedGroup)
	if result.Error != nil {
		t.Errorf("Failed to save the created Group: %q", result.Error)
	}

	var updatedGroup Group
	result = db.DB.First(&updatedGroup, group.ID)
	if result.Error != nil {
		t.Errorf("Failed to retreive the updated Group: %q", result.Error)
	}
	// The group Account should not be updated
	if updatedGroup.Account != groupInitialAccount {
		t.Errorf("The group Account has been updated expected: %q  but found %q", groupInitialAccount, updatedGroup.Account)
	}
	// The group Type should not be updated
	if updatedGroup.Type != groupInitialType {
		t.Errorf("The group Type has been updated expected: %q  but found %q", groupInitialAccount, updatedGroup.Type)
	}
	// The Group Name has to be updated
	if updatedGroup.Name != groupNewName {
		t.Errorf("Failed to update group name expected: %q but found: %q", groupNewName, updatedGroup.Name)
	}
}
