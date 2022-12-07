package models

import (
	"errors"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/stretchr/testify/assert"
)

func TestValidateUpdateTransactionRequest(t *testing.T) {
	cases := []struct {
		Name     string
		Input    UpdateTransaction
		Expected error
	}{
		{
			"Doesn't have devices",
			UpdateTransaction{},
			errors.New(DevicesCantBeEmptyMessage),
		},
		{
			"Has zero devices",
			UpdateTransaction{
				Devices: []Device{},
			},
			errors.New(DevicesCantBeEmptyMessage),
		},
		{
			"Has devices",
			UpdateTransaction{
				Devices: []Device{{}},
			},
			nil,
		},
	}

	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Expected, test.Input.ValidateRequest())
		})
	}
}

func TestUpdateTransactionBeforeCreate(t *testing.T) {
	cases := []struct {
		Name     string
		Input    UpdateTransaction
		Expected error
	}{
		{
			"Missing orgID",
			UpdateTransaction{},
			ErrOrgIDIsMandatory,
		},
		{
			"Has orgID",
			UpdateTransaction{OrgID: faker.UUIDHyphenated()},
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
