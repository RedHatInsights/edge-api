package services_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/pkg/clients/inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/inventory/mock_inventory"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher/mock_playbookdispatcher"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	apiError "github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"

	"github.com/redhatinsights/edge-api/config"
)

var _ = Describe("UpdateService Basic functions", func() {
	f, _ := os.Getwd()
	templatesPath := fmt.Sprintf("%s/../templates/", filepath.Dir(f))
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
			orgID := faker.UUIDHyphenated()

			device := models.Device{
				UUID:  faker.UUIDHyphenated(),
				OrgID: orgID,
			}
			db.DB.Create(&device)
			device2 := models.Device{
				UUID:  faker.UUIDHyphenated(),
				OrgID: orgID,
			}
			db.DB.Create(&device2)
			updates := []models.UpdateTransaction{
				{
					Devices: []models.Device{
						device,
					},
					OrgID: orgID,
				},
				{
					Devices: []models.Device{
						device,
					},
					OrgID: orgID,
				},
				{
					Devices: []models.Device{
						device2,
					},
					OrgID: orgID,
				},
			}
			db.DB.Omit("Devices.*").Create(&updates[0])
			db.DB.Omit("Devices.*").Create(&updates[1])
			db.DB.Omit("Devices.*").Create(&updates[2])

			It("to return two updates for first device", func() {
				actual, err := updateService.GetUpdateTransactionsForDevice(&device)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual).ToNot(BeNil())
				Expect(*actual).To(HaveLen(2))
			})
			It("to return one update for second device", func() {
				actual, err := updateService.GetUpdateTransactionsForDevice(&device2)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual).ToNot(BeNil())
				Expect(*actual).To(HaveLen(1))
			})
		})
	})

	Context("SetUpdateErrorStatusWhenInterrupted", func() {
		var updateService *services.UpdateService
		var ctrl *gomock.Controller
		var testLog *log.Entry
		var logBuffer bytes.Buffer
		var updateTransaction models.UpdateTransaction
		var originalLogLevel log.Level
		orgID := common.DefaultOrgID

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())

			updateService = &services.UpdateService{
				Service: services.NewService(context.Background(), log.WithField("service", "update")),
			}
			// configure the logger
			originalLogLevel = log.GetLevel()
			testLog = log.NewEntry(log.StandardLogger())
			log.SetLevel(log.DebugLevel)
			// Set the output to use our new local logBuffer
			logBuffer = bytes.Buffer{}
			testLog.Logger.SetOutput(&logBuffer)

			updateTransaction = models.UpdateTransaction{OrgID: orgID, Status: models.UpdateStatusBuilding}
			result := db.DB.Create(&updateTransaction)
			Expect(result.Error).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
			log.SetLevel(originalLogLevel)
		})

		It("update transaction status set to error when interrupted", func() {
			intctx, intcancel := context.WithCancel(context.Background())
			sigint := make(chan os.Signal, 1)
			sigint <- os.Interrupt
			updateService.SetUpdateErrorStatusWhenInterrupted(intctx, updateTransaction, sigint, intcancel)
			Expect(logBuffer.String()).To(ContainSubstring("Select case SIGINT interrupt has been triggered"))
			Expect(logBuffer.String()).To(ContainSubstring("Update updated with Error status"))
			result := db.DB.First(&updateTransaction, updateTransaction.ID)
			Expect(result.Error).ToNot(HaveOccurred())
			Expect(updateTransaction.Status).To(Equal(models.UpdateStatusError))
		})

		It("log error when update transaction not found", func() {
			updateTransaction = models.UpdateTransaction{OrgID: common.DefaultOrgID, Status: models.UpdateStatusBuilding}
			intctx, intcancel := context.WithCancel(context.Background())
			sigint := make(chan os.Signal, 1)
			sigint <- os.Interrupt
			updateService.SetUpdateErrorStatusWhenInterrupted(intctx, updateTransaction, sigint, intcancel)
			Expect(logBuffer.String()).To(ContainSubstring("Select case SIGINT interrupt has been triggered"))
			Expect(logBuffer.String()).To(ContainSubstring("Error retrieving update"))
		})

		It("log error when update transaction not found", func() {
			updateTransaction = models.UpdateTransaction{OrgID: common.DefaultOrgID, Status: models.UpdateStatusBuilding}
			intctx, intcancel := context.WithCancel(context.Background())
			sigint := make(chan os.Signal, 1)
			sigint <- os.Interrupt
			updateService.SetUpdateErrorStatusWhenInterrupted(intctx, updateTransaction, sigint, intcancel)
			Expect(logBuffer.String()).To(ContainSubstring("Select case SIGINT interrupt has been triggered"))
			Expect(logBuffer.String()).To(ContainSubstring("Error retrieving update"))
		})

		It("when intcancel called exit SetUpdateErrorStatusWhenInterrupted", func() {
			intctx, intcancel := context.WithCancel(context.Background())
			sigint := make(chan os.Signal, 1)
			intcancel()
			updateService.SetUpdateErrorStatusWhenInterrupted(intctx, updateTransaction, sigint, intcancel)
			Expect(logBuffer.String()).To(ContainSubstring("Select case context intCtx done has been triggered"))
			Expect(logBuffer.String()).To(ContainSubstring("exiting SetUpdateErrorStatusWhenInterrupted"))
		})
	})

	Describe("update creation", func() {
		var updateService services.UpdateServiceInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var mockFilesService *mock_services.MockFilesService
		var mockPlaybookClient *mock_playbookdispatcher.MockClientInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var mockProducer *mock_kafkacommon.MockProducer
		var mockTopicService *mock_kafkacommon.MockTopicServiceInterface
		var update models.UpdateTransaction
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockFilesService = mock_services.NewMockFilesService(ctrl)
			mockPlaybookClient = mock_playbookdispatcher.NewMockClientInterface(ctrl)
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
			mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)
			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				FilesService:    mockFilesService,
				RepoBuilder:     mockRepoBuilder,
				PlaybookClient:  mockPlaybookClient,
				ProducerService: mockProducerService,
				TopicService:    mockTopicService,
				WaitForReboot:   0,
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("send notification", func() {
			uuid := faker.UUIDHyphenated()
			orgID := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: orgID,
			}
			db.DB.Create(&device)
			update = models.UpdateTransaction{
				Devices: []models.Device{
					device,
				},
				OrgID:  orgID,
				Status: models.UpdateStatusBuilding,
			}
			db.DB.Create(&update)
			It("should send the notification", func() {
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)
				notify, err := updateService.SendDeviceNotification(&update)
				Expect(err).ToNot(HaveOccurred())
				Expect(notify.Version).To(Equal("v1.1.0"))
				Expect(notify.EventType).To(Equal("update-devices"))
			})
			It("should send return an error", func() {
				err := errors.New("error producing message")
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(err)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)
				_, err2 := updateService.SendDeviceNotification(&update)
				Expect(err2).To(HaveOccurred())
				Expect(err).To(Equal(err2))
			})
			It("should return error when producer is undefined", func() {
				expectedError := new(services.KafkaProducerInstanceUndefined)
				mockProducerService.EXPECT().GetProducerInstance().Return(nil)
				_, err := updateService.SendDeviceNotification(&update)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})
			It("should return error when GetTopic fail", func() {
				expectedError := errors.New("topic-service GetTopic expected error")
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return("", expectedError)
				// produce function should not be called
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Times(0)
				_, err := updateService.SendDeviceNotification(&update)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})

		Context("#CreateUpdate", func() {
			var uuid string
			var device models.Device
			var update models.UpdateTransaction
			var cfg *config.EdgeConfig
			BeforeEach(func() {
				cfg = config.Get()
				cfg.TemplatesPath = fmt.Sprintf("%v", templatesPath)
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

					db.DB.Preload("DispatchRecords").Preload("DispatchRecords.Device").Preload("Devices").First(&updateTransaction, update.ID)

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
			When("EDA feature UpdateRepoRequested is Enabled", func() {
				var updateService services.UpdateServiceInterface
				var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
				var ctrl *gomock.Controller
				conf := config.Get()

				BeforeEach(func() {
					ctrl = gomock.NewController(GinkgoT())
					mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
					updateService = &services.UpdateService{
						Service:         services.NewService(context.Background(), log.WithField("service", "update")),
						ProducerService: mockProducerService,
					}
					// enable feature by environment
					err := os.Setenv("FEATURE_UPDATE_REPO_REQUESTED", "True")
					Expect(err).ToNot(HaveOccurred())
				})

				AfterEach(func() {
					ctrl.Finish()
					// disable feature by clearing the environment
					err := os.Unsetenv("FEATURE_UPDATE_REPO_REQUESTED")
					Expect(err).ToNot(HaveOccurred())
				})

				It("should create kafka event", func() {
					mockProducerService.EXPECT().ProduceEvent(
						kafkacommon.TopicFleetmgmtUpdateRepoRequested, models.EventTypeEdgeUpdateRepoRequested, gomock.Any(),
					).Return(nil)
					_, err := updateService.CreateUpdate(update.ID)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should return error create kafka event failed", func() {
					expectedErr := errors.New("producer event error")
					mockProducerService.EXPECT().ProduceEvent(
						kafkacommon.TopicFleetmgmtUpdateRepoRequested, models.EventTypeEdgeUpdateRepoRequested, gomock.Any(),
					).Return(expectedErr)
					_, err := updateService.CreateUpdate(update.ID)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(Equal(expectedErr.Error()))
				})

				When("missing identity in context", func() {
					var originalAuth bool
					BeforeEach(func() {
						originalAuth = conf.Auth
						conf.Auth = true
					})

					AfterEach(func() {
						conf.Auth = originalAuth
					})

					It("should not ProduceEvent", func() {
						// Produce event should not be called
						mockProducerService.EXPECT().ProduceEvent(
							kafkacommon.TopicFleetmgmtUpdateRepoRequested, models.EventTypeEdgeUpdateRepoRequested, gomock.Any(),
						).Times(0)
						_, err := updateService.CreateUpdate(update.ID)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal("cannot find org-id"))
					})
				})
			})
		})
	})
	Describe("playbook dispatcher event handling", func() {

		var updateService services.UpdateServiceInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				ProducerService: mockProducerService,
			}

		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("when record is found and status is success", func() {
			uuid := faker.UUIDHyphenated()
			orgID := faker.UUIDHyphenated()
			availableHash := faker.UUIDHyphenated()
			currentHash := faker.UUIDHyphenated()
			image := models.Image{OrgID: orgID, Name: faker.Name(), Commit: &models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}}
			db.DB.Create(&image)
			device := models.Device{
				UUID:          uuid,
				OrgID:         orgID,
				AvailableHash: availableHash,
				CurrentHash:   currentHash,
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
				OrgID:           orgID,
				Commit:          image.Commit,
			}
			db.DB.Omit("Devices.*").Create(u)

			event := &services.PlaybookDispatcherEvent{
				Payload: services.PlaybookDispatcherEventPayload{
					ID:     d.PlaybookDispatcherID,
					Status: services.PlaybookStatusSuccess,
					OrgID:  orgID,
				},
			}
			message, _ := json.Marshal(event)

			It("should update status when record is found", func() {
				err := updateService.ProcessPlaybookDispatcherRunEvent(message)
				Expect(err).ToNot(HaveOccurred())
				db.DB.Preload("Device").First(&d, d.ID)
				Expect(d.Status).To(Equal(models.DispatchRecordStatusComplete))
				Expect(d.Device.AvailableHash).To(Equal(os.DevNull))
				Expect(d.Device.CurrentHash).To(Equal(availableHash))
			})
			It("should update status of the dispatch record", func() {
				err := updateService.ProcessPlaybookDispatcherRunEvent(message)
				Expect(err).ToNot(HaveOccurred())
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
			orgID := faker.UUIDHyphenated()
			device := models.Device{
				UUID:  uuid,
				OrgID: orgID,
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
		orgID := faker.UUIDHyphenated()

		Context("when upload works", func() {
			It("to build the template for update properly", func() {
				t := services.TemplateRemoteInfo{
					UpdateTransactionID: 1000,
					RemoteName:          "remote-name",
					RemoteOstreeUpdate:  "false",
					OSTreeRef:           "rhel/8/x86_64/edge",
					GpgVerify:           "false",
				}
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", orgID, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", orgID, fname)

				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				mockProducerService := mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
				updateService := &services.UpdateService{
					Service:         services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService:    mockFilesService,
					ProducerService: mockProducerService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", orgID, fname)).Do(func(x, y string) {
					actual, err := os.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := os.ReadFile(fmt.Sprintf("%s/%s", templatesPath, "template_playbook_dispatcher_ostree_upgrade_payload.test.yml"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, orgID)

				Expect(err).ToNot(HaveOccurred())
				Expect(url).ToNot(BeNil())
				Expect(url).To(BeEquivalentTo("http://localhost:3000/api/edge/v1/updates/1000/update-playbook.yml"))
			})
		})

		Context("when upload works", func() {
			BeforeEach(func() {
				os.Setenv("ENABLE_GPG_VERIFY", "True")
			})
			AfterEach(func() {
				os.Unsetenv("ENABLE_GPG_VERIFY")
			})
			It("to build the template for PROD rebase properly", func() {
				t := services.TemplateRemoteInfo{
					UpdateTransactionID: 1000,
					RemoteName:          "remote-name",
					RemoteOstreeUpdate:  "true",
					OSTreeRef:           "rhel/9/x86_64/edge",
					GpgVerify:           "true",
				}
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", orgID, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", orgID, fname)
				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				mockProducerService := mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
				updateService := &services.UpdateService{
					Service:         services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService:    mockFilesService,
					ProducerService: mockProducerService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", orgID, fname)).Do(func(x, y string) {
					actual, err := os.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := os.ReadFile(fmt.Sprintf("%s/%s", templatesPath, "template_playbook_dispatcher_ostree_rebase_payload_prod.test.yml"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, orgID)

				Expect(err).ToNot(HaveOccurred())
				Expect(url).ToNot(BeNil())
				Expect(url).To(BeEquivalentTo("http://localhost:3000/api/edge/v1/updates/1000/update-playbook.yml"))
			})

			It("to build the template for rebase properly", func() {
				t := services.TemplateRemoteInfo{
					UpdateTransactionID: 1000,
					RemoteName:          "remote-name",
					RemoteOstreeUpdate:  "true",
					OSTreeRef:           "rhel/9/x86_64/edge",
					GpgVerify:           "false",
				}
				fname := fmt.Sprintf("playbook_dispatcher_update_%s_%d.yml", orgID, t.UpdateTransactionID)
				tmpfilepath := fmt.Sprintf("/tmp/v2/%s/%s", orgID, fname)
				ctrl := gomock.NewController(GinkgoT())
				defer ctrl.Finish()
				mockFilesService := mock_services.NewMockFilesService(ctrl)
				mockProducerService := mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
				updateService := &services.UpdateService{
					Service:         services.NewService(context.Background(), log.WithField("service", "update")),
					FilesService:    mockFilesService,
					ProducerService: mockProducerService,
				}
				mockUploader := mock_services.NewMockUploader(ctrl)
				mockUploader.EXPECT().UploadFile(tmpfilepath, fmt.Sprintf("%s/playbooks/%s", orgID, fname)).Do(func(x, y string) {
					actual, err := os.ReadFile(x)
					Expect(err).ToNot(HaveOccurred())
					expected, err := os.ReadFile(fmt.Sprintf("%s/%s", templatesPath, "template_playbook_dispatcher_ostree_rebase_payload.test.yml"))
					Expect(err).ToNot(HaveOccurred())
					Expect(string(actual)).To(BeEquivalentTo(string(expected)))
				}).Return("url", nil)
				mockFilesService.EXPECT().GetUploader().Return(mockUploader)

				url, err := updateService.WriteTemplate(t, orgID)

				Expect(err).ToNot(HaveOccurred())
				Expect(url).ToNot(BeNil())
				Expect(url).To(BeEquivalentTo("http://localhost:3000/api/edge/v1/updates/1000/update-playbook.yml"))
			})
		})
	})

	Describe("Set status on update", func() {

		var updateService services.UpdateServiceInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var ctrl *gomock.Controller

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				ProducerService: mockProducerService,
			}

		})

		AfterEach(func() {
			ctrl.Finish()
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
				err := updateService.SetUpdateStatus(u)
				Expect(err).ToNot(HaveOccurred())
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
				err := updateService.SetUpdateStatus(u)
				Expect(err).ToNot(HaveOccurred())
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
				err := updateService.SetUpdateStatus(u)
				Expect(err).ToNot(HaveOccurred())
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
		var orgID string
		var imageSet models.ImageSet
		BeforeEach(func() {

			orgID = faker.UUIDHyphenated()
			imageSet = models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
			db.DB.Create(&imageSet)
			currentCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&currentCommit)
			currentImage = models.Image{OrgID: orgID, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
			db.DB.Create(&currentImage)

			newCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&newCommit)
			newImage = models.Image{OrgID: orgID, CommitID: newCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
			db.DB.Create(&newImage)

			device = models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
			db.DB.Create(&device)
			update = models.UpdateTransaction{

				OrgID:    orgID,
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
				commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
				result := db.DB.Create(&commit)
				Expect(result.Error).To(BeNil())
				image := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&image)
				Expect(result.Error).To(BeNil())

				// create a new image,  without commit as we do not need it for the current function
				lastImage := models.Image{OrgID: orgID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				result = db.DB.Create(&lastImage)
				Expect(result.Error).To(BeNil())

				// create a new update with commit and image, knowing that we have a new image
				update := models.UpdateTransaction{

					OrgID:    orgID,
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

		orgID := faker.UUIDHyphenated()
		var updateService services.UpdateServiceInterface
		var ctrl *gomock.Controller
		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var mockProducer *mock_kafkacommon.MockProducer
		var mockTopicService *mock_kafkacommon.MockTopicServiceInterface

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
			mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)

			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:     mockRepoBuilder,
				Inventory:       mockInventoryClient,
				ImageService:    mockImageService,
				ProducerService: mockProducerService,
				TopicService:    mockTopicService,
				WaitForReboot:   0,
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: orgID, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-86"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: true}
		db.DB.Create(&commit)
		image := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		latestCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&latestCommit)
		latestImage := models.Image{OrgID: orgID, CommitID: latestCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&latestImage)

		device := models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{

			OrgID:    orgID,
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
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(latestCommit.OSTreeCommit).Return(&latestImage, nil)

				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, orgID, &latestCommit)
				Expect(err).To(BeNil())
				Expect(upd).ToNot(BeNil())
				Expect(len(*upd) > 0).To(BeTrue())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeTrue())
				}
			})
		})
	})

	Describe("Update Devices to same distribution", func() {
		orgID := faker.UUIDHyphenated()
		var ctrl *gomock.Controller
		var updateService services.UpdateServiceInterface
		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var mockProducer *mock_kafkacommon.MockProducer
		var mockTopicService *mock_kafkacommon.MockTopicServiceInterface

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
			mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)

			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:     mockRepoBuilder,
				Inventory:       mockInventoryClient,
				ImageService:    mockImageService,
				ProducerService: mockProducerService,
				TopicService:    mockTopicService,
				WaitForReboot:   0,
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: orgID, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&commit)
		image := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		latestCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&latestCommit)
		latestImage := models.Image{OrgID: orgID, CommitID: latestCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&latestImage)

		device := models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{

			OrgID:    orgID,
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

		Context("when update do not change Refs success", func() {

			It("initialisation should pass", func() {
				resp := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: faker.UUIDHyphenated(),
						RpmOstreeDeployments: []inventory.OSTree{
							{Checksum: currentCommit.OSTreeCommit, Booted: true},
						},
					},
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(latestCommit.OSTreeCommit).Return(&latestImage, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, orgID, &latestCommit)
				Expect(err).To(BeNil())
				Expect(upd).ToNot(BeNil())
				Expect(len(*upd) > 0).To(BeTrue())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeFalse())
				}
			})
		})
	})

	Describe("Update Devices from 1 to 2", func() {
		orgID := faker.UUIDHyphenated()
		var updateService services.UpdateServiceInterface
		var ctrl *gomock.Controller
		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var mockTopicService *mock_kafkacommon.MockTopicServiceInterface
		var mockProducer *mock_kafkacommon.MockProducer

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
			mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)

			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:     mockRepoBuilder,
				Inventory:       mockInventoryClient,
				ImageService:    mockImageService,
				ProducerService: mockProducerService,
				TopicService:    mockTopicService,
				WaitForReboot:   0,
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: orgID, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-86"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&commit)
		image := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		device := models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{
			OrgID:    orgID,
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
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(commit.OSTreeCommit).Return(&image, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, orgID, &commit)
				Expect(err).To(BeNil())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeTrue())
				}
			})
		})
	})

	Describe("Update Devices from 1 to 2 but the latest is version 3", func() {
		orgID := faker.UUIDHyphenated()
		var updateService services.UpdateServiceInterface
		var ctrl *gomock.Controller
		var mockImageService *mock_services.MockImageServiceInterface
		var mockInventoryClient *mock_inventory.MockClientInterface
		var mockRepoBuilder *mock_services.MockRepoBuilderInterface
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var mockProducer *mock_kafkacommon.MockProducer
		var mockTopicService *mock_kafkacommon.MockTopicServiceInterface

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
			mockInventoryClient = mock_inventory.NewMockClientInterface(ctrl)
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
			mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)

			updateService = &services.UpdateService{
				Service:         services.NewService(context.Background(), log.WithField("service", "update")),
				RepoBuilder:     mockRepoBuilder,
				Inventory:       mockInventoryClient,
				ImageService:    mockImageService,
				ProducerService: mockProducerService,
				TopicService:    mockTopicService,
				WaitForReboot:   0,
			}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
		db.DB.Create(&imageSet)

		currentCommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
		db.DB.Create(&currentCommit)
		currentImage := models.Image{OrgID: orgID, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&currentImage)

		commit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&commit)
		image := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&image)

		latestcommit := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), ChangesRefs: false}
		db.DB.Create(&latestcommit)
		latestimage := models.Image{OrgID: orgID, CommitID: commit.ID, ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess, Distribution: "rhel-90"}
		db.DB.Create(&latestimage)

		device := models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
		db.DB.Create(&device)

		repo := models.Repo{Status: models.RepoStatusSuccess, URL: "www.redhat.com"}
		db.DB.Create(&repo)

		update := models.UpdateTransaction{
			OrgID:    orgID,
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
						OrgID: orgID,
					},
				}}
				mockInventoryClient.EXPECT().ReturnDevicesByID(device.UUID).Return(resp, nil)

				mockImageService.EXPECT().GetImageByOSTreeCommitHash(currentCommit.OSTreeCommit).Return(&currentImage, nil)
				mockImageService.EXPECT().GetImageByOSTreeCommitHash(commit.OSTreeCommit).Return(&image, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

				upd, err := updateService.BuildUpdateTransactions(&devicesUpdate, orgID, &commit)
				Expect(err).To(BeNil())
				for _, u := range *upd {
					Expect(u.ChangesRefs).To(BeFalse())
				}
			})
		})
	})

	Describe("Create Update Transaction", func() {
		orgID := common.DefaultOrgID
		rhcClientID := faker.UUIDHyphenated()
		dist := "rhel-85"
		updateDist := "rhel-86"
		var ctrl *gomock.Controller
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
		var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
		var mockProducer *mock_kafkacommon.MockProducer
		var mockTopicService *mock_kafkacommon.MockTopicServiceInterface

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
			mockInventory = mock_inventory.NewMockClientInterface(ctrl)
			mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
			mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
			mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)

			ctx := context.Background()
			updateService = &services.UpdateService{
				Service:         services.NewService(ctx, log.WithField("service", "update")),
				RepoBuilder:     mockRepoBuilder,
				Inventory:       mockInventory,
				ImageService:    services.NewImageService(ctx, log.WithField("service", "image")),
				ProducerService: mockProducerService,
				TopicService:    mockTopicService,
				WaitForReboot:   0,
			}

			imageSet = models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
			db.DB.Create(&imageSet)
			currentCommit = models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&currentCommit)
			currentImage = models.Image{OrgID: orgID, CommitID: currentCommit.ID, ImageSetID: &imageSet.ID, Distribution: dist, Status: models.ImageStatusSuccess}
			db.DB.Create(&currentImage)
			newCommit = models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
			db.DB.Create(&newCommit)
			newImage = models.Image{OrgID: orgID, CommitID: newCommit.ID, ImageSetID: &imageSet.ID, Distribution: updateDist, Status: models.ImageStatusSuccess}
			db.DB.Create(&newImage)
			device = models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated(), RHCClientID: rhcClientID}
			db.DB.Create(&device)
			device2 = models.Device{OrgID: orgID, ImageID: currentImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated()}
			db.DB.Create(&device2)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("when device has rhc_client_id", func() {
			It("should create an update transaction with a repo", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}

				responseInventory := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: rhcClientID,
					}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

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

			It("when current image dist and update image dist has same refs ChangesRefs should be false", func() {
				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}
				responseInventory := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID: rhcClientID,
					}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, orgID, &newCommit)
				Expect(err).To(BeNil())
				Expect(len(*updates)).Should(Equal(1))
				Expect((*updates)[0].ChangesRefs).To(BeFalse())
			})

			It("when current image dist and update image dist has different refs ChangesRefs should be true", func() {
				updateDist = "rhel-90"
				rhcClientID := faker.UUIDHyphenated()
				newCommit2 := models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated()}
				db.DB.Create(&newCommit2)
				newImage2 := models.Image{OrgID: orgID, CommitID: newCommit2.ID, ImageSetID: &imageSet.ID, Distribution: updateDist, Status: models.ImageStatusSuccess}
				db.DB.Create(&newImage2)
				device = models.Device{OrgID: orgID, ImageID: newImage.ID, UpdateAvailable: true, UUID: faker.UUIDHyphenated(), RHCClientID: rhcClientID}
				db.DB.Create(&device)

				var devicesUpdate models.DevicesUpdate
				devicesUpdate.DevicesUUID = []string{device.UUID}
				responseInventory := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device.UUID, Ostree: inventory.SystemProfile{
						RHCClientID:          rhcClientID,
						RpmOstreeDeployments: []inventory.OSTree{{Booted: true, Checksum: newCommit.OSTreeCommit}},
					}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

				updates, err := updateService.BuildUpdateTransactions(&devicesUpdate, orgID, &newCommit2)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(*updates)).Should(Equal(1))
				Expect((*updates)[0].ChangesRefs).To(BeTrue())
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
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)

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
						RHCClientID: rhcClientID,
					}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device.UUID).
					Return(responseInventory, nil)

				responseInventory2 := inventory.Response{Total: 1, Count: 1, Result: []inventory.Device{
					{ID: device2.UUID, Ostree: inventory.SystemProfile{}},
				}}
				mockInventory.EXPECT().ReturnDevicesByID(device2.UUID).
					Return(responseInventory2, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Return(nil)
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil).Times(2)

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

	Context("Test build remote info feature flag disable", func() {
		var update *models.UpdateTransaction

		BeforeEach(func() {
			os.Unsetenv("ENABLE_GPG_VERIFY")
			orgID := faker.UUIDHyphenated()
			update = &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{},
				OrgID:           orgID,
				Commit:          &models.Commit{OSTreeRef: "ref"},
				Repo:            &models.Repo{URL: "http://rh.com"},
			}
		})

		It("should return template with gpg false", func() {
			remoteInfo := services.NewTemplateRemoteInfo(update)
			Expect(remoteInfo.GpgVerify).To(Equal("false"))
		})
	})

	Context("Test build remote info with feature flag enable", func() {
		var update *models.UpdateTransaction
		BeforeEach(func() {
			os.Setenv("ENABLE_GPG_VERIFY", "True")
			orgID := faker.UUIDHyphenated()
			update = &models.UpdateTransaction{
				DispatchRecords: []models.DispatchRecord{},
				OrgID:           orgID,
				Commit:          &models.Commit{OSTreeRef: "ref"},
				Repo:            &models.Repo{URL: "http://rh.com"},
			}

		})
		AfterEach(func() {
			os.Unsetenv("ENABLE_GPG_VERIFY")
		})

		It("should return template with gpg true", func() {
			remoteInfo := services.NewTemplateRemoteInfo(update)
			Expect(remoteInfo.GpgVerify).To(Equal("true"))

		})
	})
})
