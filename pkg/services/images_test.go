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
})
