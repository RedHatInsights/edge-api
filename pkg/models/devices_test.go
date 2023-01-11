// FIXME: golangci-lint
// nolint:govet,revive,typecheck
package models

import (
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestDevicesBeforeCreate(t *testing.T) {

	cases := []struct {
		Name     string
		Input    Device
		Expected error
	}{
		{
			"Missing orgID",
			Device{},
			ErrOrgIDIsMandatory,
		},
		{
			"Can be created",
			Device{
				OrgID: faker.UUIDHyphenated(),
				UUID:  faker.UUIDHyphenated(),
			},
			nil,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Input.BeforeCreate(db.DB)
			assert.Equal(t, test.Expected, got)
		})
	}
}

func TestDeviceBeforeCreateAlreadyExistDevice(t *testing.T) {
	UUId := faker.UUIDHyphenated()
	// create a device to populate database with UUId
	device := &Device{
		OrgID: faker.UUIDHyphenated(),
		UUID:  UUId,
	}
	result := db.DB.Create(&device)
	assert.Equal(t, result.Error, nil)
	// check if a new device with same UUId could be included
	newDevice := &Device{
		OrgID: faker.UUIDHyphenated(),
		UUID:  UUId,
	}
	err := newDevice.BeforeCreate(db.DB)

	assert.Error(t, err, "expected error not raised")
	assert.Equal(t, err, ErrDeviceExists, "Cannot create a device that already exists")
}
