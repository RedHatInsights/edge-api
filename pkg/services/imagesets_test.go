package services_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
)

var _ = Describe("ImageSets Service Test", func() {
	var service services.ImageSetsService
	Describe("get imageSet", func() {
		When("image-set exists", func() {
			var imageSet1 *models.ImageSet

			BeforeEach(func() {
				imageSet1 = &models.ImageSet{
					Name:    "test",
					Version: 2,
					OrgID:   common.DefaultOrgID,
				}
				result := db.DB.Create(imageSet1)
				Expect(result.Error).ToNot(HaveOccurred())
			})
			Context("by ID", func() {
				var imageSet *models.ImageSet
				var err error
				BeforeEach(func() {
					imageSet, err = service.GetImageSetsByID(int(imageSet1.ID))
				})
				It("should not have an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
				It("should have a v1 image", func() {
					Expect(imageSet.ID).To(Equal(imageSet1.ID))
				})
			})
		})
	})
})
