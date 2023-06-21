package deleteorphanedimages

import (
	"os"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"
)

var _ = Describe("Delete orphaned images", func() {
	It("Won't do anything if the feature is disabled", func() {
		err := os.Setenv(feature.DeleteOrphanedImages.EnvVar, "false")
		Expect(err).ToNot(HaveOccurred())
	})

	Context("will delete orphaned images", func() {
		var orgID string
		// imageset to delete
		var imageSet1 *models.ImageSet
		var image11 *models.Image
		var image12 *models.Image

		// imageset to not delete
		var imageSet2 *models.ImageSet
		var image21 *models.Image
		var image22 *models.Image

		BeforeEach(func() {
			err := os.Setenv(feature.DeleteOrphanedImages.EnvVar, "true")
			Expect(err).ToNot(HaveOccurred())

			if orgID == "" {
				orgID = faker.UUIDHyphenated()

				// setup imageset 1 (set to delete) + images associated
				imageSet1 = &models.ImageSet{OrgID: orgID, Name: faker.Name(), Version: 1}
				imageSet1err := db.DB.Create(&imageSet1).Error
				Expect(imageSet1err).ToNot(HaveOccurred())

				image11 = &models.Image{Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 1}
				image12 = &models.Image{Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 2}
				image1err := db.DB.Create([]models.Image{*image11, *image12}).Error
				Expect(image1err).ToNot(HaveOccurred())

				// setup image 2 (set to not delete) + images associated
				imageSet2 = &models.ImageSet{OrgID: orgID, Name: faker.Name(), Version: 2}
				imageSet2err := db.DB.Create(&imageSet2).Error
				Expect(imageSet2err).ToNot(HaveOccurred())

				image21 = &models.Image{Name: imageSet2.Name, ImageSetID: &imageSet2.ID, Version: 1}
				image22 = &models.Image{Name: imageSet2.Name, ImageSetID: &imageSet2.ID, Version: 2}
				image2err := db.DB.Create([]models.Image{*image21, *image22}).Error
				Expect(image2err).ToNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			err := os.Setenv(feature.DeleteOrphanedImages.EnvVar, "false")
			Expect(err).ToNot(HaveOccurred())
		})

		It("without errors", func() {
			var orphanedImages []models.Image
			var count int64

			imageSet1Delete := db.DB.Delete(imageSet1)
			Expect(imageSet1Delete.Error).ToNot(HaveOccurred())

			query := findOrphanedImages(db.DB, &count, &orphanedImages)
			Expect(query.Error).ToNot(HaveOccurred())
			Expect(count).To(Equal(2))
		})
	})
})
