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
	log "github.com/sirupsen/logrus"

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
			s := services.NewUpdateService(ctx, log.NewEntry(log.StandardLogger()))
			It("not to be nil", func() {
				Expect(s).ToNot(BeNil())
			})
		})
	})
	Describe("update retrieval", func() {
		var updateService services.UpdateServiceInterface
		BeforeEach(func() {
			updateService = services.NewUpdateService(context.Background(), log.NewEntry(log.StandardLogger()))
		})
		Context("by device", func() {
			org_id := faker.UUIDHyphenated()
			uuid := faker.UUIDHyphenated()
			uuid2 := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: org_id,
			}
			db.DB.Create(&device)
			device2 := models.Device{
				UUID:  uuid2,
				OrgID: org_id,
			}
			db.DB.Create(&device2)
			updates := []models.UpdateTransaction{
				{
					Devices: []models.Device{
						device,
					},
					OrgID: org_id,
				},
				{
					Devices: []models.Device{
						device,
					},
					OrgID: org_id,
				},
				{
					Devices: []models.Device{
						device2,
					},
					OrgID: org_id,
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
				Service:       services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:   mockRepoBuilder,
				WaitForReboot: 0,
			}
		})
		Context("send notification", func() {
			uuid := faker.UUIDHyphenated()
			org_id := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: org_id,
			}
			db.DB.Create(&device)
			update = models.UpdateTransaction{
				Devices: []models.Device{
					device,
				},
				OrgID:  org_id,
				Status: models.UpdateStatusBuilding,
			}
			db.DB.Create(&update)
			It("should send the notification", func() {
				notify, err := updateService.SendDeviceNotification(&update)
				Expect(err).ToNot(HaveOccurred())
				Expect(notify.Version).To(Equal("v1.1.0"))
				Expect(notify.EventType).To(Equal("update-devices"))
			})
		})

		Context("from the beginning", func() {
			uuid := faker.UUIDHyphenated()
			org_id := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: org_id,
			}
			db.DB.Create(&device)
			update = models.UpdateTransaction{
				Devices: []models.Device{
					device,
				},
				OrgID:  org_id,
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
				Service: services.NewService(context.Background(), log.WithField("service", "update")),
			}

		})
		Context("when record is found and status is success", func() {
			uuid := faker.UUIDHyphenated()
			org_id := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: org_id,
			}
			db.DB.Create(&device)
			fmt.Printf("TESTETETETETETETETE :::::: %v\n", device.ID)
			d := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.UpdateStatusBuilding,
				DeviceID:             device.ID,
			}
			db.DB.Create(d)
			u := &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*d},
				OrgID:           org_id,
			}
			db.DB.Create(u)

			event := &services.PlaybookDispatcherEvent{
				Payload: services.PlaybookDispatcherEventPayload{
					ID:     d.PlaybookDispatcherID,
					Status: services.PlaybookStatusSuccess,
					OrgID:  org_id,
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
			uuid := faker.UUIDHyphenated()
			org_id := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: org_id,
			}
			db.DB.Create(&device)
			d := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.UpdateStatusBuilding,
				DeviceID:             device.ID,
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
			It("to build the template for update properly", func() {
				cfg := config.Get()
				cfg.TemplatesPath = "./../../templates/"
				t := services.TemplateRemoteInfo{
					UpdateTransactionID: 1000,
					RemoteName:          "remote-name",
					RemoteOstreeUpdate:  "false",
					OSTreeRef:           "rhel/8/x86_64/edge",
				}
				//TODO change to org_id once migration is complete
				account := "1005"
				org_id := "1005"
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", account, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/%s", fname)

				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				updateService := &services.UpdateService{
					Service:      services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService: mockFilesService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", account, fname)).Do(func(x, y string) {
					actual, err := ioutil.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := ioutil.ReadFile("./../../templates/template_playbook_dispatcher_ostree_upgrade_payload.test.yml")
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, account, org_id)

				Expect(err).ToNot(HaveOccurred())
				Expect(url).ToNot(BeNil())
				Expect(url).To(BeEquivalentTo("http://localhost:3000/api/edge/v1/updates/1000/update-playbook.yml"))
			})
		})

		Context("when upload works", func() {
			It("to build the template for rebase properly", func() {
				cfg := config.Get()
				cfg.TemplatesPath = "./../../templates/"
				t := services.TemplateRemoteInfo{
					UpdateTransactionID: 1000,
					RemoteName:          "remote-name",
					RemoteOstreeUpdate:  "true",
					OSTreeRef:           "rhel/9/x86_64/edge",
				}
				//TODO change to org_id once migration is complete
				account := "1005"
				org_id := "1005"
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", account, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/%s", fname)

				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				updateService := &services.UpdateService{
					Service:      services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService: mockFilesService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", account, fname)).Do(func(x, y string) {
					actual, err := ioutil.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := ioutil.ReadFile("./../../templates/template_playbook_dispatcher_ostree_rebase_payload.test.yml")
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, account, org_id)

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
				Service: services.NewService(context.Background(), log.WithField("service", "update")),
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

	Describe("Update Devices From Update Transaction", func() {
		account := faker.UUIDHyphenated()
		org_id := faker.UUIDHyphenated()
		imageSet := models.ImageSet{Account: account, OrgID: org_id, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)
		currentCommit := models.Commit{Account: account, OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{Account: account, OrgID: org_id, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
		db.DB.Create(&currentImage)

		newCommit := models.Commit{Account: account, OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&newCommit)
		newImage := models.Image{Account: account, OrgID: org_id, CommitID: newCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
		db.DB.Create(&newImage)

		device := models.Device{Account: account, OrgID: org_id, ImageID: currentImage.ID, UpdateAvailable: true}
		db.DB.Create(&device)
		update := models.UpdateTransaction{
			Account:  account,
			OrgID:    org_id,
			Devices:  []models.Device{device},
			CommitID: newCommit.ID,
			Status:   models.UpdateStatusBuilding,
		}
		db.DB.Create(&update)

		ctx := context.Background()
		updateService := services.NewUpdateService(ctx, log.NewEntry(log.StandardLogger()))

		Context("when update status is not success", func() {
			It("initialisation should pass", func() {
				err := updateService.UpdateDevicesFromUpdateTransaction(update)
				Expect(err).To(BeNil())
			})

			It("should not update device", func() {
				var currentDevice models.Device
				result := db.DB.First(&currentDevice, device.ID)
				Expect(result.Error).To(BeNil())

				Expect(currentDevice.ImageID).To(Equal(currentImage.ID))
				Expect(currentDevice.UpdateAvailable).To(Equal(true))

			})
		})

		Context("when update status is success", func() {
			It("initialisation should pass", func() {
				update.Status = models.UpdateStatusSuccess
				result := db.DB.Save(&update)
				Expect(result.Error).To(BeNil())

				err := updateService.UpdateDevicesFromUpdateTransaction(update)
				Expect(err).To(BeNil())
			})

			It("should update device", func() {
				var currentDevice models.Device
				result := db.DB.First(&currentDevice, device.ID)
				Expect(result.Error).To(BeNil())

				Expect(currentDevice.ImageID).To(Equal(newImage.ID))
				Expect(currentDevice.UpdateAvailable).To(Equal(false))
			})

			It("should update device image_id to update one and UpdateAvailable to true  ", func() {
				commit := models.Commit{Account: account, OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
				result := db.DB.Create(&commit)
				Expect(result.Error).To(BeNil())
				image := models.Image{Account: account, OrgID: org_id, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&image)
				Expect(result.Error).To(BeNil())

				// create a new image,  without commit as we do not need it for the current function
				lastImage := models.Image{Account: account, OrgID: org_id, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&lastImage)
				Expect(result.Error).To(BeNil())

				// create a new update with commit and image, knowing that we have a new image
				update := models.UpdateTransaction{
					Account:  account,
					OrgID:    org_id,
					Devices:  []models.Device{device},
					CommitID: commit.ID,
					Status:   models.UpdateStatusSuccess,
				}
				result = db.DB.Create(&update)
				Expect(result.Error).To(BeNil())

				err := updateService.UpdateDevicesFromUpdateTransaction(update)
				Expect(err).To(BeNil())
				var currentDevice models.Device
				result = db.DB.First(&currentDevice, device.ID)
				Expect(result.Error).To(BeNil())

				// should detect the new update commit image
				Expect(currentDevice.ImageID).To(Equal(image.ID))
				// should detect that we have newer images
				Expect(currentDevice.UpdateAvailable).To(Equal(true))
			})
		})
	})
})
