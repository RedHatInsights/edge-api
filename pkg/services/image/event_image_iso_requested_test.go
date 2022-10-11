// FIXME: golangci-lint
// nolint:revive
package image_test

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
	eventImageReq "github.com/redhatinsights/edge-api/pkg/services/image"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Event Image ISO Event Test", func() {
	var ctx context.Context
	var mockImageService *mock_services.MockImageServiceInterface
	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)

		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
		})
		ctx = eventImageReq.ContextWithLogger(ctx, log.NewEntry(log.StandardLogger()))
	})
	Describe("consume image iso event", func() {
		When("image iso is requested", func() {
			Context("when image does not exist", func() {
				It("should be ok", func() {
					orgID := faker.UUIDHyphenated()

					image := &models.Image{
						OrgID:        orgID,
						Commit:       &models.Commit{},
						Distribution: "rhel-85",
						OutputTypes:  []string{models.ImageTypeInstaller},
						Version:      1,
						Name:         faker.Name(),
					}

					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeImageISORequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      image.RequestID,
						},
						NewImage: *image,
					}
					Expect(edgePayload).ToNot(BeNil())

					mockImageService.EXPECT().AddUserInfo(gomock.Any()).Return(nil)
					event := &eventImageReq.EventImageISORequestedBuildHandler{}
					event.Data = *edgePayload
					event.Consume(ctx)

				})
			})
			Context("when image passed is nil", func() {
				It("should not be ok", func() {
					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeImageISORequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      faker.UUIDHyphenated(),
						},
					}
					Expect(edgePayload).ToNot(BeNil())

					event := &eventImageReq.EventImageISORequestedBuildHandler{}
					event.Data = *edgePayload
					event.Consume(ctx)
				})
			})
			Context("when add image info fails", func() {
				It("should be ok", func() {
					orgID := faker.UUIDHyphenated()

					image := &models.Image{
						OrgID:        orgID,
						Commit:       &models.Commit{},
						Distribution: "rhel-85",
						OutputTypes:  []string{models.ImageTypeInstaller},
						Version:      1,
						Name:         faker.Name(),
					}

					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeImageISORequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      image.RequestID,
						},
						NewImage: *image,
					}
					Expect(edgePayload).ToNot(BeNil())

					mockImageService.EXPECT().AddUserInfo(gomock.Any()).Return(err)
					mockImageService.EXPECT().SetErrorStatusOnImage(gomock.Any(), gomock.Any())
					event := &eventImageReq.EventImageISORequestedBuildHandler{}
					event.Data = *edgePayload
					event.Consume(ctx)

				})
			})
		})
	})
})
