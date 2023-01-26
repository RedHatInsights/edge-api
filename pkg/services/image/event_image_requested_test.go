// nolint:revive,typecheck
package image

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

	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Event Image Build Requested Test", func() {
	var ctrl *gomock.Controller
	var ctx context.Context
	var mockImageService *mock_services.MockImageServiceInterface
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)

		ctx = context.Background()
		ctx = utility.ContextWithLogger(ctx, log.NewEntry(log.StandardLogger()))
	})

	AfterEach(func() {
		ctrl.Finish()
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

					mockImageService.EXPECT().SetLog(gomock.Any()).Return()
					mockImageService.EXPECT().ProcessImage(gomock.Any(), gomock.Any()).Return(nil)
					event := &EventImageRequestedBuildHandler{}
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

						mockImageService.EXPECT().SetLog(gomock.Any()).Return()
						mockImageService.EXPECT().ProcessImage(gomock.Any(), gomock.Any()).Return(errors.New("this failed"))
						event := &EventImageRequestedBuildHandler{}
						event.Data = *edgePayload
						event.Consume(ctx, mockImageService)
					})
				})
			})
		})
	})
})
