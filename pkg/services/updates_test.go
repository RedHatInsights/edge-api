package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/redhatinsights/edge-api/config"
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
				Context:       context.Background(),
				RepoBuilder:   mockRepoBuilder,
				WaitForReboot: 0,
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
			db.DB.Create(&update)
			It("should return error when can't build repo", func() {
				mockRepoBuilder.EXPECT().BuildUpdateRepo(update.ID).Return(nil, errors.New("error building repo"))
				actual, err := updateService.CreateUpdate(update.ID)

				Expect(actual).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})
	Describe("playbook dispatcher event handling", func() {

		var updateService services.UpdateServiceInterface

		BeforeEach(func() {
			updateService = &services.UpdateService{
				Context: context.Background(),
			}

		})
		Context("when record is found and status is success", func() {
			d := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.UpdateStatusBuilding,
			}
			db.DB.Create(d)
			u := &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*d},
			}
			db.DB.Create(u)

			event := &services.PlaybookDispatcherEvent{
				Payload: services.PlaybookDispatcherEventPayload{
					ID:     d.PlaybookDispatcherID,
					Status: services.PlaybookStatusSuccess,
				},
			}
			message, _ := json.Marshal(event)

			It("should update status when record is found", func() {
				updateService.ProcessPlaybookDispatcherRunEvent(message)
				db.DB.First(&d, d.ID)
				Expect(d.Status).To(Equal(models.DispatchRecordStatusComplete))
			})
			It("should update status of the dispatch record", func() {
				updateService.ProcessPlaybookDispatcherRunEvent(message)
				db.DB.First(&u, u.ID)
				Expect(u.Status).To(Equal(models.UpdateStatusSuccess))
			})
			It("should set status with an error when failure is received", func() {
				event.Payload.Status = services.PlaybookStatusFailure
				message, _ := json.Marshal(event)
				err := updateService.ProcessPlaybookDispatcherRunEvent(message)
				Expect(err).To(BeNil())
				db.DB.First(&d, d.ID)
				Expect(d.Status).To(Equal(models.DispatchRecordStatusError))
			})
			It("should set status with an error when timeout is received", func() {
				event.Payload.Status = services.PlaybookStatusTimeout
				message, _ := json.Marshal(event)
				err := updateService.ProcessPlaybookDispatcherRunEvent(message)
				Expect(err).To(BeNil())
				db.DB.First(&d, d.ID)
				db.DB.First(&u, u.ID)
				Expect(d.Status).To(Equal(models.DispatchRecordStatusError))
				Expect(u.Status).To(Equal(models.UpdateStatusError))
			})
		})

		It("should give error when record is not found", func() {
			event := &services.PlaybookDispatcherEvent{
				Payload: services.PlaybookDispatcherEventPayload{
					ID:     faker.UUIDHyphenated(),
					Status: services.PlaybookStatusSuccess,
				},
			}
			message, _ := json.Marshal(event)
			err := updateService.ProcessPlaybookDispatcherRunEvent(message)
			Expect(err).To(HaveOccurred())
		})
		It("should give error when dispatch record is not found", func() {
			d := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.UpdateStatusBuilding,
			}
			db.DB.Create(d)
			event := &services.PlaybookDispatcherEvent{
				Payload: services.PlaybookDispatcherEventPayload{
					ID:     d.PlaybookDispatcherID,
					Status: services.PlaybookStatusSuccess,
				},
			}
			message, _ := json.Marshal(event)
			err := updateService.ProcessPlaybookDispatcherRunEvent(message)
			Expect(err).To(HaveOccurred())
		})
	})
	Describe("write template", func() {
		Context("when upload works", func() {
			It("to build the template properly", func() {
				cfg := config.Get()
				cfg.TemplatesPath = "./template_playbook/"
				t := services.TemplateRemoteInfo{
					UpdateTransactionID: 1000,
					RemoteName:          "remote-name",
				}
				account := "1005"
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", account, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/%s", fname)

				defer GinkgoRecover()
				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				updateService := &services.UpdateService{
					Context:      context.Background(),
					FilesService: mockFilesService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", account, fname)).Do(func(x, y string) {
					actual, err := ioutil.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := ioutil.ReadFile("./template_playbook/template_playbook_dispatcher_ostree_upgrade_payload.test.yml")
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, account)

				Expect(err).ToNot(HaveOccurred())
				Expect(url).ToNot(BeNil())
				Expect(url).To(BeEquivalentTo("http://localhost:3000/api/edge/v1/updates/1000/update-playbook.yml"))
			})
		})
	})

	Describe("Set status on update", func() {

		var updateService services.UpdateServiceInterface

		BeforeEach(func() {
			updateService = &services.UpdateService{
				Context: context.Background(),
			}

		})
		Context("when update is still processing", func() {
			d1 := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusCreated,
			}
			d2 := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusComplete,
			}
			db.DB.Create(d1)
			db.DB.Create(d2)
			u := &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*d1, *d2},
				Status:          models.UpdateStatusBuilding,
			}
			db.DB.Create(u)
			It("should keep update status", func() {
				updateService.SetUpdateStatus(u)
				db.DB.First(&u, u.ID)
				Expect(u.Status).To(Equal(models.UpdateStatusBuilding))
			})
		})
		Context("when one of the dispatch records has error", func() {
			d1 := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusError,
			}
			d2 := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusCreated,
			}
			db.DB.Create(d1)
			db.DB.Create(d2)
			u := &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*d1, *d2},
				Status:          models.UpdateStatusBuilding,
			}
			db.DB.Create(u)
			It("should set the update status as error", func() {
				updateService.SetUpdateStatus(u)
				db.DB.First(&u, u.ID)
				Expect(u.Status).To(Equal(models.UpdateStatusError))
			})
		})
		Context("when all of the dispatch records have completed", func() {
			d1 := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusComplete,
			}
			d2 := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.DispatchRecordStatusComplete,
			}
			db.DB.Create(d1)
			db.DB.Create(d2)
			u := &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*d1, *d2},
				Status:          models.UpdateStatusBuilding,
			}
			db.DB.Create(u)
			It("should set the update status as error", func() {
				updateService.SetUpdateStatus(u)
				db.DB.First(&u, u.ID)
				Expect(u.Status).To(Equal(models.UpdateStatusSuccess))
			})
		})
	})
})
