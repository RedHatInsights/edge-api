package devices_test

import (
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

var _ = Describe("Event Device sync Event Test", func() {
	var ctx context.Context
	var mockDeviceService *mock_services.MockDeviceServiceInterface
	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		mockDeviceService = mock_services.NewMockDeviceServiceInterface(ctrl)

		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			DeviceService: mockDeviceService,
		})
		ctx = utility.ContextWithLogger(ctx, log.NewEntry(log.StandardLogger()))
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
			})
		})
	})
})
