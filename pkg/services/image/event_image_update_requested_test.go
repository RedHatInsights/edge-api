// nolint:revive,typecheck
package image

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/edge-api/pkg/services/utility"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Event Image Update Requested Test", func() {
	var ctx context.Context
	var logBuffer bytes.Buffer
	var testLog *log.Entry
	var mockImageService *mock_services.MockImageServiceInterface
	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		mockImageService = mock_services.NewMockImageServiceInterface(ctrl)
		testLog = log.NewEntry(log.StandardLogger())
		// Set the output to use our new local logBuffer
		logBuffer = bytes.Buffer{}
		testLog.Logger.SetOutput(&logBuffer)

		ctx = context.Background()
		ctx = dependencies.ContextWithServices(ctx, &dependencies.EdgeAPIServices{
			ImageService: mockImageService,
		})
		ctx = utility.ContextWithLogger(ctx, log.NewEntry(log.StandardLogger()))
	})
	Describe("consume image build update event", func() {
		When("image build update is requested", func() {
			Context("image update is processed successfully", func() {
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

					edgePayload := &models.EdgeImageUpdateRequestedEventPayload{
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
					event := &EventImageUpdateRequestedBuildHandler{}

					event.Data = *edgePayload
					event.Consume(ctx)
				})
				Context("image update process errors", func() {
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

						edgePayload := &models.EdgeImageUpdateRequestedEventPayload{
							EdgeBasePayload: models.EdgeBasePayload{
								Identity:       ident,
								LastHandleTime: time.Now().Format(time.RFC3339),
								RequestID:      image.RequestID,
							},
							NewImage: *image,
						}

						mockImageService.EXPECT().SetLog(gomock.Any()).Return()
						mockImageService.EXPECT().ProcessImage(gomock.Any(), gomock.Any()).Return(errors.New("error processing the image"))
						event := &EventImageUpdateRequestedBuildHandler{}
						event.Data = *edgePayload
						event.Consume(ctx)
						Expect(logBuffer.String()).To(ContainSubstring("Error processing the image"))

					})
				})
			})
		})
	})
})
