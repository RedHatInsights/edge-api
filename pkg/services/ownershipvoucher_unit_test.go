//go:build fdo
// +build fdo

package services

import (
	"context"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var _ = Describe("Ownershipvoucher unit tests", func() {
	// create Ownershipvouchers
	ownershipVoucherService := NewOwnershipVoucherService(context.Background(), log.NewEntry(log.New()))
	ownershipVouchers := []models.OwnershipVoucherData{}
	for i := 0; i < 5; i++ {
		ownershipVouchers = append(ownershipVouchers, models.OwnershipVoucherData{
			GUID:            faker.UUIDHyphenated(),
			ProtocolVersion: 100,
			DeviceName:      faker.Name(),
		})
	}
	fdoUUIDList := []string{}
	for _, ownershipVoucher := range ownershipVouchers {
		fdoUUIDList = append(fdoUUIDList, ownershipVoucher.GUID)
	}

	Context("store FDO devices", func() {
		ownershipVoucherService.storeFDODevices(ownershipVouchers)
		It("should store devices", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				fdoDevice, err := ownershipVoucherService.GetFDODeviceByGUID(ownershipVoucher.GUID)
				Expect(fdoDevice).ToNot(BeNil())
				Expect(fdoDevice.InitialUser).ToNot(BeNil())
				Expect(fdoDevice.OwnershipVoucherData.GUID).To(Equal(ownershipVoucher.GUID))
				Expect(fdoDevice.OwnershipVoucherData.ProtocolVersion).To(Equal(ownershipVoucher.ProtocolVersion))
				Expect(fdoDevice.OwnershipVoucherData.DeviceName).To(Equal(ownershipVoucher.DeviceName))
				Expect(err).To(BeNil())
			}
		})
	})

	Context("connect devices", func() {
		It("all disconnected", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				fdoDevice, err := ownershipVoucherService.GetFDODeviceByGUID(ownershipVoucher.GUID)
				Expect(fdoDevice.Connected).To(Equal(false))
				Expect(err).To(BeNil())
			}
		})
		It("should connect devices", func() {
			ownershipVoucherService.ConnectDevices(fdoUUIDList)
			for _, ownershipVoucher := range ownershipVouchers {
				fdoDevice, err := ownershipVoucherService.GetFDODeviceByGUID(ownershipVoucher.GUID)
				Expect(fdoDevice.Connected).To(Equal(true))
				Expect(err).To(BeNil())
			}
		})
	})

	Context("delete devices", func() {
		It("devices should be found", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				_, err := ownershipVoucherService.GetFDODeviceByGUID(ownershipVoucher.GUID)
				Expect(err).To(BeNil())
			}
		})
		It("should delete devices", func() {
			ownershipVoucherService.removeFDODevices(fdoUUIDList)
			for _, ownershipVoucher := range ownershipVouchers {
				device, err := ownershipVoucherService.GetFDODeviceByGUID(ownershipVoucher.GUID)
				Expect(device).To(BeNil())
				Expect(err).ToNot(BeNil())
				var deletedDevice models.FDODevice
				result := db.DB.Unscoped().Preload("OwnershipVoucherData",
					"guid = ?", ownershipVoucher.GUID).Preload("InitialUser").Find(&deletedDevice)
				Expect(result.Error).To(BeNil())
				Expect(deletedDevice.DeletedAt.Valid).To(BeTrue())
			}
		})
		It("ownershipvouchers shouldn't be found", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				var deletedDevice models.FDODevice
				result := db.DB.Unscoped().Preload("OwnershipVoucherData",
					"guid = ?", ownershipVoucher.GUID).Preload("OwnershipVoucherData",
					func(db *gorm.DB) *gorm.DB {
						return db.Unscoped()
					}).Preload("InitialUser").Find(&deletedDevice)
				Expect(result.Error).To(BeNil())
				Expect(deletedDevice.OwnershipVoucherData.DeletedAt.Valid).To(BeTrue())
			}
		})
		It("users shouldn't be found", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				var deletedDevice models.FDODevice
				result := db.DB.Unscoped().Preload("OwnershipVoucherData",
					"guid = ?", ownershipVoucher.GUID).Preload("InitialUser", func(db *gorm.DB) *gorm.DB {
					return db.Unscoped()
				}).Find(&deletedDevice)
				Expect(result.Error).To(BeNil())
				Expect(deletedDevice.InitialUser.DeletedAt.Valid).To(BeTrue())
			}
		})
		It("`BeforeDelete` works without error", func() {
			// create new ownershipvoucher
			ov := models.OwnershipVoucherData{
				GUID:            faker.UUIDHyphenated(),
				ProtocolVersion: 100,
				DeviceName:      faker.Name(),
			}
			ownershipVoucherService.storeFDODevices([]models.OwnershipVoucherData{ov})
			// check existence
			device, err := ownershipVoucherService.GetFDODeviceByGUID(ov.GUID)
			Expect(device).ToNot(BeNil())
			Expect(err).To(BeNil())
			// execute before delete
			err = device.BeforeDelete(db.DB)
			Expect(err).To(BeNil())
			// device should exist after `BeforeDelete`
			result := db.DB.Unscoped().Joins("OwnershipVoucherData").Joins("InitialUser").Find(&models.FDODevice{},
				"uuid = ?", device.UUID).First(&device)
			Expect(device).ToNot(BeNil())
			Expect(result.Error).To(BeNil())
			Expect(device.DeletedAt.Valid).To(BeFalse())                     // not deleted
			Expect(device.OwnershipVoucherData.DeletedAt.Valid).To(BeTrue()) // deleted
			Expect(device.InitialUser.DeletedAt.Valid).To(BeTrue())          // deleted
		})
	})
})
