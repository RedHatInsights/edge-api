// FIXME: golangci-lint
// nolint:errcheck,gosec,govet,revive,typecheck
package services_test

import (
	"context"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("ValidateDevicesImageSetWithCommit", func() {
	faker.SetRandomNumberBoundaries(1000, 100000) // set the boundaries for the random number generator - avoids collisions
	var (
		ctx           context.Context
		commitService services.CommitServiceInterface
		device        models.Device
		commits       []models.Commit
		imageSet      models.ImageSet
		images        []models.Image
	)
	BeforeEach(func() {
		orgID := common.DefaultOrgID

		imageSet = models.ImageSet{
			OrgID: orgID,
		}
		db.DB.Create(&imageSet)

		commits = []models.Commit{{OrgID: orgID, Name: "1"},
			{OrgID: orgID, Name: "2"},
			{OrgID: orgID, Name: "3"},
			{OrgID: orgID, Name: "4"},
			{OrgID: orgID, Name: "5"}}
		db.DB.Debug().Create(&commits)

		images = []models.Image{
			{OrgID: orgID, CommitID: commits[0].ID, Status: models.ImageStatusSuccess, Version: 1, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[1].ID, Status: models.ImageStatusSuccess, Version: 2, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[2].ID, Status: models.ImageStatusSuccess, Version: 3, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[3].ID, Status: models.ImageStatusSuccess, Version: 4, ImageSetID: &imageSet.ID},
			{OrgID: orgID, CommitID: commits[4].ID, Status: models.ImageStatusSuccess, Version: 5, ImageSetID: &imageSet.ID},
		}
		db.DB.Debug().Create(&images)

		device = models.Device{
			OrgID:   orgID,
			UUID:    faker.UUIDHyphenated(),
			ImageID: images[1].ID,
		}
		db.DB.Create(&device)
		ctx = context.Background()
		commitService = services.NewCommitService(ctx, log.NewEntry(log.StandardLogger()))
	})

	Context("Validate if user provided commitID belong to same ImageSet and its valid to perform update", func() {

		It("commit is invalid to update", func() {
			err := commitService.ValidateDevicesImageSetWithCommit([]string{device.UUID}, commits[0].ID)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("commit not valid to update"))
		})

		It("device not found for comit", func() {
			err := commitService.ValidateDevicesImageSetWithCommit([]string{faker.UUIDHyphenated()}, commits[0].ID)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("image-set was not found"))
		})

		It("commit not found for device", func() {
			err := commitService.ValidateDevicesImageSetWithCommit([]string{device.UUID}, 9999999)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(Equal("commit image does not found"))
		})

		It("should not return error", func() {
			err := commitService.ValidateDevicesImageSetWithCommit([]string{device.UUID}, commits[3].ID)
			Expect(err).To(BeNil())
		})
	})
})
