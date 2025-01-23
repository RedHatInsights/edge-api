// nolint:revive,typecheck
package devices_test

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	eventReq "github.com/redhatinsights/edge-api/pkg/services/devices"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
)

var _ = Describe("Event Device sync Event Test", func() {
	var ctx context.Context
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	var ctrl *gomock.Controller
	var logBuffer bytes.Buffer
	oldLog := log.Default()

	BeforeEach(func() {
		logBuffer.Reset()
		testLog := log.NewProxyFor(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
		log.SetDefault(testLog)

		ctrl = gomock.NewController(GinkgoT())
		mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)

		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			DeviceService: mockDeviceService,
		})
		ctx = utility.ContextWithLogger(ctx, testLog)
	})

	AfterEach(func() {
		ctrl.Finish()
		log.SetDefault(oldLog)
	})

	Describe("consume device sync event", func() {
		When("device sync is requested", func() {
			Context("DB and inventory mismatch", func() {
				It("should be ok", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())
					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					}
					event := &eventReq.EventDeviceSyncHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					mockDeviceService.EXPECT().SyncDevicesWithInventory(event.RedHatOrgID).Return()
					event.Consume(ctx)
				})

				It("SyncDevicesWithInventory should not be called when RequestID is empty", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())
					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      "",
					}
					event := &eventReq.EventDeviceSyncHandler{}
					event.RedHatOrgID = ident.Identity.OrgID
					event.Data = *edgePayload
					// SyncDevicesWithInventory should not be called
					mockDeviceService.EXPECT().SyncDevicesWithInventory(event.RedHatOrgID).Times(0)
					event.Consume(ctx)
					Expect(logBuffer.String()).To(ContainSubstring("Malformed device sync request, required data missing"))
				})

				It("SyncDevicesWithInventory should not be called when OrgID is empty", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())
					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					}
					event := &eventReq.EventDeviceSyncHandler{}
					event.RedHatOrgID = ""
					event.Data = *edgePayload
					// SyncDevicesWithInventory should not be called
					mockDeviceService.EXPECT().SyncDevicesWithInventory(gomock.Any()).Times(0)
					event.Consume(ctx)
					Expect(logBuffer.String()).To(ContainSubstring("Malformed device sync request, required data missing"))
				})

				It("SyncDevicesWithInventory should not be called when OrgID is different from identity one", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())
					edgePayload := &models.EdgeBasePayload{
						Identity:       ident,
						LastHandleTime: time.Now().Format(time.RFC3339),
						RequestID:      faker.UUIDHyphenated(),
					}
					event := &eventReq.EventDeviceSyncHandler{}
					event.RedHatOrgID = faker.UUIDHyphenated()
					event.Data = *edgePayload
					// SyncDevicesWithInventory should not be called
					mockDeviceService.EXPECT().SyncDevicesWithInventory(gomock.Any()).Times(0)
					event.Consume(ctx)
					Expect(logBuffer.String()).To(ContainSubstring("Malformed device sync request, required data mismatch"))
				})
			})
		})
	})
})
