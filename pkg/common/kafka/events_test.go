package kafkacommon_test

import (
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2" // nolint: revive
	. "github.com/onsi/gomega"    // nolint: revive
	"github.com/redhatinsights/edge-api/config"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
)

var _ = Describe("Test Create Edge Event", func() {
	var ctrl *gomock.Controller

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		config.Init()

	})

	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("Create Edge Event", func() {
		It("should be ok", func() {
			var ident identity.XRHID
			ident.Identity.OrgID = common.DefaultOrgID
			imageSet := &models.ImageSet{
				Name:    "test",
				Version: 2,
				OrgID:   common.DefaultOrgID,
			}
			image := &models.Image{
				Commit: &models.Commit{
					OSTreeCommit: faker.UUIDHyphenated(),
					OrgID:        common.DefaultOrgID,
					Status:       "BUILDING",
				},
				Status:     models.ImageStatusSuccess,
				ImageSetID: &imageSet.ID,
				Version:    1,
				OrgID:      common.DefaultOrgID,
				Name:       "test",
			}
			edgePayload := &models.EdgeImageRequestedEventPayload{
				EdgeBasePayload: models.EdgeBasePayload{
					Identity:       ident,
					LastHandleTime: time.Now().Format(time.RFC3339),
					RequestID:      uuid.New().String(),
				},
				NewImage: *image,
			}

			event := kafkacommon.CreateEdgeEvent(common.DefaultOrgID, models.SourceEdgeEventAPI, image.RequestID, models.EventTypeEdgeImageRequested, image.Name, edgePayload)
			Expect(event.Data).To(Equal(edgePayload))
			Expect(event.RedHatOrgID).To(Equal(common.DefaultOrgID))
			Expect(event.Type).To(Equal(models.EventTypeEdgeImageRequested))
			Expect(event.Source).To(Equal(models.SourceEdgeEventAPI))
			Expect(event.Subject).To(Equal(image.Name))

		})
	})
})
