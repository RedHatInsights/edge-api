package services_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
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
	Describe("update retrieval", func() {
		var updateService services.UpdateServiceInterface
		BeforeEach(func() {
			updateService = services.NewUpdateService(context.Background())
		})
		Context("by device", func() {
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
	Describe("update creation", func() {
		var updateService services.UpdateServiceInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var update models.UpdateTransaction
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			updateService = &services.UpdateService{
				Context:     context.Background(),
				RepoBuilder: mockRepoBuilder,
			}
		})
		Context("from the beginning", func() {
			uuid := faker.UUIDHyphenated()
			device := models.Device{
				UUID: uuid,
			}
			db.DB.Create(&device)
			update = models.UpdateTransaction{
				Devices: []models.Device{
					device,
				},
				Status: models.UpdateStatusBuilding,
			}
			tx := db.DB.Create(&update)
			fmt.Println(tx.Error)
			It("should return error when can't build repo", func() {
				mockRepoBuilder.EXPECT().BuildUpdateRepo(update.ID).Return(nil, errors.New("error building repo"))
				actual, err := updateService.CreateUpdate(update.ID)

				Expect(actual).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
