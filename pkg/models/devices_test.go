// FIXME: golangci-lint
// nolint:revive,typecheck
package models_test

import (
	"testing"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	// . "github.com/onsi/gomega"
	// "github.com/redhatinsights/edge-api/pkg/models"
)

var _ = Describe("Devices", func() {

})

func TestDevicesBeforeCreate(t *testing.T) {
	orgID := faker.UUIDHyphenated()
	account := faker.UUIDHyphenated()
	devices := &models.Device{
		Name:    faker.Name(),
		UUID:    faker.UUIDHyphenated(),
		OrgID:   orgID,
		Account: account,
	}

	// BeforeCreate make sure Device has orgID
	err := devices.BeforeCreate(db.DB)
	if err != nil {
		t.Error("Error running BeforeCreate")
	}
}
