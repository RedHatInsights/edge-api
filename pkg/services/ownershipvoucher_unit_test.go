package services

import (
	"context"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
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
			}
		})
		It("ownershipvouchers shouldn't be found", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				ov, err := ownershipVoucherService.GetOwnershipVouchersByGUID(ownershipVoucher.GUID)
				Expect(ov).To(BeNil())
				Expect(err).ToNot(BeNil())
			}
		})
		It("users shouldn't be found", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				user, err := ownershipVoucherService.GetFDOUserByGUID(ownershipVoucher.GUID)
				Expect(user).To(BeNil())
				Expect(err).ToNot(BeNil())
			}
		})
	})
})
