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

func TestDevicesBeforeCreateNoHaveOrgId(t *testing.T) {
	orgID := ""
	account := faker.UUIDHyphenated()
	devices := &models.Device{
		Name:    faker.Name(),
		UUID:    faker.UUIDHyphenated(),
		OrgID:   orgID,
		Account: account,
	}

	err := devices.BeforeCreate(db.DB)
	if err.Error() != "org_id is mandatory" {
		t.Error("Device create without OrgID")
	}

}
