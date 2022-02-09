package services

import (
	"context"
	"fmt"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
	"testing"

	"github.com/redhatinsights/edge-api/pkg/db"
)

func TestAddDeviceGroupDevices(t *testing.T) {
	deviceGroupsService := NewDeviceGroupsService(context.Background(), log.NewEntry(log.StandardLogger()))
	account1 := "1111111"
	account2 := "2222222"

	deviceGroups := []models.DeviceGroup{
		{Name: "test_group_1", Account: account1, Type: models.DeviceGroupTypeDefault},
		{Name: "test_group_2", Account: account2, Type: models.DeviceGroupTypeDefault},
	}

	devices := []models.Device{
		{Account: account1, UUID: "1"},
		{Account: account1, UUID: "2"},
		{Account: account2, UUID: "3"},
	}

	for _, deviceGroup := range deviceGroups {
		if res := db.DB.Create(&deviceGroup); res.Error != nil {
			t.Errorf("Failed to create DeviceGroup: %q", res.Error)
		}
	}
	for _, device := range devices {
		if res := db.DB.Create(&device); res.Error != nil {
			t.Errorf("Failed to create Device: %q", res.Error)
		}
	}

	var account1DeviceGroup models.DeviceGroup
	if res := db.DB.Where(models.DeviceGroup{Account: account1}).First(&account1DeviceGroup); res.Error != nil {
		t.Errorf("Failed to get device group: %q", res.Error)
	}
	var account1Devices []models.Device
	if res := db.DB.Where(models.Device{Account: account1}).Find(&account1Devices); res.Error != nil {
		t.Errorf("Failed to get Devices: %q", res.Error)
	}

	if len(account1Devices) == 0 {
		t.Errorf("account 2 Devices was not found")
	}

	addedDevices, err := deviceGroupsService.AddDeviceGroupDevices(account1, account1DeviceGroup.ID, account1Devices)
	if err != nil {
		t.Errorf(err.Error())
	}
	if addedDevices == nil {
		t.Fatal("no device added")
	}

	if len(*addedDevices) != len(account1Devices) {
		t.Errorf(fmt.Sprintf("expected the The number of added devices to be: %d but found %d", len(account1Devices), len(*addedDevices)))
	}

	for _, device := range *addedDevices {
		if device.Account != account1 {
			t.Errorf(fmt.Sprintf("expected device account to be: %s but found %s", account1, device.Account))
		}
	}

	// re-add devices
	_, err = deviceGroupsService.AddDeviceGroupDevices(account1, account1DeviceGroup.ID, account1Devices)
	if err != nil {
		t.Errorf(err.Error())
	}

	// get device group with and devices with account2
	var account2DeviceGroup models.DeviceGroup
	if res := db.DB.Where(models.DeviceGroup{Account: account2}).First(&account2DeviceGroup); res.Error != nil {
		t.Errorf("Failed to get device group: %q", res.Error)
	}
	var account2Devices []models.Device
	if res := db.DB.Where(models.Device{Account: account2}).Find(&account2Devices); res.Error != nil {
		t.Errorf("Failed to get Devices: %q", res.Error)
	}
	if len(account2Devices) == 0 {
		t.Errorf("account 2 Devices was not found")
	}

	// add devices with account2 to device group with account 1
	_, err = deviceGroupsService.AddDeviceGroupDevices(account1, account1DeviceGroup.ID, account2Devices)
	ExpectedError := "devices not found among the device group account"
	if err == nil {
		t.Errorf("Expected add devices to fail and error not nil")
	} else if err.Error() != ExpectedError {
		t.Errorf(fmt.Sprintf("Expected error : %s  , but received %s", ExpectedError, err.Error()))
	}

	// add with empty devices
	_, err = deviceGroupsService.AddDeviceGroupDevices(account1, account1DeviceGroup.ID, []models.Device{})
	ExpectedError = "devices must be supplied to be added to device group"
	if err == nil {
		t.Errorf("Expected add devices to fail and error not nil")
	} else if err.Error() != ExpectedError {
		t.Errorf(fmt.Sprintf("Expected error : %s  , but received %s", ExpectedError, err.Error()))
	}

	// add with empty account
	_, err = deviceGroupsService.AddDeviceGroupDevices("", account1DeviceGroup.ID, account1Devices)
	ExpectedError = "account or deviceGroupID undefined"
	if err == nil {
		t.Errorf("Expected add devices to fail and error not nil")
	} else if err.Error() != ExpectedError {
		t.Errorf(fmt.Sprintf("Expected error : %s  , but received %s", ExpectedError, err.Error()))
	}

	// add with empty device group id
	_, err = deviceGroupsService.AddDeviceGroupDevices(account1, 0, account1Devices)
	if err == nil {
		t.Errorf("Expected add devices to fail and error not nil")
	} else if err.Error() != ExpectedError {
		t.Errorf(fmt.Sprintf("Expected error : %s  , but received %s", ExpectedError, err.Error()))
	}
}
