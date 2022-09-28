// FIXME: golangci-lint
// nolint:errcheck,govet,revive
package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	apiError "github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher/mock_playbookdispatcher"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

var _ = Describe("UpdateService Basic functions", func() {
	cfg := config.Get()
	// backup original signing key and EdgeAPIBaseURL
	originalSigningKey := cfg.PayloadSigningKey
	originalEdgeAPIBaseURL := cfg.EdgeAPIBaseURL
	signingKey := "OJ_6Ww7BIpWAqktkelIkPDHRO6j0vtb6prME7uXXZXZLVtBjiAiHFnTK1XUv74fn"

	BeforeEach(func() {
		cfg.PayloadSigningKey = signingKey
		cfg.EdgeAPIBaseURL = "http://localhost:3000"
	})
	AfterEach(func() {
		// restore original signing key and EdgeAPIBaseURL
		cfg.PayloadSigningKey = originalSigningKey
		cfg.EdgeAPIBaseURL = originalEdgeAPIBaseURL
	})

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

			device := models.Device{
				UUID:  faker.UUIDHyphenated(),
				OrgID: org_id,
			}
			db.DB.Create(&device)
			device2 := models.Device{
				UUID:  faker.UUIDHyphenated(),
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
			db.DB.Debug().Omit("Devices.*").Create(&updates[0])
			db.DB.Debug().Omit("Devices.*").Create(&updates[1])
			db.DB.Debug().Omit("Devices.*").Create(&updates[2])

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
		var mockFilesService *mock_services.MockFilesService
		var mockPlaybookClient *mock_playbookdispatcher.MockClientInterface
		var update models.UpdateTransaction
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			defer ctrl.Finish()
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockFilesService = mock_services.NewMockFilesService(ctrl)
			mockPlaybookClient = mock_playbookdispatcher.NewMockClientInterface(ctrl)
			updateService = &services.UpdateService{
				Service:        services.NewService(context.Background(), log.WithField("service", "update")),
				FilesService:   mockFilesService,
				RepoBuilder:    mockRepoBuilder,
				PlaybookClient: mockPlaybookClient,
				WaitForReboot:  0,
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

		Context("#CreateUpdate", func() {
			var uuid string
			var device models.Device
			var update models.UpdateTransaction

			BeforeEach(func() {
				uuid = faker.UUIDHyphenated()
				device = models.Device{
					UUID:        uuid,
					OrgID:       common.DefaultOrgID,
					RHCClientID: faker.UUIDHyphenated(),
				}
				db.DB.Create(&device)
				update = models.UpdateTransaction{
					Repo: &models.Repo{URL: faker.URL()},
					Commit: &models.Commit{
						OSTreeRef: "rhel/8/x86_64/edge",
						OrgID:     common.DefaultOrgID,
					},
					Devices: []models.Device{
						device,
					},
					OrgID:  common.DefaultOrgID,
					Status: models.UpdateStatusBuilding,
				}
				db.DB.Omit("Devices.*").Create(&update)
			})

			When("when build repo fail", func() {
				It("should return error when can't build repo", func() {
					mockRepoBuilder.EXPECT().BuildUpdateRepo(update.ID).Return(nil, errors.New("error building repo"))
					actual, err := updateService.CreateUpdate(update.ID)

					Expect(actual).To(BeNil())
					Expect(err).To(HaveOccurred())
				})
			})

			When("when playbook dispatcher respond with success", func() {
				It("should create dispatcher records with status created", func() {
					cfg := config.Get()
					cfg.TemplatesPath = "./../../templates/"
					fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", update.OrgID, update.ID)
					tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", update.OrgID, fname)

					mockRepoBuilder.EXPECT().BuildUpdateRepo(update.ID).Return(&update, nil)
					mockUploader := mock_services.NewMockUploader(ctrl)
					mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", update.OrgID, fname)).Return("url", nil)
					mockFilesService.EXPECT().GetUploader().Return(mockUploader)

					playbookDispatcherID := faker.UUIDHyphenated()
					playbookURL := fmt.Sprintf("http://localhost:3000/api/edge/v1/updates/%d/update-playbook.yml", update.ID)
					mockPlaybookClient.EXPECT().ExecuteDispatcher(playbookdispatcher.DispatcherPayload{
						Recipient:    device.RHCClientID,
						PlaybookURL:  playbookURL,
						OrgID:        update.OrgID,
						PlaybookName: "Edge-management",
						Principal:    common.DefaultUserName,
					}).Return([]playbookdispatcher.Response{
						{
							StatusCode:           http.StatusCreated,
							PlaybookDispatcherID: playbookDispatcherID,
						},
					}, nil)

					updateTransaction, err := updateService.CreateUpdate(update.ID)

					Expect(err).To(BeNil())
					Expect(updateTransaction).ToNot(BeNil())
					Expect(updateTransaction.ID).Should(Equal(update.ID))
					Expect(updateTransaction.Status).Should(Equal(models.UpdateStatusBuilding))
					Expect(updateTransaction.OrgID).Should(Equal(update.OrgID))
					Expect(updateTransaction.Account).Should(Equal(update.Account))

					Expect(len(updateTransaction.DispatchRecords)).Should(Equal(1))
					Expect(updateTransaction.DispatchRecords[0].Status).Should(Equal(models.DispatchRecordStatusCreated))
					Expect(updateTransaction.DispatchRecords[0].Reason).Should(BeEmpty())
					Expect(updateTransaction.DispatchRecords[0].PlaybookDispatcherID).Should(Equal(playbookDispatcherID))
					Expect(updateTransaction.DispatchRecords[0].Device.ID).Should(Equal(device.ID))

					Expect(len(updateTransaction.Devices)).Should(Equal(1))
					Expect(updateTransaction.Devices[0].ID).Should(Equal(device.ID))
				})
			})

			When("when playbook dispatcher respond with an error", func() {
				It("should create dispatcher records with status error and reason failure", func() {
					cfg := config.Get()
					cfg.TemplatesPath = "./../../templates/"
					fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", update.OrgID, update.ID)
					tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", update.OrgID, fname)

					mockRepoBuilder.EXPECT().BuildUpdateRepo(update.ID).Return(&update, nil)
					mockUploader := mock_services.NewMockUploader(ctrl)
					mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", update.OrgID, fname)).Return("url", nil)
					mockFilesService.EXPECT().GetUploader().Return(mockUploader)

					playbookDispatcherID := faker.UUIDHyphenated()
					playbookURL := fmt.Sprintf("http://localhost:3000/api/edge/v1/updates/%d/update-playbook.yml", update.ID)
					mockPlaybookClient.EXPECT().ExecuteDispatcher(playbookdispatcher.DispatcherPayload{
						Recipient:    device.RHCClientID,
						PlaybookURL:  playbookURL,
						OrgID:        update.OrgID,
						PlaybookName: "Edge-management",
						Principal:    common.DefaultUserName,
					}).Return([]playbookdispatcher.Response{
						{
							StatusCode:           http.StatusBadRequest,
							PlaybookDispatcherID: playbookDispatcherID,
						},
					}, nil)

					updateTransaction, err := updateService.CreateUpdate(update.ID)
					Expect(updateTransaction).ToNot(BeNil())
					Expect(err).To(BeNil())
					Expect(updateTransaction).ToNot(BeNil())
					Expect(updateTransaction.ID).Should(Equal(update.ID))
					Expect(updateTransaction.Status).Should(Equal(models.UpdateStatusError))
					Expect(updateTransaction.OrgID).Should(Equal(update.OrgID))
					Expect(updateTransaction.Account).Should(Equal(update.Account))

					Expect(len(updateTransaction.DispatchRecords)).Should(Equal(1))
					Expect(updateTransaction.DispatchRecords[0].Status).Should(Equal(models.DispatchRecordStatusError))
					Expect(updateTransaction.DispatchRecords[0].Reason).Should(Equal(models.UpdateReasonFailure))
					Expect(updateTransaction.DispatchRecords[0].PlaybookDispatcherID).Should(BeEmpty())
					Expect(updateTransaction.DispatchRecords[0].Device.ID).Should(Equal(device.ID))

					Expect(len(updateTransaction.Devices)).Should(Equal(1))
					Expect(updateTransaction.Devices[0].ID).Should(Equal(device.ID))
				})
			})

			When("when playbook dispatcher client got an error", func() {
				It("should create dispatcher records with status error and reason failure", func() {
					cfg := config.Get()
					cfg.TemplatesPath = "./../../templates/"
					fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", update.OrgID, update.ID)
					tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", update.OrgID, fname)

					mockRepoBuilder.EXPECT().BuildUpdateRepo(update.ID).Return(&update, nil)
					mockUploader := mock_services.NewMockUploader(ctrl)
					mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", update.OrgID, fname)).Return("url", nil)
					mockFilesService.EXPECT().GetUploader().Return(mockUploader)

					playbookURL := fmt.Sprintf("http://localhost:3000/api/edge/v1/updates/%d/update-playbook.yml", update.ID)
					mockPlaybookClient.EXPECT().ExecuteDispatcher(playbookdispatcher.DispatcherPayload{
						Recipient:    device.RHCClientID,
						PlaybookURL:  playbookURL,
						OrgID:        update.OrgID,
						PlaybookName: "Edge-management",
						Principal:    common.DefaultUserName,
					}).Return(nil, errors.New("error on playbook dispatcher client"))

					_, err := updateService.CreateUpdate(update.ID)

					Expect(err).ShouldNot(BeNil())

					var updateTransaction models.UpdateTransaction

					db.DB.Debug().Preload("DispatchRecords").Preload("DispatchRecords.Device").Preload("Devices").First(&updateTransaction, update.ID)

					Expect(updateTransaction.ID).Should(Equal(update.ID))
					Expect(updateTransaction.Status).Should(Equal(models.UpdateStatusError))
					Expect(updateTransaction.OrgID).Should(Equal(update.OrgID))
					Expect(updateTransaction.Account).Should(Equal(update.Account))

					Expect(len(updateTransaction.DispatchRecords)).Should(Equal(1))
					Expect(updateTransaction.DispatchRecords[0].Status).Should(Equal(models.DispatchRecordStatusError))
					Expect(updateTransaction.DispatchRecords[0].Reason).Should(Equal(models.UpdateReasonFailure))
					Expect(updateTransaction.DispatchRecords[0].PlaybookDispatcherID).Should(BeEmpty())
					Expect(updateTransaction.DispatchRecords[0].Device.ID).Should(Equal(device.ID))

					Expect(len(updateTransaction.Devices)).Should(Equal(1))
					Expect(updateTransaction.Devices[0].ID).Should(Equal(device.ID))
				})
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
			d := &models.DispatchRecord{
				PlaybookDispatcherID: faker.UUIDHyphenated(),
				Status:               models.UpdateStatusBuilding,
				DeviceID:             device.ID,
			}
			db.DB.Omit("Devices.*").Create(d)
			u := &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{*d},
				OrgID:           org_id,
			}
			db.DB.Omit("Devices.*").Create(u)

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
				Expect(d.Reason).To(Equal(models.UpdateReasonFailure))
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
				Expect(d.Reason).To(Equal(models.UpdateReasonTimeout))
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
					RemoteCookieValue:   "eyJvcmdfaWQiOiIxMDA1IiwiZGV2aWNlX3V1aWQiOiIiLCJ1cGRhdGVfdHJhbnNhY3Rpb25faWQiOjEwMDB9::BKLp9JOXWyf7YmMzMMcw36tlgprO-OITJ771wxdKKlM=",
				}
				// TODO change to org_id once migration is complete
				org_id := "1005"
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", org_id, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", org_id, fname)

				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				updateService := &services.UpdateService{
					Service:      services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService: mockFilesService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", org_id, fname)).Do(func(x, y string) {
					actual, err := ioutil.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := ioutil.ReadFile("./../../templates/template_playbook_dispatcher_ostree_upgrade_payload.test.yml")
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, org_id)

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
					RemoteCookieValue:   "eyJvcmdfaWQiOiIxMDA1IiwiZGV2aWNlX3V1aWQiOiIiLCJ1cGRhdGVfdHJhbnNhY3Rpb25faWQiOjEwMDB9::BKLp9JOXWyf7YmMzMMcw36tlgprO-OITJ771wxdKKlM=",
				}
				// TODO change to org_id once migration is complete
				org_id := "1005"
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", org_id, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", org_id, fname)
				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				updateService := &services.UpdateService{
					Service:      services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService: mockFilesService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", org_id, fname)).Do(func(x, y string) {
					actual, err := ioutil.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := ioutil.ReadFile("./../../templates/template_playbook_dispatcher_ostree_rebase_payload.test.yml")
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, org_id)

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
		var update models.UpdateTransaction
		var device models.Device
		var currentImage models.Image
		var newImage models.Image
		var org_id string
		var imageSet models.ImageSet
		BeforeEach(func() {

			org_id = faker.UUIDHyphenated()
			imageSet = models.ImageSet{OrgID: org_id, Name: faker.UUIDHyphenated()}
			db.DB.Create(&imageSet)
			currentCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&currentCommit)
			currentImage = models.Image{OrgID: org_id, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
			db.DB.Create(&currentImage)

			newCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&newCommit)
			newImage = models.Image{OrgID: org_id, CommitID: newCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
			db.DB.Create(&newImage)

			device = models.Device{OrgID: org_id, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
			db.DB.Create(&device)
			update = models.UpdateTransaction{

				OrgID:    org_id,
				Devices:  []models.Device{device},
				CommitID: newCommit.ID,
				Status:   models.UpdateStatusBuilding,
			}
			db.DB.Create(&update)

		})
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
			It("initialisation and update should pass", func() {
				update.Status = models.UpdateStatusSuccess
				result := db.DB.Omit("Devices.*").Save(&update)
				Expect(result.Error).To(BeNil())

				err := updateService.UpdateDevicesFromUpdateTransaction(update)
				Expect(err).To(BeNil())
				var currentDevice models.Device
				result = db.DB.First(&currentDevice, device.ID)
				Expect(result.Error).To(BeNil())

				Expect(currentDevice.ImageID).To(Equal(newImage.ID))
				Expect(currentDevice.UpdateAvailable).To(Equal(false))
			})

			It("should update device image_id to update one and UpdateAvailable to true  ", func() {
				commit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
				result := db.DB.Create(&commit)
				Expect(result.Error).To(BeNil())
				image := models.Image{OrgID: org_id, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&image)
				Expect(result.Error).To(BeNil())

				// create a new image,  without commit as we do not need it for the current function
				lastImage := models.Image{OrgID: org_id, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&lastImage)
				Expect(result.Error).To(BeNil())

				// create a new update with commit and image, knowing that we have a new image
				update := models.UpdateTransaction{

					OrgID:    org_id,
					Devices:  []models.Device{device},
					CommitID: commit.ID,
					Status:   models.UpdateStatusSuccess,
				}
				result = db.DB.Omit("Devices.*").Create(&update)
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
	Describe("Update Devices from version 1 to version 3", func() {

		org_id := faker.UUIDHyphenated()
		var updateService services.UpdateServiceInterface

		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface

		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			updateService = &services.UpdateService{
				Service:       services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:   mockRepoBuilder,
				Inventory:     mockInventoryClient,
				ImageService:  mockImageService,
				WaitForReboot: 0,
			}
		})

		imageSet := models.ImageSet{OrgID: org_id, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: org_id, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-86"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: true}
		db.DB.Create(&commit)
		image := models.Image{OrgID: org_id, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		latestCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&latestCommit)
		latestImage := models.Image{OrgID: org_id, CommitID: latestCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&latestImage)

		device := models.Device{OrgID: org_id, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{

			OrgID:    org_id,
			Devices:  []models.Device{device},
			CommitID: latestCommit.ID,
			Commit:   &latestCommit,
			RepoID:   &repo.ID,
			Repo:     &repo,
			Status:   models.UpdateStatusBuilding,
		}
		db.DB.Create(&update)

		var devicesUpdate models.DevicesUpdate
		devicesUpdate.DevicesUUID = append(devicesUpdate.DevicesUUID, device.UUID)
		devicesUpdate.CommitID = latestCommit.ID

		Context("when update change Refs success", func() {

			It("initialisation should pass", func() {
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: currentCommit.OSTreeCommit, Booted: true},
						},
					},
						OrgID: org_id,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(commit.OSTreeCommit).Return(&image, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(latestCommit.OSTreeCommit).Return(&latestImage, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, org_id, &commit)
				Expect(err).To(BeNil())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeTrue())
				}
			})
		})
	})

	Describe("Update Devices to same distribution", func() {
		org_id := faker.UUIDHyphenated()
		var updateService services.UpdateServiceInterface

		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface

		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			updateService = &services.UpdateService{
				Service:       services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:   mockRepoBuilder,
				Inventory:     mockInventoryClient,
				ImageService:  mockImageService,
				WaitForReboot: 0,
			}
		})

		imageSet := models.ImageSet{OrgID: org_id, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: org_id, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&commit)
		image := models.Image{OrgID: org_id, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		latestCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&latestCommit)
		latestImage := models.Image{OrgID: org_id, CommitID: latestCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&latestImage)

		device := models.Device{OrgID: org_id, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{

			OrgID:    org_id,
			Devices:  []models.Device{device},
			CommitID: latestCommit.ID,
			Commit:   &latestCommit,
			RepoID:   &repo.ID,
			Repo:     &repo,
			Status:   models.UpdateStatusBuilding,
		}
		db.DB.Create(&update)

		var devicesUpdate models.DevicesUpdate
		devicesUpdate.DevicesUUID = append(devicesUpdate.DevicesUUID, device.UUID)
		devicesUpdate.CommitID = latestCommit.ID

		Context("when update change Refs success", func() {

			It("initialisation should pass", func() {
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: currentCommit.OSTreeCommit, Booted: true},
						},
					},
						OrgID: org_id,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(commit.OSTreeCommit).Return(&image, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(latestCommit.OSTreeCommit).Return(&latestImage, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, org_id, &commit)
				Expect(err).To(BeNil())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeFalse())
				}
			})
		})
	})

	Describe("Update Devices from 1 to 2", func() {
		org_id := faker.UUIDHyphenated()
		var updateService services.UpdateServiceInterface

		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface

		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			updateService = &services.UpdateService{
				Service:       services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:   mockRepoBuilder,
				Inventory:     mockInventoryClient,
				ImageService:  mockImageService,
				WaitForReboot: 0,
			}
		})

		imageSet := models.ImageSet{OrgID: org_id, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: org_id, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-86"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: org_id, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&commit)
		image := models.Image{OrgID: org_id, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		device := models.Device{OrgID: org_id, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{
			OrgID:    org_id,
			Devices:  []models.Device{device},
			CommitID: commit.ID,
			Commit:   &commit,
			RepoID:   &repo.ID,
			Repo:     &repo,
			Status:   models.UpdateStatusBuilding,
		}
		db.DB.Omit("Devices.*").Create(&update)

		var devicesUpdate models.DevicesUpdate
		devicesUpdate.DevicesUUID = append(devicesUpdate.DevicesUUID, device.UUID)
		devicesUpdate.CommitID = commit.ID

		Context("when update change Refs success", func() {

			It("initialisation should pass", func() {
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: currentCommit.OSTreeCommit, Booted: true},
						},
					},
						OrgID: org_id,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(commit.OSTreeCommit).Return(&image, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, org_id, &commit)
				Expect(err).To(BeNil())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeTrue())
				}
			})
		})
	})

	Describe("Create Update Transaction", func() {
		orgId := common.DefaultOrgID
		rhcClientId := faker.UUIDHyphenated()
		var imageSet models.ImageSet
		var currentCommit models.Commit
		var currentImage models.Image
		var newCommit models.Commit
		var newImage models.Image
		var device models.Device
		var device2 models.Device

		var updateService services.UpdateServiceInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var mockInventory *mock_inventory.MockClientInterface

		BeforeEach(func() {
			ctrl := gomock.NewController(GinkgoT())
			defer ctrl.Finish()
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockInventory = mock_inventory.NewMockClientInterface(ctrl)
			updateService = &services.UpdateService{
				Service:       services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:   mockRepoBuilder,
				Inventory:     mockInventory,
				WaitForReboot: 0,
			}

			imageSet = models.ImageSet{OrgID: orgId, Name: faker.UUIDHyphenated()}
			db.DB.Create(&imageSet)
			currentCommit = models.Commit{OrgID: orgId, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&currentCommit)
			currentImage = models.Image{OrgID: orgId, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
			db.DB.Create(&currentImage)
			newCommit = models.Commit{OrgID: orgId, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&newCommit)
			newImage = models.Image{OrgID: orgId, CommitID: newCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
			db.DB.Create(&newImage)
			device = models.Device{OrgID: orgId, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated(), RHCClientID: rhcClientId}
			db.DB.Create(&device)
			device2 = models.Device{OrgID: orgId, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
			db.DB.Create(&device2)
		})

		Context("when device has rhc_client_id", func() {
			It("should create an update transaction with a repo", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}

				responseInventory := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: rhcClientId,
					}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, common.DefaultOrgID, &newCommit)

				Expect(err).To(BeNil())
				Expect(len(*updates)).Should(Equal(1))
				Expect((*updates)[0].ID).Should(BeNumerically(">", 0))
				Expect((*updates)[0].RepoID).ToNot(BeNil())
				Expect((*updates)[0].OrgID).Should(Equal(common.DefaultOrgID))
				Expect((*updates)[0].Status).Should(Equal(models.UpdateStatusCreated))
				Expect((*updates)[0].Repo.ID).Should(BeNumerically(">", 0))
				Expect((*updates)[0].Repo.URL).Should(BeEmpty())
				Expect((*updates)[0].Repo.Status).Should(Equal(models.RepoStatusBuilding))

				Expect(len((*updates)[0].Devices)).Should(Equal(1))
				Expect((*updates)[0].Devices[0].UUID).Should(Equal(device.UUID))
				Expect((*updates)[0].Devices[0].RHCClientID).Should(Equal(device.RHCClientID))
			})
		})

		Context("when device haven't rhc_client_id", func() {
			It("should create an update transaction with status disconnected without a repo", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}

				responseInventory := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, common.DefaultOrgID, &newCommit)

				Expect(err).To(BeNil())
				Expect(len(*updates)).Should(Equal(1))
				Expect((*updates)[0].ID).Should(BeNumerically(">", 0))
				Expect((*updates)[0].RepoID).Should(BeNil())
				Expect((*updates)[0].OrgID).Should(Equal(common.DefaultOrgID))
				Expect((*updates)[0].Status).Should(Equal(models.UpdateStatusDeviceDisconnected))
				Expect((*updates)[0].Repo).Should(BeNil())

				Expect(len((*updates)[0].Devices)).Should(Equal(1))
			})
		})

		Context("when has two devices, one with rhc_client_id and another without", func() {
			It("should create two update transactions, one with a repo and another without", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID, device2.UUID}

				responseInventory := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: rhcClientId,
					}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)

				responseInventory2 := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device2.UUID, Ostree: inventory.SystemProfile{}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device2.UUID).
					Return(responseInventory2, nil)

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, common.DefaultOrgID, &newCommit)

				Expect(err).To(BeNil())
				Expect(len(*updates)).Should(Equal(2))
				Expect((*updates)[0].ID).Should(BeNumerically(">", 0))
				Expect((*updates)[0].RepoID).ToNot(BeNil())
				Expect((*updates)[0].OrgID).Should(Equal(common.DefaultOrgID))
				Expect((*updates)[0].Status).Should(Equal(models.UpdateStatusCreated))
				Expect((*updates)[0].Repo.ID).Should(BeNumerically(">", 0))
				Expect((*updates)[0].Repo.URL).Should(BeEmpty())
				Expect((*updates)[0].Repo.Status).Should(Equal(models.RepoStatusBuilding))

				Expect(len((*updates)[0].Devices)).Should(Equal(1))
				Expect((*updates)[0].Devices[0].UUID).Should(Equal(device.UUID))
				Expect((*updates)[0].Devices[0].RHCClientID).Should(Equal(device.RHCClientID))

				Expect((*updates)[1].ID).Should(BeNumerically(">", 0))
				Expect((*updates)[1].RepoID).Should(BeNil())
				Expect((*updates)[1].OrgID).Should(Equal(common.DefaultOrgID))
				Expect((*updates)[1].Status).Should(Equal(models.UpdateStatusDeviceDisconnected))
				Expect((*updates)[1].Repo).Should(BeNil())
			})
		})

		Context("when device doesn't exist on inventory", func() {
			It("should not create update transaction", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}

				responseInventory := inventory.Response{Total: 0, Count: 0, Result: []inventory.Device{}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, common.DefaultOrgID, &newCommit)

				Expect(err).To(BeNil())
				Expect(len(*updates)).Should(Equal(0))
			})
		})

		Context("when inventory return error", func() {
			It("should return device doesn't exist", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}

				responseInventory := inventory.Response{Total: 0, Count: 0, Result: []inventory.Device{}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, errors.New(""))

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, common.DefaultOrgID, &newCommit)

				Expect(err.(apiError.APIError).GetStatus()).To(Equal(404))
				Expect(updates).Should(BeNil())
			})
		})
	})
})
