package deleteimages_test

import (
	"os"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/redhatinsights/edge-api/cmd/cleanup/deleteimages"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	"gorm.io/gorm"
)

var _ = Describe("Delete Images", func() {

	Context("BuildImagesNamesToKeepQuery", func() {
		var initialImagesWithNamesToKeep []string

		BeforeEach(func() {
			initialImagesWithNamesToKeep = deleteimages.ImagesWithNamesToKeep
			deleteimages.ImagesWithNamesToKeep = []string{
				faker.Name(),
				faker.Name(),
			}
		})

		AfterEach(func() {
			deleteimages.ImagesWithNamesToKeep = initialImagesWithNamesToKeep
		})

	})

	When("CleanUPDeleteImages feature flag is disabled", func() {

		BeforeEach(func() {
			err := os.Unsetenv(feature.CleanUPDeleteImages.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not run DeleteAllImages when feature flag is disabled", func() {
			err := deleteimages.DeleteAllImages(nil)
			Expect(err).To(MatchError(deleteimages.ErrDeleteImagesCleanUpNotAvailable))
		})
	})

	When("CleanUPDeleteImages feature flag is enabled", func() {

		var initialImagesWithNamesToKeep []string

		BeforeEach(func() {
			err := os.Setenv(feature.CleanUPDeleteImages.EnvVar, "true")
			Expect(err).NotTo(HaveOccurred())

			initialImagesWithNamesToKeep = deleteimages.ImagesWithNamesToKeep
			deleteimages.ImagesWithNamesToKeep = []string{
				faker.Name(),
				faker.Name(),
			}
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.CleanUPDeleteImages.EnvVar)
			Expect(err).NotTo(HaveOccurred())
			deleteimages.ImagesWithNamesToKeep = initialImagesWithNamesToKeep
		})

		Context("Delete Orphan images", func() {
			var orgID string
			var imageSet models.ImageSet
			var image models.Image
			var deletedImageSet models.ImageSet
			var imageToDelete models.Image

			BeforeEach(func() {
				// setup only once
				if orgID == "" {
					orgID = faker.UUIDHyphenated()
					imageSet = models.ImageSet{OrgID: orgID, Name: faker.Name()}
					err := db.DB.Create(&imageSet).Error
					Expect(err).ToNot(HaveOccurred())

					image = models.Image{OrgID: orgID, Name: imageSet.Name, ImageSetID: &imageSet.ID}
					err = db.DB.Create(&image).Error
					Expect(err).ToNot(HaveOccurred())

					deletedImageSet = models.ImageSet{OrgID: orgID, Name: faker.Name()}
					err = db.DB.Create(&deletedImageSet).Error
					Expect(err).ToNot(HaveOccurred())

					imageToDelete = models.Image{OrgID: orgID, Name: deletedImageSet.Name, ImageSetID: &deletedImageSet.ID}
					err = db.DB.Create(&imageToDelete).Error
					Expect(err).ToNot(HaveOccurred())

					// delete deletedimageset
					err = db.DB.Delete(&deletedImageSet).Error
					Expect(err).ToNot(HaveOccurred())
				}
			})

			It("should run DeleteAllImages successfully", func() {
				err := deleteimages.DeleteAllImages(db.DB.Where("images.org_id = ?", orgID).Session(&gorm.Session{}))
				Expect(err).ToNot(HaveOccurred())
			})

			It("imageToDelete should have been soft", func() {
				err := db.DB.First(&imageToDelete).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// ensure imageToDelete still exist in db
				err = db.DB.Unscoped().First(&imageToDelete).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("image should not have been affected", func() {
				err := db.DB.First(&image).Error
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Delete images", func() {
			var orgID string
			var device models.Device
			// old image to keep , because a device is linked to it
			var deviceImage models.Image
			// old image that should be deleted
			var oldImageToDelete models.Image
			// old image to keep , because it's name is in the list to keep
			var oldImageToKeep models.Image
			// new image to keep even it's name is not in the list to keep
			var newImageToKeep models.Image

			var initialDeleteImagesOlderThan = deleteimages.DeleteImagesOlderThan

			BeforeEach(func() {
				// setup only once
				if orgID == "" {
					orgID = faker.UUIDHyphenated()
					deviceImage = models.Image{OrgID: orgID, Name: faker.Name()}
					err := db.DB.Create(&deviceImage).Error
					Expect(err).ToNot(HaveOccurred())

					device = models.Device{OrgID: orgID, ImageID: deviceImage.ID}
					err = db.DB.Create(&device).Error
					Expect(err).ToNot(HaveOccurred())

					oldImageToDelete = models.Image{OrgID: orgID, Name: faker.Name()}
					err = db.DB.Create(&oldImageToDelete).Error
					Expect(err).ToNot(HaveOccurred())

					oldImageToKeep = models.Image{OrgID: orgID, Name: deleteimages.ImagesWithNamesToKeep[0]}
					err = db.DB.Create(&oldImageToKeep).Error
					Expect(err).ToNot(HaveOccurred())

					// create a new DeleteImagesOlderThan to speed up tests, as we cannot wait for 7 days
					deleteimages.DeleteImagesOlderThan = 100 * time.Millisecond

					// make time elapse for same amount to force consider the already created mages as old images
					time.Sleep(deleteimages.DeleteImagesOlderThan + 10*time.Millisecond)

					newImageToKeep = models.Image{OrgID: orgID, Name: faker.Name()}
					err = db.DB.Create(&newImageToKeep).Error
					Expect(err).ToNot(HaveOccurred())
				}
			})

			AfterEach(func() {
				deleteimages.DeleteImagesOlderThan = initialDeleteImagesOlderThan
			})

			It("should run DeleteAllImages successfully", func() {
				err := deleteimages.DeleteAllImages(db.DB.Where("images.org_id = ?", orgID).Session(&gorm.Session{}))
				Expect(err).ToNot(HaveOccurred())
			})

			It("should delete old images with name not in the list", func() {
				err := db.DB.First(&oldImageToDelete).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// ensure record still in db
				err = db.DB.Unscoped().First(&oldImageToDelete).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should keep images with name in the list", func() {
				err := db.DB.First(&oldImageToKeep).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should keep images with linked devices", func() {
				err := db.DB.First(&deviceImage).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should keep new images", func() {
				err := db.DB.First(&newImageToKeep).Error
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
