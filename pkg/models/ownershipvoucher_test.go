// FIXME: golangci-lint
// nolint:revive
package models

import (
	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"gorm.io/gorm"
)

func checkFDODeviceExist(guid string) *gorm.DB {
	return db.DB.Joins("OwnershipVoucherData").Joins("InitialUser").Find(&FDODevice{},
		"guid = ?", guid)
}

var _ = Describe("Ownershipvoucher unit tests", func() {
	Context("delete devices", func() {
		It("`BeforeDelete` works without error", func() {
			// create new ownershipvoucher
			ov := OwnershipVoucherData{
				GUID:            faker.UUIDHyphenated(),
				ProtocolVersion: 100,
				DeviceName:      faker.Name(),
			}
			// store fdo device
			device := FDODevice{
				OwnershipVoucherData: &ov,
				InitialUser:          &FDOUser{},
			}
			db.DB.Create(&device)
			// check existence
			var dbDevide FDODevice
			result := checkFDODeviceExist(ov.GUID).First(&dbDevide)
			Expect(result.Error).To(BeNil())
			// execute before delete
			err := dbDevide.BeforeDelete(db.DB)
			Expect(err).To(BeNil())
		})
	})
})
