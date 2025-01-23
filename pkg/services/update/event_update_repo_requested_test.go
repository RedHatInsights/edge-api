// nolint:revive,typecheck
package update_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"time"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	eventReq "github.com/redhatinsights/edge-api/pkg/services/update"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/osbuild/logging/pkg/logrus"
)

var _ = Describe("UpdateRepoRequested Event Consumer Test", func() {
	var ctx context.Context
	var mockUpdateService *mock_services.MockUpdateServiceInterface
	var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
	var ctrl *gomock.Controller
	var logBuffer bytes.Buffer
	var updateTransaction models.UpdateTransaction
	var ident identity.XRHID
	orgID := common.DefaultOrgID
	oldLog := log.Default()

	BeforeEach(func() {
		logBuffer.Reset()
		testLog := log.NewProxyFor(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
		log.SetDefault(testLog)

		ctrl = gomock.NewController(GinkgoT())
		mockUpdateService = mock_services.NewMockUpdateServiceInterface(ctrl)
		mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)

		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			UpdateService:   mockUpdateService,
			ProducerService: mockProducerService,
		})
		ctx = utility.ContextWithLogger(ctx, testLog)
		updateTransaction = models.UpdateTransaction{OrgID: orgID, Status: models.UpdateStatusBuilding}
		res := db.DB.Create(&updateTransaction)
		Expect(res.Error).ToNot(HaveOccurred())
		var err error
		ident, err = common.GetIdentityFromContext(ctx)
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		ctrl.Finish()
		log.SetDefault(oldLog)
	})

	Describe("consume UpdateRepoRequested event", func() {
		When("UpdateRepoRequested is requested", func() {
			Context("ValidateEvent", func() {
				It("No Validation Error occurs when all data matches", func() {
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					event.Data = *edgePayload
					refreshedUpdatedTransaction, err := event.ValidateEvent()
					Expect(err).ToNot(HaveOccurred())
					Expect(refreshedUpdatedTransaction.ID).To(Equal(updateTransaction.ID))
				})

				It("Validation Error when RequestID is empty", func() {
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      "",
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerMissingRequiredData))
				})

				It("Validation Error when OrgID is empty", func() {
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ""
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerMissingRequiredData))
				})

				It("Validation Error when OrgID is different from identity one", func() {
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = faker.UUIDHyphenated()
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerRequiredDataMismatch))
				})

				It("Validation Error when Update id is not defined", func() {
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: models.UpdateTransaction{OrgID: orgID},
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerUpdateIDRequired))
				})

				It("Validation Error when event orgID and update ORG mismatch", func() {
					updateTransaction = models.UpdateTransaction{Model: models.Model{ID: 999}, OrgID: faker.UUIDHyphenated(), Status: models.UpdateStatusBuilding}
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerUpdateOrgIDMismatch))
				})

				It("Validation Error when update does not exist", func() {
					updateTransaction = models.UpdateTransaction{Model: models.Model{ID: 9999999999}, OrgID: orgID, Status: models.UpdateStatusBuilding}
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerUpdateDoesNotExist))
				})

				It("Validation Error when update not in building state", func() {
					updateTransaction = models.UpdateTransaction{OrgID: orgID, Status: models.UpdateStatusError}
					res := db.DB.Create(&updateTransaction)
					Expect(res.Error).ToNot(HaveOccurred())
					edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
						Update: updateTransaction,
					}
					event := &eventReq.EventUpdateRepoRequestedHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					_, err := event.ValidateEvent()
					Expect(err).To(Equal(eventReq.ErrEventHandlerUpdateBadStatus))
				})
			})

			It("BuildUpdateRepo and WriteTemplate event is emitted", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Return(&updateTransaction, nil)
				mockProducerService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Return(nil)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("Finished UpdateRepoRequested consume"))
			})

			It("when update does not exist, BuildUpdateRepo and Produce WriteTemplate event should not be called", func() {
				updateTransaction = models.UpdateTransaction{Model: models.Model{ID: 9999999999}, OrgID: orgID, Status: models.UpdateStatusBuilding}
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				mockProducerService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerUpdateDoesNotExist.Error()))
			})

			It("when update not in building state, BuildUpdateRepo and Produce WriteTemplate event should not be called", func() {
				updateTransaction = models.UpdateTransaction{OrgID: orgID, Status: models.UpdateStatusError}
				res := db.DB.Create(&updateTransaction)
				Expect(res.Error).ToNot(HaveOccurred())
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				mockProducerService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerUpdateBadStatus.Error()))
			})

			It("BuildUpdateRepo passed but WriteTemplate event failed to be emitted", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Return(&updateTransaction, nil)
				expectedError := errors.New("producer error")
				mockProducerService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Return(expectedError)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("producing the WriteTemplate event failed"))
				// ensure update transaction has status ERROR
				res := db.DB.First(&updateTransaction, updateTransaction.ID)
				Expect(res.Error).ToNot(HaveOccurred())
				Expect(updateTransaction.Status).To(Equal(models.UpdateStatusError))
			})

			It("should log error when BuildUpdateRepo return error", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				expectedError := errors.New("BuildUpdateRepo error")
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Return(nil, expectedError)
				// ProduceEvent WriteTemplate should not be called
				mockProducerService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(expectedError.Error()))
			})

			It("BuildUpdateRepo should not be called when RequestID is empty", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      "",
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				// BuildUpdateRepo should not be called
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerMissingRequiredData.Error()))
			})

			It("BuildUpdateRepo should not be called when OrgID is empty", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ""
				event.Data = *edgePayload
				// BuildUpdateRepo should not be called
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerMissingRequiredData.Error()))
			})

			It("BuildUpdateRepo should not be called when OrgID is different from identity one", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = faker.UUIDHyphenated()
				event.Data = *edgePayload
				// BuildUpdateRepo should not be called
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerRequiredDataMismatch.Error()))
			})

			It("BuildUpdateRepo should not be called when Update id is not defined", func() {
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: models.UpdateTransaction{OrgID: orgID},
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				// BuildUpdateRepo should not be called
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerUpdateIDRequired.Error()))
			})

			It("UpdateRepoRequested should not be called when event orgID and update ORG mismatch", func() {
				updateTransaction = models.UpdateTransaction{OrgID: faker.UUIDHyphenated(), Status: models.UpdateStatusBuilding}
				res := db.DB.Create(&updateTransaction)
				Expect(res.Error).ToNot(HaveOccurred())
				edgePayload := &models.EdgeUpdateRepoRequestedEventPayload{
					EdgeBasePayload: models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					},
					Update: updateTransaction,
				}
				event := &eventReq.EventUpdateRepoRequestedHandler{}
				event.RedHatOrgID = ident.Identity.OrgID
				event.Data = *edgePayload
				// BuildUpdateRepo should not be called
				mockUpdateService.EXPECT().BuildUpdateRepo(ctx, event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(eventReq.ErrEventHandlerUpdateOrgIDMismatch.Error()))
			})
		})
	})
})
