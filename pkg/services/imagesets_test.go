// FIXME: golangci-lint
// nolint:revive
package services_test

import (
	"context"
	"fmt"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("ImageSets Service Test", func() {
	service := services.NewImageSetsService(context.Background(), log.NewEntry(log.StandardLogger()))
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
	Describe("ImageSetsView", func() {
		OrgID := common.DefaultOrgID
		CommonName := faker.UUIDHyphenated()

		imageSet1 := models.ImageSet{OrgID: OrgID, Name: CommonName + "-" + faker.Name(), Version: 3}
		db.DB.Create(&imageSet1)
		image1 := models.Image{OrgID: OrgID, Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 1, Status: models.ImageStatusSuccess}
		image1.Installer = &models.Installer{OrgID: OrgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess}
		db.DB.Create(&image1)
		image2 := models.Image{OrgID: OrgID, Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 2, Status: models.ImageStatusSuccess}
		image2.Installer = &models.Installer{OrgID: OrgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess}
		db.DB.Create(&image2)
		// image 3 Is with empty url and error status
		image3 := models.Image{OrgID: OrgID, Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 3, Status: models.ImageStatusError}
		image3.Installer = &models.Installer{OrgID: OrgID, Status: models.ImageStatusError}
		db.DB.Create(&image3)

		// other image set
		otherImageSet1 := models.ImageSet{OrgID: OrgID, Name: CommonName + "-" + faker.Name(), Version: 1}
		db.DB.Create(&otherImageSet1)
		otherImage1 := models.Image{OrgID: OrgID, Name: otherImageSet1.Name, ImageSetID: &otherImageSet1.ID, Version: 1, Status: models.ImageStatusSuccess}
		otherImage1.Installer = &models.Installer{OrgID: OrgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess}
		db.DB.Create(&otherImage1)

		It("GetStorageInstallerIsoURL return the right iso path", func() {
			isoPath := services.GetStorageInstallerIsoURL(125)
			Expect(isoPath).To(Equal("/api/edge/v1/storage/isos/125"))
		})

		It("GetStorageInstallerIsoURL return empty string when installer not defined", func() {
			isoPath := services.GetStorageInstallerIsoURL(0)
			Expect(isoPath).To(BeEmpty())
		})

		It("should return The right image view count", func() {

			dbFilter := db.DB.Where("image_sets.name LIKE ? ", CommonName+"%")

			count, err := service.GetImageSetsViewCount(dbFilter)
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(int64(2)))
		})

		It("should return image-set view with corresponding installer iso url and error status ", func() {

			dbFilter := db.DB.Debug().Where("image_sets.name = ? ", imageSet1.Name)

			imageSetsView, err := service.GetImageSetsView(100, 0, dbFilter)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(*imageSetsView)).To(Equal(1))
			imageSetRow := (*imageSetsView)[0]

			Expect(imageSetRow.ID).To(Equal(imageSet1.ID))
			Expect(imageSetRow.Version).To(Equal(image3.Version))
			Expect(imageSetRow.ImageID).To(Equal(image3.ID))
			Expect(imageSetRow.ImageBuildIsoURL).To(Equal(fmt.Sprintf("/api/edge/v1/storage/isos/%d", image2.Installer.ID)))
			Expect(imageSetRow.Status).To(Equal(image3.Status))
		})

		It("should return image-set view with corresponding installer iso url and success status ", func() {

			dbFilter := db.DB.Where("image_sets.name = ? ", otherImageSet1.Name)

			imageSetsView, err := service.GetImageSetsView(100, 0, dbFilter)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(*imageSetsView)).To(Equal(1))
			imageSetRow := (*imageSetsView)[0]

			Expect(imageSetRow.ID).To(Equal(otherImageSet1.ID))
			Expect(imageSetRow.Version).To(Equal(1))
			Expect(imageSetRow.ImageBuildIsoURL).To(Equal(fmt.Sprintf("/api/edge/v1/storage/isos/%d", otherImage1.Installer.ID)))
			Expect(imageSetRow.Status).To(Equal(models.ImageStatusSuccess))
		})
	})

	Describe("ImageSetViewByID", func() {
		OrgID := common.DefaultOrgID
		CommonName := faker.UUIDHyphenated()

		imageSet1 := models.ImageSet{OrgID: OrgID, Name: CommonName + "-" + faker.Name(), Version: 3}
		db.DB.Create(&imageSet1)
		image1 := models.Image{OrgID: OrgID, Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 1, Status: models.ImageStatusSuccess}
		image1.Installer = &models.Installer{OrgID: OrgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess}
		image1.Commit = &models.Commit{OrgID: OrgID, Status: models.ImageStatusSuccess}
		db.DB.Create(&image1)
		image2 := models.Image{OrgID: OrgID, Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 2, Status: models.ImageStatusSuccess}
		image2.Installer = &models.Installer{OrgID: OrgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess}
		image2.Commit = &models.Commit{OrgID: OrgID, Status: models.ImageStatusSuccess}
		db.DB.Create(&image2)
		// image 3 Is with empty url and error status
		image3 := models.Image{OrgID: OrgID, Name: imageSet1.Name, ImageSetID: &imageSet1.ID, Version: 3, Status: models.ImageStatusError}
		image3.Commit = &models.Commit{OrgID: OrgID, Status: models.ImageStatusSuccess}
		image3.Installer = &models.Installer{OrgID: OrgID, Status: models.ImageStatusError}
		db.DB.Create(&image3)

		// other image set
		otherImageSet1 := models.ImageSet{OrgID: OrgID, Name: CommonName + "-" + faker.Name(), Version: 1}
		db.DB.Create(&otherImageSet1)
		otherImage1 := models.Image{OrgID: OrgID, Name: otherImageSet1.Name, ImageSetID: &otherImageSet1.ID, Version: 1, Status: models.ImageStatusSuccess}
		otherImage1.Installer = &models.Installer{OrgID: OrgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess}
		otherImage1.Commit = &models.Commit{OrgID: OrgID, Status: models.ImageStatusSuccess}
		db.DB.Create(&otherImage1)

		It("GetImageSetViewByID returns ImageSet details as expected", func() {
			imagesOrderByDB := db.DB.Order("images.created_at DESC")
			imageSetIDView, err := service.GetImageSetViewByID(imageSet1.ID, 30, 0, imagesOrderByDB)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageSetIDView.ImageSet.ID).To(Equal(imageSet1.ID))
			Expect(imageSetIDView.ImageBuildIsoURL).To(Equal(fmt.Sprintf("/api/edge/v1/storage/isos/%d", image2.Installer.ID)))
			Expect(imageSetIDView.LastImageDetails.Image.ID).To(Equal(image3.ID))
			Expect(imageSetIDView.LastImageDetails.Image.Installer.ImageBuildISOURL).To(BeEmpty())
		})
		It("GetImagesViewData returns image set images views as expected", func() {
			imagesOrderByDB := db.DB.Order("images.created_at DESC")
			imagesVewData, err := service.GetImagesViewData(imageSet1.ID, 30, 0, imagesOrderByDB)
			Expect(err).ToNot(HaveOccurred())
			Expect(imagesVewData.Count).To(Equal(int64(3)))
			for ind, expectedImage := range []models.Image{image3, image2, image1} {
				imageView := imagesVewData.Data[ind]
				Expect(imageView.ID).To(Equal(expectedImage.ID))
				Expect(imageView.Status).To(Equal(expectedImage.Status))
				Expect(imageView.Version).To(Equal(expectedImage.Version))
			}
		})
		It("GetImageSetImageViewByID returns ImageSet image details as expected", func() {
			imageSetIDView, err := service.GetImageSetImageViewByID(imageSet1.ID, image1.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageSetIDView.ImageSet.ID).To(Equal(imageSet1.ID))
			Expect(imageSetIDView.ImageBuildIsoURL).To(Equal(fmt.Sprintf("/api/edge/v1/storage/isos/%d", image2.Installer.ID)))
			Expect(imageSetIDView.ImageDetails.Image.ID).To(Equal(image1.ID))
			Expect(imageSetIDView.ImageDetails.Image.Installer.ImageBuildISOURL).To(Equal(fmt.Sprintf("/api/edge/v1/storage/isos/%d", image1.Installer.ID)))
		})
		It("GetImageSetImageViewByID returns image not found error", func() {
			_, err := service.GetImageSetImageViewByID(imageSet1.ID, otherImage1.ID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(new(services.ImageNotFoundError)))
		})
		It("GetImageSetImageViewByID returns ImageSet not found error", func() {
			innerImageSet := models.ImageSet{OrgID: faker.UUIDHyphenated(), Name: faker.Name(), Version: 1}
			db.DB.Create(&innerImageSet)
			_, err := service.GetImageSetImageViewByID(innerImageSet.ID, image1.ID)
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(new(services.ImageSetNotFoundError)))
		})
	})
})
