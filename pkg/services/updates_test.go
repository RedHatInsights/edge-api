package services_test

import (
	"context"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
)

var _ = Describe("UpdateService Basic functions", func() {
	Describe("creation of the service", func() {
		Context("returns a correct instance", func() {
			ctx := context.Background()
			s := services.NewUpdateService(ctx)
			It("not to be nil", func() {
				Expect(s).ToNot(BeNil())
			})
		})
	})
	Describe("update", func() {
		var updateService services.UpdateServiceInterface
		BeforeEach(func() {
			updateService = services.NewUpdateService(context.Background())
		})
		Context("retrieve an update", func() {
			uuid := faker.UUIDHyphenated()
			uuid2 := faker.UUIDHyphenated()
			device := models.Device{
				UUID: uuid,
			}
			db.DB.Create(&device)
			device2 := models.Device{
				UUID: uuid2,
			}
			db.DB.Create(&device2)
			updates := []models.UpdateTransaction{
				{
					Devices: []models.Device{
						device,
					},
				},
				{
					Devices: []models.Device{
						device,
					},
				},
				{
					Devices: []models.Device{
						device2,
					},
				},
			}
			db.DB.Create(&updates[0])
			db.DB.Create(&updates[1])
			db.DB.Create(&updates[2])

			It("to return two updates for first device", func() {
				actual, err := updateService.GetUpdateTransactionsForDevice(&device)

				Expect(actual).ToNot(BeNil())
				Expect(*actual).To(HaveLen(2))
				Expect(err).ToNot(HaveOccurred())
			})
			It("to return one update for second device", func() {
				actual, err := updateService.GetUpdateTransactionsForDevice(&device2)

				Expect(actual).ToNot(BeNil())
				Expect(*actual).To(HaveLen(1))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
