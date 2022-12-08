package update_test

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/edge-api/pkg/services/utility"

	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	eventReq "github.com/redhatinsights/edge-api/pkg/services/update"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("UpdateRepoRequested Event Consumer Test", func() {
	var ctx context.Context
	var mockUpdateService *mock_services.MockUpdateServiceInterface
	var ctrl *gomock.Controller
	var logBuffer bytes.Buffer
	var testLog *log.Entry
	var updateTransaction models.UpdateTransaction
	orgID := common.DefaultOrgID

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockUpdateService = mock_services.NewMockUpdateServiceInterface(ctrl)
		testLog = log.NewEntry(log.StandardLogger())
		// Set the output to use our new local logBuffer
		logBuffer = bytes.Buffer{}
		testLog.Logger.SetOutput(&logBuffer)

		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			UpdateService: mockUpdateService,
		})
		ctx = utility.ContextWithLogger(ctx, testLog)
		updateTransaction = models.UpdateTransaction{OrgID: orgID, Status: models.UpdateStatusBuilding}
		res := db.DB.Create(&updateTransaction)
		Expect(res.Error).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("consume UpdateRepoRequested event", func() {
		When("UpdateRepoRequested is requested", func() {
			It("BuildUpdateRepo and WriteTemplate event is emitted", func() {
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Return(&updateTransaction, nil)
				mockUpdateService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Return(nil)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("Finished UpdateRepoRequested consume"))
			})

			It("when update does not exist, BuildUpdateRepo and Produce WriteTemplate event should not be called", func() {
				updateTransaction = models.UpdateTransaction{Model: models.Model{ID: 9999999999}, OrgID: orgID, Status: models.UpdateStatusBuilding}
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				mockUpdateService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("event UpdateRepoRequested update does not exist"))
			})

			It("when update not in building state, BuildUpdateRepo and Produce WriteTemplate event should not be called", func() {
				updateTransaction = models.UpdateTransaction{OrgID: orgID, Status: models.UpdateStatusError}
				res := db.DB.Create(&updateTransaction)
				Expect(res.Error).ToNot(HaveOccurred())
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				mockUpdateService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("event UpdateRepoRequested update not in building state"))
			})

			It("BuildUpdateRepo passed but WriteTemplate event failed to be emitted", func() {
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Return(&updateTransaction, nil)
				expectedError := errors.New("producer error")
				mockUpdateService.EXPECT().ProduceEvent(
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
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Return(nil, expectedError)
				// ProduceEvent WriteTemplate should not be called
				mockUpdateService.EXPECT().ProduceEvent(
					kafkacommon.TopicFleetmgmtUpdateWriteTemplateRequested, models.EventTypeEdgeWriteTemplateRequested, gomock.Any(),
				).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring(expectedError.Error()))
			})

			It("BuildUpdateRepo should not be called when RequestID is empty", func() {
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("Malformed UpdateRepoRequested request, required data missing"))
			})

			It("BuildUpdateRepo should not be called when OrgID is empty", func() {
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("Malformed UpdateRepoRequested request, required data missing"))
			})

			It("BuildUpdateRepo should not be called when OrgID is different from identity one", func() {
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("Malformed UpdateRepoRequested request, required data mismatch"))
			})

			It("BuildUpdateRepo should not be called when Update id is not defined", func() {
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("update repo requested, update ID is required"))
			})

			It("UpdateRepoRequested should not be called when event orgID and update ORG mismatch", func() {
				updateTransaction = models.UpdateTransaction{OrgID: faker.UUIDHyphenated(), Status: models.UpdateStatusBuilding}
				res := db.DB.Create(&updateTransaction)
				Expect(res.Error).ToNot(HaveOccurred())
				ident, err := common.GetIdentityFromContext(ctx)
				Expect(err).To(BeNil())
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
				mockUpdateService.EXPECT().BuildUpdateRepo(event.RedHatOrgID, updateTransaction.ID).Times(0)
				event.Consume(ctx)
				Expect(logBuffer.String()).To(ContainSubstring("Malformed UpdateRepoRequested request, event orgID and update orgID mismatch"))
			})
		})
	})
})
