package services

import (
	"context"
	"fmt"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var _ = Describe("Image Service Test", func() {
	var service ImageService
	var hash string
	BeforeEach(func() {
		service = ImageService{
			ctx: context.Background(),
		}
	})
	Describe("get image", func() {
		Context("by id when image is not found", func() {
			var image *models.Image
			var err error
			BeforeEach(func() {
				id, _ := faker.RandomInt(1)
				image, err = service.GetImageByID(fmt.Sprint(id[0]))
			})
			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(ImageNotFoundError)))
			})
			It("should have a empty image", func() {
				Expect(image).To(BeNil())
			})
		})

		Context("by hash when image is not found", func() {
			var image *models.Image
			var err error
			BeforeEach(func() {
				hash = faker.Word()
				image, err = service.GetImageByOSTreeCommitHash(hash)
			})

			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(ImageNotFoundError)))
			})
			It("should have a empty image", func() {
				Expect(image).To(BeNil())
			})
		})

	})
	Describe("should set status properly on a built image", func() {
		Context("when image is type of rhel for edge commit", func() {
			It("should set status to success when success", func() {
				image := &models.Image{
					Commit: &models.Commit{
						Status: models.ImageStatusSuccess,
					},
					OutputTypes: []string{models.ImageTypeCommit},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusSuccess))
			})
			It("should set status to error when error", func() {
				image := &models.Image{
					Commit: &models.Commit{
						Status: models.ImageStatusError,
					},
					OutputTypes: []string{models.ImageTypeCommit},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
			It("should set status as error when building", func() {
				image := &models.Image{
					Commit: &models.Commit{
						Status: models.ImageStatusBuilding,
					},
					OutputTypes: []string{models.ImageTypeCommit},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Commit.Status).To(Equal(models.ImageStatusError))
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
		})
		Context("when image is type of rhel for edge installer", func() {
			It("should set status to success when success", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusSuccess,
					},
					OutputTypes: []string{models.ImageTypeInstaller},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusSuccess))
			})
			It("should set status to error when error", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusError,
					},
					OutputTypes: []string{models.ImageTypeInstaller},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
			It("should set status as error when building", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusBuilding,
					},
					OutputTypes: []string{models.ImageTypeInstaller},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Installer.Status).To(Equal(models.ImageStatusError))
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
		})

		Context("when image is type of rhel for edge installer and has output type commit", func() {
			It("should set status to success when success", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusSuccess,
					},
					Commit: &models.Commit{
						Status: models.ImageStatusSuccess,
					},
					OutputTypes: []string{models.ImageTypeInstaller, models.ImageTypeCommit},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusSuccess))
			})
			It("should set status to error when error", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusError,
					},
					Commit: &models.Commit{
						Status: models.ImageStatusSuccess,
					},
					OutputTypes: []string{models.ImageTypeInstaller, models.ImageTypeCommit},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
			It("should set status as error when building", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusBuilding,
					},
					Commit: &models.Commit{
						Status: models.ImageStatusSuccess,
					},
					OutputTypes: []string{models.ImageTypeInstaller, models.ImageTypeCommit},
				}
				err := service.setFinalImageStatus(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Installer.Status).To(Equal(models.ImageStatusError))
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
		})
	})
})
