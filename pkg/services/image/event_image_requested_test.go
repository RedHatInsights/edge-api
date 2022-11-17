package image_test

import (
	"context"
	"errors"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	eventImageReq "github.com/redhatinsights/edge-api/pkg/services/image"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Event Image Build Requested Test", func() {
	var ctx context.Context
	var mockImageService *mock_services.MockImageServiceInterface
	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)

		ctx = context.Background()
		//		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
		//			ImageService: mockImageService,
		//		})
		ctx = eventImageReq.ContextWithLogger(ctx, log.NewEntry(log.StandardLogger()))
	})
	Describe("consume image build event", func() {
		When("image build is requested", func() {
			Context("image is processed successfully", func() {
				It("should be ok", func() {
					image := &models.Image{
						OrgID:        faker.UUIDHyphenated(),
						Commit:       &models.Commit{},
						Distribution: "rhel-90",
						OutputTypes:  []string{models.ImageTypeInstaller},
						Version:      1,
						Name:         faker.Name(),
					}

					ident, err := common.GetIdentityFromContext(ctx)
					Expect(err).To(BeNil())

					edgePayload := &models.EdgeImageRequestedEventPayload{
						EdgeBasePayload: models.EdgeBasePayload{
							Identity:       ident,
							LastHandleTime: time.Now().Format(time.RFC3339),
							RequestID:      image.RequestID,
						},
						NewImage: *image,
					}
					Expect(edgePayload).ToNot(BeNil())

					mockImageService.EXPECT().ProcessImage(gomock.Any(), gomock.Any()).Return(nil)
					event := &eventImageReq.EventImageRequestedBuildHandler{}
					event.Data = *edgePayload
					event.Consume(ctx, mockImageService)
				})
				Context("image process errors", func() {
					It("should not be ok", func() {
						orgID := faker.UUIDHyphenated()

						image := &models.Image{
							OrgID:        orgID,
							Commit:       &models.Commit{},
							Distribution: "rhel-90",
							OutputTypes:  []string{models.ImageTypeInstaller},
							Version:      1,
							Name:         faker.Name(),
						}

						ident, err := common.GetIdentityFromContext(ctx)
						Expect(err).To(BeNil())

						edgePayload := &models.EdgeImageRequestedEventPayload{
							EdgeBasePayload: models.EdgeBasePayload{
								Identity:       ident,
								LastHandleTime: time.Now().Format(time.RFC3339),
								RequestID:      image.RequestID,
							},
							NewImage: *image,
						}

						mockImageService.EXPECT().ProcessImage(gomock.Any(), gomock.Any()).Return(errors.New("this failed"))
						event := &eventImageReq.EventImageRequestedBuildHandler{}
						event.Data = *edgePayload
						event.Consume(ctx, mockImageService)
					})
				})
			})
		})
	})
})
