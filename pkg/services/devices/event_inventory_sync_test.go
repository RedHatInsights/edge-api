// nolint:revive,typecheck
package devices_test

import (
	"bytes"
	"context"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	eventReq "github.com/redhatinsights/edge-api/pkg/services/devices"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Event Inventory sync Event Test", func() {
	var ctx context.Context
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var ctrl *gomock.Controller
	var logBuffer bytes.Buffer
	var testLog *log.Entry
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		testLog = log.NewEntry(log.StandardLogger())
		// Set the output to use our new local logBuffer
		logBuffer = bytes.Buffer{}
		testLog.Logger.SetOutput(&logBuffer)
		mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)
		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			DeviceService: mockDeviceService,
		})
		ctx = utility.ContextWithLogger(ctx, testLog)
	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("consume inventory sync event", func() {
		When("inventory sync is requested", func() {
			Context("inventory sync is requested", func() {
				It("should be ok", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())
					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					}
					event := &eventReq.EventInventorySyncHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					mockDeviceService.EXPECT().SyncInventoryWithDevices(event.RedHatOrgID).Return()
					event.Consume(ctx)
				})
			})
			Context("inventory sync is not requested when orgID is empty", func() {
				It("SyncDevicesWithInventory should not be called when orgID is empty", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					}
					Expect(edgePayload).ToNot(BeNil())

					event := &eventReq.EventInventorySyncHandler{}
					event.RedHatOrgID = ""
					event.Data = *edgePayload
					// SyncInventoryWithDevices should not be called
					mockDeviceService.EXPECT().SyncInventoryWithDevices(event.RedHatOrgID).Times(0)
					event.Consume(ctx)
					Expect(logBuffer.String()).To(ContainSubstring("Malformed device sync request, required data missing"))
				})
			})
			Context("inventory sync is not requested when RequestID is empty", func() {
				It("SyncDevicesWithInventory should not be called when RequestID is empty", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      "",
					}
					Expect(edgePayload).ToNot(BeNil())

					event := &eventReq.EventInventorySyncHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					// SyncInventoryWithDevices should not be called
					mockDeviceService.EXPECT().SyncInventoryWithDevices(event.RedHatOrgID).Times(0)
					event.Consume(ctx)
					Expect(logBuffer.String()).To(ContainSubstring("Malformed device sync request, required data missing"))
				})
			})
			Context("inventory sync is not requested when OrgID is different from identity one", func() {
				It("SyncDevicesWithInventory should not be called when OrgID is different from identity one", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					}
					Expect(edgePayload).ToNot(BeNil())

					event := &eventReq.EventInventorySyncHandler{}
					event.RedHatOrgID = faker.UUIDHyphenated()
					event.Data = *edgePayload
					// SyncInventoryWithDevices should not be called
					mockDeviceService.EXPECT().SyncInventoryWithDevices(event.RedHatOrgID).Times(0)
					event.Consume(ctx)
					Expect(logBuffer.String()).To(ContainSubstring("Malformed device sync request, required data missing"))
				})
			})
		})
	})
})
