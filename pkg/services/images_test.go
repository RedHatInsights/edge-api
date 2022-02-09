package services_test

import (
	"context"
	"fmt"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/clients/imagebuilder/mock_imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Image Service Test", func() {
	var service services.ImageService
	var hash string
	var mockImageBuilderClient *mock_imagebuilder.MockClientInterface
	var mockRepoService *mock_services.MockRepoServiceInterface
	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		defer ctrl.Finish()
		mockImageBuilderClient = mock_imagebuilder.NewMockClientInterface(ctrl)
		mockRepoService = mock_services.NewMockRepoServiceInterface(ctrl)
		service = services.ImageService{
			Service:      services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
			ImageBuilder: mockImageBuilderClient,
			RepoService:  mockRepoService,
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
				Expect(err).To(MatchError(new(services.ImageNotFoundError)))
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
				Expect(err).To(MatchError(new(services.ImageNotFoundError)))
			})
			It("should have a empty image", func() {
				Expect(image).To(BeNil())
			})
		})

	})
	Describe("update image", func() {
		Context("when previous image doesnt exist", func() {
			var err error
			BeforeEach(func() {
				err = service.UpdateImage(&models.Image{}, nil)
			})
			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageNotFoundError)))
			})
		})
		Context("when previous image has failed status", func() {
			It("should have an error returned by image builder", func() {
				id, _ := faker.RandomInt(1)
				uid := uint(id[0])
				imageSet := &models.ImageSet{}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					Status:     models.ImageStatusSuccess,
					Commit:     &models.Commit{RepoID: &uid},
					Version:    1,
					Name:       faker.Name(),
					ImageSetID: &imageSet.ID,
				}
				image := &models.Image{
					Commit:      &models.Commit{},
					OutputTypes: []string{models.ImageTypeCommit},
					Version:     2,
					Name:        previousImage.Name,
				}
				result = db.DB.Save(previousImage)
				Expect(result.Error).To(Not(HaveOccurred()))
				expectedErr := fmt.Errorf("Failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(&models.Repo{}, nil)
				actualErr := service.UpdateImage(image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
			})
		})
		Context("when previous image has success status", func() {
			It("should have the parent image repo url set as parent commit url", func() {
				id, _ := faker.RandomInt(1)
				uid := uint(id[0])
				imageSet := &models.ImageSet{}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					Status:     models.ImageStatusSuccess,
					Commit:     &models.Commit{RepoID: &uid},
					Version:    1,
					Name:       faker.Name(),
					ImageSetID: &imageSet.ID,
				}
				image := &models.Image{
					Commit:      &models.Commit{},
					OutputTypes: []string{models.ImageTypeCommit},
					Version:     2,
					Name:        previousImage.Name,
				}
				result = db.DB.Save(previousImage)
				Expect(result.Error).To(Not(HaveOccurred()))

				parentRepo := &models.Repo{URL: faker.URL()}
				expectedErr := fmt.Errorf("Failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(parentRepo, nil)
				actualErr := service.UpdateImage(image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.OSTreeParentCommit).To(Equal(parentRepo.URL))
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
				service.SetFinalImageStatus(image)

				Expect(image.Status).To(Equal(models.ImageStatusSuccess))
			})
			It("should set status to error when error", func() {
				image := &models.Image{
					Commit: &models.Commit{
						Status: models.ImageStatusError,
					},
					OutputTypes: []string{models.ImageTypeCommit},
				}
				service.SetFinalImageStatus(image)

				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
			It("should set status as error when building", func() {
				image := &models.Image{
					Commit: &models.Commit{
						Status: models.ImageStatusBuilding,
					},
					OutputTypes: []string{models.ImageTypeCommit},
				}
				service.SetFinalImageStatus(image)

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
				service.SetFinalImageStatus(image)

				Expect(image.Status).To(Equal(models.ImageStatusSuccess))
			})
			It("should set status to error when error", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusError,
					},
					OutputTypes: []string{models.ImageTypeInstaller},
				}
				service.SetFinalImageStatus(image)

				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
			It("should set status as error when building", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusBuilding,
					},
					OutputTypes: []string{models.ImageTypeInstaller},
				}
				service.SetFinalImageStatus(image)

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
				service.SetFinalImageStatus(image)

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
				service.SetFinalImageStatus(image)

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
				service.SetFinalImageStatus(image)

				Expect(image.Installer.Status).To(Equal(models.ImageStatusError))
				Expect(image.Status).To(Equal(models.ImageStatusError))
			})
		})

		Context("when setting the status to retry an image build", func() {
			It("should set status to building", func() {
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusError,
					},
					Commit: &models.Commit{
						Status: models.ImageStatusError,
					},
					Status:      models.ImageStatusError,
					OutputTypes: []string{models.ImageTypeInstaller, models.ImageTypeCommit},
				}
				err := service.SetBuildingStatusOnImageToRetryBuild(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusBuilding))
				Expect(image.Commit.Status).To(Equal(models.ImageStatusBuilding))
				Expect(image.Installer.Status).To(Equal(models.ImageStatusCreated))
			})
		})
		Context("when checking if the image version we are trying to create is duplicate", func() {
			It("shouldnt be able to", func() {
				image := &models.Image{Version: 1, Name: "image-same-name"}
				db.DB.Save(image)
				db.DB.Save(&models.Image{Version: 2, Name: "image-same-name"})
				err := service.CheckIfIsLatestVersion(image)

				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageVersionAlreadyExists)))
			})
		})
	})
	Describe("validate images packages account", func() {
		Context("when creating an image using third party repository", func() {
			It("should validate the images packages from account", func() {
				var repos []models.ThirdPartyRepo
				account := "00000"
				validRepos, err := services.ValidateAllImagesPackagesAreFromAccount(account, repos)
				Expect(err).ToNot(HaveOccurred())
				Expect(validRepos).ToNot(BeNil())

			})
			It("should give an error", func() {
				var repos []models.ThirdPartyRepo
				account := ""
				validRepos, err := services.ValidateAllImagesPackagesAreFromAccount(account, repos)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("repository information is not valid"))
				Expect(validRepos).To(BeNil())
			})
		})
	})
})
