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
			DeviceUUID:      faker.UUIDHyphenated(),
		})
	}
	fdoUUIDList := []string{}
	for _, ownershipVoucher := range ownershipVouchers {
		fdoUUIDList = append(fdoUUIDList, ownershipVoucher.GUID)
	}

	Context("store Ownershipvouchers", func() {
		It("should store Ownershipvouchers", func() {
			ownershipVoucherService.storeOwnershipVouchers(ownershipVouchers)
			for _, ownershipVoucher := range ownershipVouchers {
				ov, err := ownershipVoucherService.GetOwnershipVoucherByGUID(ownershipVoucher.GUID)
				Expect(ov.GUID).To(Equal(ownershipVoucher.GUID))
				Expect(ov.ProtocolVersion).To(Equal(ownershipVoucher.ProtocolVersion))
				Expect(ov.DeviceName).To(Equal(ownershipVoucher.DeviceName))
				Expect(ov.DeviceUUID).To(Equal(ownershipVoucher.DeviceUUID))
				Expect(err).To(BeNil())
			}
		})
	})
	Context("connect devices", func() {
		It("all disconnected", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				Expect(ownershipVoucher.DeviceConnected).To(Equal(false))
			}
		})
		It("should connect devices", func() {
			ownershipVoucherService.ConnectDevices(fdoUUIDList)
			for _, ownershipVoucher := range ownershipVouchers {
				ov, err := ownershipVoucherService.GetOwnershipVoucherByGUID(ownershipVoucher.GUID)
				Expect(ov.DeviceConnected).To(Equal(true))
				Expect(err).To(BeNil())
			}
		})
	})

	Context("delete devices", func() {
		It("devices should be found", func() {
			for _, ownershipVoucher := range ownershipVouchers {
				_, err := ownershipVoucherService.GetOwnershipVoucherByGUID(ownershipVoucher.GUID)
				Expect(err).To(BeNil())
			}
		})
		It("should delete devices", func() {
			ownershipVoucherService.removeOwnershipVouchers(fdoUUIDList)
			for _, ownershipVoucher := range ownershipVouchers {
				_, err := ownershipVoucherService.GetOwnershipVoucherByGUID(ownershipVoucher.GUID)
				Expect(err).ToNot(BeNil())
			}
		})
	})
})
