// FIXME: golangci-lint
// nolint:dupword,errcheck,gosec,govet,revive,typecheck
package services_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	imageBuilderClient "github.com/redhatinsights/edge-api/pkg/clients/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/clients/imagebuilder/mock_imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories/mock_repositories"
	mock_kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka/mock_kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	apiErrors "github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/config"
)

var _ = Describe("Image Service Test", func() {
	ctx := context.Background()

	var ctrl *gomock.Controller
	var service services.ImageService
	var hash string
	var mockImageBuilderClient *mock_imagebuilder.MockClientInterface
	var mockRepoService *mock_services.MockRepoServiceInterface
	var mockProducerService *mock_kafkacommon.MockProducerServiceInterface
	var mockProducer *mock_kafkacommon.MockProducer
	var mockTopicService *mock_kafkacommon.MockTopicServiceInterface
	var mockFilesService *mock_services.MockFilesService
	var mockRepoBuilder *mock_services.MockRepoBuilderInterface
	var mockRepositories *mock_repositories.MockClientInterface

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockImageBuilderClient = mock_imagebuilder.NewMockClientInterface(ctrl)
		mockRepoService = mock_services.NewMockRepoServiceInterface(ctrl)
		mockProducerService = mock_kafkacommon.NewMockProducerServiceInterface(ctrl)
		mockProducer = mock_kafkacommon.NewMockProducer(ctrl)
		mockTopicService = mock_kafkacommon.NewMockTopicServiceInterface(ctrl)
		mockFilesService = mock_services.NewMockFilesService(ctrl)
		mockRepoBuilder = mock_services.NewMockRepoBuilderInterface(ctrl)
		mockRepositories = mock_repositories.NewMockClientInterface(ctrl)
		service = services.ImageService{
			Service:         services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
			ImageBuilder:    mockImageBuilderClient,
			RepoService:     mockRepoService,
			ProducerService: mockProducerService,
			TopicService:    mockTopicService,
			FilesService:    mockFilesService,
			RepoBuilder:     mockRepoBuilder,
			Repositories:    mockRepositories,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("get image", func() {
		Context("#GetImageByIDExtended", func() {
			var image models.Image
			var orgID string

			BeforeEach(func() {
				orgID = common.DefaultOrgID
				image = models.Image{
					OrgID: orgID,
					Name:  faker.UUIDHyphenated(),
					ThirdPartyRepositories: []models.ThirdPartyRepo{
						{Name: faker.Name(), URL: faker.URL(), OrgID: orgID},
						{Name: faker.Name(), URL: faker.URL(), OrgID: orgID},
					},
					CustomPackages: []models.Package{
						{Name: "nano"},
						{Name: "vim"},
					},
					Commit: &models.Commit{OrgID: orgID},
				}
				err := db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the image with requested db extensions when defined", func() {
				resultImage, err := service.GetImageByIDExtended(
					image.ID, db.DB.Preload("ThirdPartyRepositories").Preload("CustomPackages").Joins("Commit"),
				)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultImage).ToNot(BeNil())
				Expect(resultImage.ID).To(Equal(image.ID))
				Expect(len(resultImage.ThirdPartyRepositories) > 0).To(BeTrue())
				Expect(len(resultImage.ThirdPartyRepositories)).To(Equal(len(image.ThirdPartyRepositories)))
				Expect(len(resultImage.CustomPackages) > 0).To(BeTrue())
				Expect(len(resultImage.CustomPackages)).To(Equal(len(image.CustomPackages)))
				Expect(resultImage.Commit).ToNot(BeNil())
				Expect(resultImage.Commit.ID).To(Equal(image.Commit.ID))
			})

			It("should return the image without requested db extensions when not defined", func() {
				resultImage, err := service.GetImageByIDExtended(image.ID, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(resultImage).ToNot(BeNil())
				Expect(resultImage.ID).To(Equal(image.ID))
				Expect(len(resultImage.ThirdPartyRepositories) == 0).To(BeTrue())
				Expect(len(resultImage.CustomPackages) == 0).To(BeTrue())
			})

			It("should return error when orgID is not defined", func() {
				originalAuth := config.Get().Auth
				// restore auth
				defer func(auth bool) {
					config.Get().Auth = auth
				}(originalAuth)
				config.Get().Auth = true
				// create a service with identity with empty orgID
				ctx := identity.WithIdentity(context.Background(), identity.XRHID{Identity: identity.Identity{OrgID: ""}})

				service = services.ImageService{
					Service: services.NewService(ctx, log.NewEntry(log.StandardLogger())),
				}

				_, err := service.GetImageByIDExtended(image.ID, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Org ID is not set"))
			})

			It("should return error when image does not exist", func() {
				_, err := service.GetImageByIDExtended(uint(99999999), nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(services.ImageNotFoundErrorMsg))
			})

			It("should return error on query failure", func() {
				collectionName := "this_collection_does_not_exit"
				expectedErrorMessage := fmt.Sprintf("%s: unsupported relations for schema Image", collectionName)

				_, err := service.GetImageByIDExtended(image.ID, db.DB.Preload(collectionName))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErrorMessage))
			})
		})

		When("image is not found", func() {
			Context("by id", func() {
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
			Context("by hash", func() {
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
		When("image exists", func() {
			var imageV1, imageV2, imageV3 *models.Image
			var imageSet *models.ImageSet

			BeforeEach(func() {
				imageSet = &models.ImageSet{
					Name:    "test",
					Version: 2,
					OrgID:   common.DefaultOrgID,
				}
				result := db.DB.Create(imageSet)
				Expect(result.Error).ToNot(HaveOccurred())
				imageV1 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        common.DefaultOrgID,
						InstalledPackages: []models.InstalledPackage{
							{Name: "vim"},
						},
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      common.DefaultOrgID,
				}
				result = db.DB.Create(imageV1.Commit)
				Expect(result.Error).ToNot(HaveOccurred())
				result = db.DB.Create(imageV1)
				Expect(result.Error).ToNot(HaveOccurred())
				imageV2 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        common.DefaultOrgID,
						InstalledPackages: []models.InstalledPackage{
							{Name: "vim"},
						},
					},
					Status:     models.ImageStatusError,
					ImageSetID: &imageSet.ID,
					Version:    2,
					OrgID:      common.DefaultOrgID,
				}
				db.DB.Create(imageV2.Commit.InstalledPackages)
				db.DB.Create(imageV2.Commit)
				db.DB.Create(imageV2)
				imageV3 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        common.DefaultOrgID,
						InstalledPackages: []models.InstalledPackage{
							{Name: "vim"},
						},
					},

					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    3,
					OrgID:      common.DefaultOrgID,
				}
				db.DB.Create(imageV3.Commit.InstalledPackages)
				db.DB.Create(imageV3.Commit)
				db.DB.Create(imageV3)
			})
			Context("by ID", func() {
				var image *models.Image
				var err error
				BeforeEach(func() {
					image, err = service.GetImageByID(fmt.Sprint(imageV1.ID))
				})
				It("should not have an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
				It("should have a v1 image", func() {
					Expect(image.ID).To(Equal(imageV1.ID))
				})
			})
			Context("by hash", func() {
				var image *models.Image
				var err error
				BeforeEach(func() {
					image, err = service.GetImageByOSTreeCommitHash(imageV1.Commit.OSTreeCommit)
				})
				It("should not have an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
				It("should have a v1 image", func() {
					Expect(image.ID).To(Equal(imageV1.ID))
				})
			})
			Context("when rollback image exists", func() {
				var image *models.Image
				var err error
				BeforeEach(func() {
					image, err = service.GetRollbackImage(imageV3)
				})
				It("should have an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})
				It("should have a v1 image", func() {
					Expect(image.ID).To(Equal(imageV1.ID))
				})
				It("should have a total packages", func() {
					Expect(image.TotalPackages).To(Equal(1))
				})
			})
			Context("when rollback image does not exists", func() {
				var image *models.Image
				var err error
				BeforeEach(func() {
					image, err = service.GetRollbackImage(imageV1)
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
	})
	Describe("#CreateImage", func() {
		Context("when creating a new image", func() {
			It("should raise OrgIDNotSet", func() {
				image := models.Image{}
				error := service.CreateImage(&image)
				Expect(error).To(MatchError(new(services.OrgIDNotSet)))
			})
			It("should raise ImageNameUndefined", func() {
				image := models.Image{OrgID: faker.UUIDHyphenated()}
				error := service.CreateImage(&image)
				Expect(error).To(MatchError(new(services.ImageNameUndefined)))
			})
			It("should raise ImageNameAlreadyExists", func() {
				orgId := faker.UUIDHyphenated()
				name := faker.UUIDHyphenated()
				image := &models.Image{OrgID: orgId, Name: name}
				result := db.DB.Create(image)
				Expect(result.Error).ToNot(HaveOccurred())
				error := service.CreateImage(image)
				Expect(error).To(MatchError(new(services.ImageNameAlreadyExists)))
			})
			It("should raise ThirdPartyRepositoryNotFound", func() {
				orgID := faker.UUIDHyphenated()
				name := faker.UUIDHyphenated()
				repos := []models.ThirdPartyRepo{
					{OrgID: orgID, Name: faker.UUIDHyphenated(), URL: "https://repo1.simple.com"},
				}
				image := models.Image{
					OrgID:                  orgID,
					Name:                   name,
					Distribution:           "rhel-90",
					ThirdPartyRepositories: repos,
				}
				error := service.CreateImage(&image)
				Expect(error).To(MatchError(new(services.ThirdPartyRepositoryNotFound)))
			})

			Context("send Create Image notification", func() {
				var image models.Image
				var orgID string
				BeforeEach(func() {
					orgID = faker.UUIDHyphenated()
					err := os.Setenv("ACG_CONFIG", "true")
					Expect(err).ToNot(HaveOccurred())
					image = models.Image{
						Name:         faker.UUIDHyphenated(),
						OrgID:        orgID,
						Distribution: "rhel-91",
						ImageType:    models.ImageTypeCommit,
						OutputTypes:  []string{models.ImageTypeCommit},
						Commit:       &models.Commit{OrgID: orgID},
					}
				})

				AfterEach(func() {
					err := os.Unsetenv("ACG_CONFIG")
					Expect(err).ToNot(HaveOccurred())
				})

				It("should send Create Image notification", func() {
					mockImageBuilderClient.EXPECT().ComposeCommit(&image).Return(&image, nil)
					mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
					mockProducer.EXPECT().Produce(gomock.AssignableToTypeOf(&kafka.Message{}), nil).Return(nil)
					mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)
					err := service.CreateImage(&image)
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})
	})
	Describe("update image", func() {
		ctx := context.Background()

		Context("when previous image does not exist", func() {
			var err error
			BeforeEach(func() {
				err = service.UpdateImage(ctx, &models.Image{}, nil)
			})
			It("should have an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageNotFoundError)))
			})
		})
		Context("when previous image has failed status", func() {
			It("should have an error returned by image builder", func() {
				ctx := context.Background()
				id, _ := faker.RandomInt(1)
				uid := uint(id[0])
				orgID := faker.UUIDHyphenated()
				imageSet := &models.ImageSet{OrgID: orgID}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					OrgID:        orgID,
					Status:       models.ImageStatusSuccess,
					Commit:       &models.Commit{RepoID: &uid, OrgID: orgID},
					Version:      1,
					Distribution: "rhel-85",
					Name:         faker.Name(),
					ImageSetID:   &imageSet.ID,
				}
				image := &models.Image{
					OrgID:        orgID,
					Commit:       &models.Commit{},
					Distribution: "rhel-85",
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      2,
					Name:         previousImage.Name,
				}
				result = db.DB.Save(previousImage)
				Expect(result.Error).To(Not(HaveOccurred()))
				expectedErr := fmt.Errorf("Failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(&models.Repo{}, nil)
				actualErr := service.UpdateImage(ctx, image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
			})
		})
		Context("updating image within same major version when all previous images failed", func() {
			orgID := faker.UUIDHyphenated()
			imageName := faker.Name()
			imageSet := models.ImageSet{Name: imageName, OrgID: orgID}
			dist84 := "rhel-84"
			dist85 := "rhel-85"
			imageSetResult := db.DB.Create(&imageSet)
			previousImage1 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      1,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage1Result := db.DB.Create(&previousImage1)
			previousImage2 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      2,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage2Result := db.DB.Create(&previousImage2)
			image := &models.Image{
				OrgID:        orgID,
				Distribution: dist85,
				Commit:       &models.Commit{OrgID: orgID},
				OutputTypes:  []string{models.ImageTypeCommit},
				Name:         imageName,
			}

			It("new image should have no parent repo url and parent ostree ref", func() {
				Expect(imageSetResult.Error).ToNot(HaveOccurred())
				Expect(previousImage1Result.Error).ToNot(HaveOccurred())
				Expect(previousImage2Result.Error).ToNot(HaveOccurred())

				// simulate an error build in order to check the image values only
				expectedErr := fmt.Errorf("failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)

				actualErr := service.UpdateImage(ctx, image, &previousImage2)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.OSTreeRef).To(Equal(config.DistributionsRefs[dist85]))
				Expect(image.Commit.OSTreeParentRef).To(BeEmpty())
				Expect(image.Commit.OSTreeParentCommit).To(BeEmpty())
			})
		})
		Context("updating image within same major version when one of previous images succeed", func() {
			orgID := faker.UUIDHyphenated()
			imageName := faker.Name()
			imageSet := models.ImageSet{Name: imageName, OrgID: orgID}
			dist84 := "rhel-84"
			dist85 := "rhel-85"
			imageSetResult := db.DB.Create(&imageSet)
			previousImage1 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      1,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage1Result := db.DB.Create(&previousImage1)
			previousImage2 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusSuccess,
				Commit:       &models.Commit{Repo: &models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess}, OrgID: orgID},
				Version:      2,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage2Result := db.DB.Create(&previousImage2)
			previousImage3 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      3,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage3Result := db.DB.Create(&previousImage3)
			image := &models.Image{
				OrgID:        orgID,
				Distribution: dist85,
				Commit:       &models.Commit{OrgID: orgID},
				OutputTypes:  []string{models.ImageTypeCommit},
				Name:         imageName,
			}

			It("new image should have no parent repo url and parent ostree ref", func() {
				Expect(imageSetResult.Error).ToNot(HaveOccurred())
				Expect(previousImage1Result.Error).ToNot(HaveOccurred())
				Expect(previousImage2Result.Error).ToNot(HaveOccurred())
				Expect(previousImage3Result.Error).ToNot(HaveOccurred())
				// simulate an error build in order to check the image values only
				expectedErr := fmt.Errorf("failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage2.Commit.RepoID).Return(previousImage2.Commit.Repo, nil)

				// the previous successful image is previousImage2
				actualErr := service.UpdateImage(ctx, image, &previousImage3)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.OSTreeRef).To(Equal(config.DistributionsRefs[dist85]))
				Expect(image.Commit.OSTreeParentRef).To(Equal(config.DistributionsRefs[dist84]))
				Expect(image.Commit.OSTreeParentCommit).To(Equal(previousImage2.Commit.Repo.URL))
			})
		})
		Context("updating image major version when all previous images failed", func() {
			orgID := faker.UUIDHyphenated()
			imageName := faker.Name()
			imageSet := models.ImageSet{Name: imageName, OrgID: orgID}
			dist84 := "rhel-84"
			dist90 := "rhel-90"
			imageSetResult := db.DB.Create(&imageSet)
			previousImage1 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      1,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage1Result := db.DB.Create(&previousImage1)
			previousImage2 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      2,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage2Result := db.DB.Create(&previousImage2)
			image := &models.Image{
				OrgID:        orgID,
				Distribution: dist90,
				Commit:       &models.Commit{OrgID: orgID},
				OutputTypes:  []string{models.ImageTypeCommit},
				Name:         imageName,
			}

			It("new image should have no parent repo url and parent ostree ref", func() {
				Expect(imageSetResult.Error).ToNot(HaveOccurred())
				Expect(previousImage1Result.Error).ToNot(HaveOccurred())
				Expect(previousImage2Result.Error).ToNot(HaveOccurred())

				// simulate an error build in order to check the image values only
				expectedErr := fmt.Errorf("failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)

				actualErr := service.UpdateImage(ctx, image, &previousImage2)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.OSTreeRef).To(Equal(config.DistributionsRefs[dist90]))
				Expect(image.Commit.OSTreeParentRef).To(BeEmpty())
				Expect(image.Commit.OSTreeParentCommit).To(BeEmpty())
			})
		})
		Context("updating image major version when one of previous images succeed", func() {
			orgID := faker.UUIDHyphenated()
			imageName := faker.Name()
			imageSet := models.ImageSet{Name: imageName, OrgID: orgID}
			dist84 := "rhel-84"
			dist85 := "rhel-85"
			dist86 := "rhel-86"
			dist90 := "rhel-90"
			imageSetResult := db.DB.Create(&imageSet)
			previousImage1 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      1,
				Distribution: dist84,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage1Result := db.DB.Create(&previousImage1)
			previousImage2 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusSuccess,
				Commit:       &models.Commit{Repo: &models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess}, OrgID: orgID},
				Version:      2,
				Distribution: dist85,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage2Result := db.DB.Create(&previousImage2)
			previousImage3 := models.Image{
				OrgID:        orgID,
				Status:       models.ImageStatusError,
				Commit:       &models.Commit{OrgID: orgID},
				Version:      3,
				Distribution: dist86,
				Name:         imageName,
				ImageSetID:   &imageSet.ID,
			}
			previousImage3Result := db.DB.Create(&previousImage3)
			image := &models.Image{
				OrgID:        orgID,
				Distribution: dist90,
				Commit:       &models.Commit{OrgID: orgID},
				OutputTypes:  []string{models.ImageTypeCommit},
				Name:         imageName,
			}

			It("new image should have no parent repo url and parent ostree ref", func() {
				Expect(imageSetResult.Error).ToNot(HaveOccurred())
				Expect(previousImage1Result.Error).ToNot(HaveOccurred())
				Expect(previousImage2Result.Error).ToNot(HaveOccurred())
				Expect(previousImage3Result.Error).ToNot(HaveOccurred())
				// simulate an error build in order to check the image values only
				expectedErr := fmt.Errorf("failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage2.Commit.RepoID).Return(previousImage2.Commit.Repo, nil)

				// the previous successful image is previousImage2
				actualErr := service.UpdateImage(ctx, image, &previousImage3)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.OSTreeRef).To(Equal(config.DistributionsRefs[dist90]))
				Expect(image.Commit.OSTreeParentRef).To(Equal(config.DistributionsRefs[dist85]))
				Expect(image.Commit.OSTreeParentCommit).To(Equal(previousImage2.Commit.Repo.URL))
			})
		})

		Context("when previous image has success status", func() {
			It("should have the parent image repo url set as parent commit url", func() {
				id, _ := faker.RandomInt(1)
				uid := uint(id[0])
				orgID := faker.UUIDHyphenated()
				imageSet := &models.ImageSet{OrgID: orgID}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					OrgID:        orgID,
					Status:       models.ImageStatusSuccess,
					Commit:       &models.Commit{RepoID: &uid, OrgID: orgID},
					Version:      1,
					Distribution: "rhel-85",
					Name:         faker.Name(),
					ImageSetID:   &imageSet.ID,
				}
				image := &models.Image{
					OrgID:        orgID,
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      2,
					Distribution: "rhel-85",
					Name:         previousImage.Name,
				}
				result = db.DB.Save(previousImage)
				Expect(result.Error).To(Not(HaveOccurred()))

				parentRepo := &models.Repo{URL: faker.URL()}
				expectedErr := fmt.Errorf("Failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(parentRepo, nil)
				actualErr := service.UpdateImage(ctx, image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.ChangesRefs).To(BeFalse())
				Expect(image.Commit.OSTreeParentCommit).To(Equal(parentRepo.URL))
				Expect(image.Commit.OSTreeParentRef).To(Equal("rhel/8/x86_64/edge"))
				Expect(image.Commit.OSTreeRef).To(Equal("rhel/8/x86_64/edge"))
			})

			When("updating major version, from 8.6 to 9.0", func() {
				orgID := faker.UUIDHyphenated()
				imageSet := &models.ImageSet{OrgID: orgID}
				dist := "rhel-86"
				newDist := "rhel-90"
				imageSetResult := db.DB.Create(imageSet)
				repo := models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess}
				repoResult := db.DB.Create(&repo)
				previousImage := &models.Image{
					OrgID:  orgID,
					Status: models.ImageStatusSuccess,
					Commit: &models.Commit{
						Repo:  &models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess},
						OrgID: orgID,
					},
					Version:      1,
					Distribution: dist,
					Name:         faker.Name(),
					ImageSetID:   &imageSet.ID,
				}
				previousImageResult := db.DB.Create(previousImage)
				image := &models.Image{
					OrgID:        orgID,
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      2,
					Distribution: newDist,
					Name:         previousImage.Name,
				}
				It("should have parent ref and url defined when updating major version", func() {
					Expect(repoResult.Error).ToNot(HaveOccurred())
					Expect(imageSetResult.Error).ToNot(HaveOccurred())
					Expect(previousImageResult.Error).ToNot(HaveOccurred())
					Expect(dist).NotTo(Equal(newDist))
					osTreeParentRef := config.DistributionsRefs[dist]
					osTreeRef := config.DistributionsRefs[newDist]
					Expect(osTreeRef).ToNot(Equal(osTreeParentRef))

					// simulate error building image to analyse the image values only
					expectedErr := fmt.Errorf("Failed creating commit for image")
					mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
					mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(&repo, nil)
					actualErr := service.UpdateImage(ctx, image, previousImage)
					Expect(actualErr).To(HaveOccurred())
					Expect(actualErr).To(MatchError(expectedErr))

					Expect(image.Commit.OSTreeParentCommit).To(Equal(repo.URL))
					Expect(image.Commit.OSTreeRef).To(Equal(osTreeRef))
					Expect(image.Commit.OSTreeParentRef).To(Equal(osTreeParentRef))
					Expect(image.Commit.ChangesRefs).To(BeTrue())
				})

				When("previous image commit has osTree Refs defined", func() {
					previousImage.Commit.OSTreeRef = config.DistributionsRefs[dist]
					previousImage.Commit.OSTreeParentRef = config.DistributionsRefs[dist]
					result := db.DB.Save(&previousImage.Commit)

					It("should have parent ref and url defined when updating major version", func() {
						Expect(result.Error).ToNot(HaveOccurred())
						Expect(dist).NotTo(Equal(newDist))
						osTreeParentRef := config.DistributionsRefs[dist]
						osTreeRef := config.DistributionsRefs[newDist]
						Expect(osTreeRef).ToNot(Equal(osTreeParentRef))

						// simulate error building image to analyse the image values only
						expectedErr := fmt.Errorf("Failed creating commit for image")
						mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
						mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(&repo, nil)
						actualErr := service.UpdateImage(ctx, image, previousImage)
						Expect(actualErr).To(HaveOccurred())
						Expect(actualErr).To(MatchError(expectedErr))

						Expect(image.Commit.OSTreeParentCommit).To(Equal(repo.URL))
						Expect(image.Commit.OSTreeRef).To(Equal(osTreeRef))
						Expect(image.Commit.OSTreeParentRef).To(Equal(osTreeParentRef))
						Expect(image.Commit.ChangesRefs).To(BeTrue())
					})
				})
			})
			When("not updating major version, from 8.5 to 8.6", func() {
				orgID := faker.UUIDHyphenated()
				imageSet := &models.ImageSet{OrgID: orgID}
				dist := "rhel-85"
				newDist := "rhel-86"
				imageSetResult := db.DB.Create(imageSet)
				repo := models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess}
				repoResult := db.DB.Create(&repo)
				previousImage := &models.Image{
					OrgID:  orgID,
					Status: models.ImageStatusSuccess,
					Commit: &models.Commit{
						Repo:  &models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess},
						OrgID: orgID,
					},
					Version:      1,
					Distribution: dist,
					Name:         faker.Name(),
					ImageSetID:   &imageSet.ID,
				}
				previousImageResult := db.DB.Create(previousImage)
				image := &models.Image{
					OrgID:        orgID,
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      2,
					Distribution: newDist,
					Name:         previousImage.Name,
				}
				It("should have parent ref and ref to be equal and url defined when not updating major version", func() {
					Expect(repoResult.Error).ToNot(HaveOccurred())
					Expect(imageSetResult.Error).ToNot(HaveOccurred())
					Expect(previousImageResult.Error).ToNot(HaveOccurred())
					Expect(dist).NotTo(Equal(newDist))
					osTreeParentRef := config.DistributionsRefs[dist]
					osTreeRef := config.DistributionsRefs[newDist]
					Expect(osTreeRef).To(Equal(osTreeParentRef))

					// simulate error building image to analyse the image values only
					expectedErr := fmt.Errorf("Failed creating commit for image")
					mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
					mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(&repo, nil)
					actualErr := service.UpdateImage(ctx, image, previousImage)
					Expect(actualErr).To(HaveOccurred())
					Expect(actualErr).To(MatchError(expectedErr))

					Expect(image.Commit.OSTreeParentCommit).To(Equal(repo.URL))
					Expect(image.Commit.OSTreeRef).To(Equal(osTreeRef))
					Expect(image.Commit.OSTreeParentRef).To(Equal(osTreeParentRef))
					Expect(image.Commit.OSTreeRef).To(Equal(image.Commit.OSTreeParentRef))
					Expect(image.Commit.ChangesRefs).To(BeFalse())
				})

			})

		})

		Context("when previous image has success status", func() {
			It("should have the parent image repo url set as parent commit url", func() {
				orgID := faker.UUIDHyphenated()
				id, _ := faker.RandomInt(1)
				uid := uint(id[0])
				imageSet := &models.ImageSet{OrgID: orgID}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					OrgID:        orgID,
					Status:       models.ImageStatusSuccess,
					Commit:       &models.Commit{RepoID: &uid, OrgID: orgID},
					Version:      1,
					Distribution: "rhel-86",
					Name:         faker.Name(),
					ImageSetID:   &imageSet.ID,
				}
				image := &models.Image{
					OrgID:        orgID,
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      2,
					Distribution: "rhel-90",
					Name:         previousImage.Name,
				}
				result = db.DB.Save(previousImage)
				Expect(result.Error).To(Not(HaveOccurred()))

				parentRepo := &models.Repo{URL: faker.URL()}
				expectedErr := fmt.Errorf("Failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)
				mockRepoService.EXPECT().GetRepoByID(previousImage.Commit.RepoID).Return(parentRepo, nil)
				actualErr := service.UpdateImage(ctx, image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.ChangesRefs).To(BeTrue())
				Expect(image.Commit.OSTreeParentCommit).To(Equal(parentRepo.URL))
				Expect(image.Commit.OSTreeParentRef).To(Equal("rhel/8/x86_64/edge"))
				Expect(image.Commit.OSTreeRef).To(Equal("rhel/9/x86_64/edge"))
			})
		})
		Context("edge-management.storage_images_repos feature", func() {
			BeforeEach(func() {
				// enable the feature
				err := os.Setenv("STORAGE_IMAGES_REPOS", "True")
				Expect(err).ToNot(HaveOccurred())
			})
			AfterEach(func() {
				// disable the feature
				os.Unsetenv("STORAGE_IMAGES_REPOS")
			})
			It("should have the parent image repo url set to edge api cert storage images-repos url", func() {
				orgID := faker.UUIDHyphenated()
				id, _ := faker.RandomInt(1)
				uid := uint(id[0])
				imageSet := &models.ImageSet{OrgID: orgID}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					OrgID:        orgID,
					Status:       models.ImageStatusSuccess,
					Commit:       &models.Commit{RepoID: &uid, OrgID: orgID},
					Version:      1,
					Distribution: "rhel-86",
					Name:         faker.Name(),
					ImageSetID:   &imageSet.ID,
				}
				image := &models.Image{
					OrgID:        orgID,
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      2,
					Distribution: "rhel-90",
					Name:         previousImage.Name,
				}
				result = db.DB.Save(previousImage)
				Expect(result.Error).To(Not(HaveOccurred()))

				expectedURL := fmt.Sprintf("http://cert.localhost:3000/api/edge/v1/storage/images-repos/%d", previousImage.ID)
				expectedErr := fmt.Errorf("Failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(image).Return(image, expectedErr)

				actualErr := service.UpdateImage(ctx, image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.ChangesRefs).To(BeTrue())
				Expect(image.Commit.OSTreeParentCommit).To(Equal(expectedURL))
			})
		})

		Context("image update when changing image name", func() {
			var imageSet models.ImageSet
			var image models.Image
			var orgID string
			var imageName string
			BeforeEach(func() {
				orgID = common.DefaultOrgID
				imageName = faker.Name()
				imageSet = models.ImageSet{OrgID: orgID, Name: imageName}
				err := db.DB.Create(&imageSet).Error
				Expect(err).ToNot(HaveOccurred())
				image = models.Image{
					OrgID:        orgID,
					ImageSetID:   &imageSet.ID,
					Name:         imageName,
					Commit:       &models.Commit{OrgID: orgID},
					OutputTypes:  []string{models.ImageTypeCommit},
					Version:      1,
					Distribution: "rhel-90",
				}
				err = db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("when not supplying a name the old previous name should be preserved", func() {
				updateImage := models.Image{
					OrgID:        orgID,
					ImageSetID:   &imageSet.ID,
					Name:         "",
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Distribution: "rhel-90",
				}

				// simulate image-builder error, as not important in the current scenario
				expectedErr := fmt.Errorf("failed creating commit for image")
				mockImageBuilderClient.EXPECT().ComposeCommit(&updateImage).Return(&updateImage, expectedErr)

				actualErr := service.UpdateImage(ctx, &updateImage, &image)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(updateImage.Name).To(Equal(image.Name))
			})

			It("should return error when trying to change the name", func() {
				updateImage := models.Image{
					OrgID:        orgID,
					ImageSetID:   &imageSet.ID,
					Name:         faker.Name(),
					Commit:       &models.Commit{},
					OutputTypes:  []string{models.ImageTypeCommit},
					Distribution: "rhel-90",
				}
				Expect(updateImage.Name).ToNot(Equal(image.Name))

				expectedError := new(services.ImageNameChangeIsProhibited)

				actualErr := service.UpdateImage(ctx, &updateImage, &image)
				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedError))
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
				orgID := faker.UUIDHyphenated()
				image := &models.Image{
					Installer: &models.Installer{
						Status: models.ImageStatusError,
						OrgID:  orgID,
					},
					Commit: &models.Commit{
						Status: models.ImageStatusError,
						OrgID:  orgID,
					},
					Status:      models.ImageStatusError,
					OutputTypes: []string{models.ImageTypeInstaller, models.ImageTypeCommit},
					OrgID:       orgID,
				}
				err := service.SetBuildingStatusOnImageToRetryBuild(image)

				Expect(err).ToNot(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusBuilding))
				Expect(image.Commit.Status).To(Equal(models.ImageStatusBuilding))
				Expect(image.Installer.Status).To(Equal(models.ImageStatusCreated))
			})
		})
		Context("when checking if the image version we are trying to update is latest", func() {
			orgID1 := faker.UUIDHyphenated()
			imageSet := models.ImageSet{OrgID: orgID1}
			db.DB.Save(&imageSet)
			image := models.Image{OrgID: orgID1, ImageSetID: &imageSet.ID, Version: 1, Name: "image-same-name"}
			db.DB.Save(&image)
			image2 := models.Image{OrgID: orgID1, ImageSetID: &imageSet.ID, Version: 2, Name: "image-same-name"}
			db.DB.Save(&image2)
			image3 := models.Image{OrgID: orgID1, ImageSetID: &imageSet.ID, Version: 3, Name: "image-same-name"}
			db.DB.Save(&image3)
			// foreign image without org_id
			image4 := models.Image{ImageSetID: &imageSet.ID, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image4)
			// foreign image without image-set
			image5 := models.Image{OrgID: orgID1, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image5)

			// foreign image from another org_id and image-set, is here to ensure we are analysing the correct collection
			orgID2 := faker.UUIDHyphenated()
			orgID2ImageSet := models.ImageSet{OrgID: orgID1}
			db.DB.Save(&orgID2ImageSet)
			image6 := models.Image{OrgID: orgID2, ImageSetID: &orgID2ImageSet.ID, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image6)
			// foreign image from another image-set, is here to ensure we are analysing the correct collection
			imageSet2 := models.ImageSet{OrgID: orgID2}
			db.DB.Save(&imageSet2)
			image7 := models.Image{OrgID: orgID2, ImageSetID: &imageSet2.ID, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image7)

			It("the image has to be defined", func() {
				err := service.CheckIfIsLatestVersion(&models.Image{OrgID: orgID1, ImageSetID: &imageSet.ID, Version: 5, Name: "image-same-name"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageUnDefined)))
			})

			It("the image orgID must be defined", func() {

				err := service.CheckIfIsLatestVersion(&image4)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.OrgIDNotSet)))
			})

			It("the image image-set must be be defined", func() {
				err := service.CheckIfIsLatestVersion(&image5)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageSetUnDefined)))
			})

			It("other latest version already exists", func() {
				err := service.CheckIfIsLatestVersion(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageVersionAlreadyExists)))
			})

			It("nearest latest version already exists", func() {
				err := service.CheckIfIsLatestVersion(&image2)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageVersionAlreadyExists)))
			})

			It("Latest version is checked without errors", func() {
				err := service.CheckIfIsLatestVersion(&image3)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
	Describe("validate images packages orgID", func() {
		Context("when creating an image using third party repository", func() {
			It("should validate the images with empty repos", func() {
				var repos []models.ThirdPartyRepo
				orgID := "11111"
				_, err := services.GetImageReposFromDB(orgID, repos)
				Expect(err).ToNot(HaveOccurred())

			})
			It("should raise ThirdPartyRepositoryNotFound error", func() {
				orgID := faker.UUIDHyphenated()
				repos := []models.ThirdPartyRepo{
					{OrgID: orgID, Name: faker.UUIDHyphenated(), URL: "https://repo1.simple.com"},
				}
				_, err := services.GetImageReposFromDB(orgID, repos)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ThirdPartyRepositoryNotFound).Error()))
			})
			It("should raise OrgIDNotSet error", func() {
				var repos []models.ThirdPartyRepo
				orgID := ""
				_, err := services.GetImageReposFromDB(orgID, repos)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.OrgIDNotSet).Error()))
			})
			It("should validate the images with repos within same org_id", func() {
				orgID := "00000"
				repo1 := models.ThirdPartyRepo{OrgID: orgID, Name: faker.UUIDHyphenated(), URL: "https://repo1.simple.com"}
				result := db.DB.Create(&repo1)
				Expect(result.Error).ToNot(HaveOccurred())
				repo2 := models.ThirdPartyRepo{OrgID: orgID, Name: faker.UUIDHyphenated(), URL: "https://repo2.simple.com"}
				result = db.DB.Create(&repo2)
				Expect(result.Error).ToNot(HaveOccurred())
				imagesRepos, err := services.GetImageReposFromDB(orgID, []models.ThirdPartyRepo{{Model: models.Model{ID: repo1.ID}}, {Model: models.Model{ID: repo2.ID}}})
				Expect(len(*imagesRepos)).To(Equal(2))
				Expect(err).ToNot(HaveOccurred())

			})
			It("should not validate the images with repos from different org_id", func() {
				orgID1 := "1111111"
				orgID2 := "2222222"
				repo1 := models.ThirdPartyRepo{OrgID: orgID1, Name: faker.UUIDHyphenated(), URL: "https://repo1.simple.com"}
				result := db.DB.Create(&repo1)
				Expect(result.Error).ToNot(HaveOccurred())
				repo2 := models.ThirdPartyRepo{OrgID: orgID2, Name: faker.UUIDHyphenated(), URL: "https://repo2.simple.com"}
				result = db.DB.Create(&repo2)
				Expect(result.Error).ToNot(HaveOccurred())
				_, err := services.GetImageReposFromDB(orgID1, []models.ThirdPartyRepo{repo1, repo2})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ThirdPartyRepositoryNotFound).Error()))
			})
		})
	})

	Describe("content-sources repositories", func() {
		var orgID string
		var image models.Image
		// edge management third-pary repos
		var emRepos []models.ThirdPartyRepo
		// content-sources custom repos
		var csRepos []repositories.Repository
		var existingEMRepo models.ThirdPartyRepo
		var otherEMRepo models.ThirdPartyRepo
		var existingEMRepo2 models.ThirdPartyRepo

		BeforeEach(func() {
			orgID = faker.UUIDHyphenated()
			// set content-sources feature enabled
			err := os.Setenv("FEATURE_CONTENT_SOURCES", "true")
			Expect(err).ToNot(HaveOccurred())
			existingEMRepo = models.ThirdPartyRepo{
				OrgID: orgID, Name: faker.UUIDHyphenated(), UUID: faker.UUIDHyphenated(), URL: faker.URL(),
			}
			err = db.DB.Create(&existingEMRepo).Error
			Expect(err).ToNot(HaveOccurred())
			// other emRepo does not exist in db, but may exist in content-sources repositories
			otherEMRepo = models.ThirdPartyRepo{OrgID: orgID, Name: faker.UUIDHyphenated(), UUID: faker.UUIDHyphenated(), URL: faker.URL()}
			// other emRepo2 does exist in db, but exist in content-sources repositories with other uuid
			existingEMRepo2 = models.ThirdPartyRepo{OrgID: orgID, Name: faker.UUIDHyphenated(), UUID: faker.UUIDHyphenated(), URL: faker.URL()}
			err = db.DB.Create(&existingEMRepo2).Error
			Expect(err).ToNot(HaveOccurred())

			emRepos = []models.ThirdPartyRepo{
				existingEMRepo,
				otherEMRepo,
				existingEMRepo2,
			}
			csRepos = []repositories.Repository{
				// add an existing one, but change some data
				{
					UUID: uuid.MustParse(existingEMRepo.UUID), Name: faker.UUIDHyphenated(), URL: existingEMRepo.URL,
					DistributionArch: faker.UUIDHyphenated(), GpgKey: faker.UUIDHyphenated(), PackageCount: 200,
				},
				// add the other that do not exist in db
				{
					UUID: uuid.MustParse(otherEMRepo.UUID), Name: otherEMRepo.Name, URL: models.AddSlashToURL(faker.URL()), DistributionArch: faker.UUIDHyphenated(),
					GpgKey: faker.UUIDHyphenated(), PackageCount: 160,
				},
				// add the existing one that do exist in db with other uuid, this repos maybe deleted and created again with same url at content-sources
				{
					UUID: uuid.New(), Name: faker.Name(), URL: existingEMRepo2.URL, DistributionArch: faker.UUIDHyphenated(),
					GpgKey: faker.UUIDHyphenated(), PackageCount: 180,
				},
			}

			image = models.Image{
				OrgID:                  orgID,
				Name:                   faker.UUIDHyphenated(),
				Commit:                 &models.Commit{OrgID: orgID},
				ImageType:              models.ImageTypeCommit,
				ThirdPartyRepositories: emRepos,
			}
		})

		AfterEach(func() {
			// disable content-sources feature enabled
			err := os.Unsetenv("FEATURE_CONTENT_SOURCES")
			Expect(err).ToNot(HaveOccurred())
		})

		Context("#SetImageContentSourcesRepositories", func() {
			It("should set the image third-party repos successfully", func() {
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[0].URL).Return(&csRepos[0], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[1].URL).Return(&csRepos[1], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[2].URL).Return(&csRepos[2], nil)

				err := service.SetImageContentSourcesRepositories(&image)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(image.ThirdPartyRepositories)).To(Equal(3))
				// The existing repo has not changed id, URL and UUID
				Expect(image.ThirdPartyRepositories[0].ID).To(Equal(existingEMRepo.ID))
				Expect(image.ThirdPartyRepositories[0].UUID).To(Equal(existingEMRepo.UUID))
				Expect(image.ThirdPartyRepositories[0].URL).To(Equal(existingEMRepo.URL))
				// the other repo has been saved
				Expect(image.ThirdPartyRepositories[1].ID > 0).To(BeTrue())

				// check that the existing repos has been updated and the newly created has the expected data
				for ind := range []int{0, 1, 2} {
					Expect(image.ThirdPartyRepositories[ind].Name).To(Equal(csRepos[ind].Name))
					Expect(image.ThirdPartyRepositories[ind].UUID).To(Equal(csRepos[ind].UUID.String()))
					Expect(image.ThirdPartyRepositories[ind].URL).To(Equal(csRepos[ind].URL))
					Expect(image.ThirdPartyRepositories[ind].DistributionArch).To(Equal(csRepos[ind].DistributionArch))
					Expect(image.ThirdPartyRepositories[ind].GpgKey).To(Equal(csRepos[ind].GpgKey))
					Expect(image.ThirdPartyRepositories[ind].PackageCount).To(Equal(csRepos[ind].PackageCount))
				}

				// ensure that the initial existing records in images are the same as the resulting one.
				existingRecords := []int{0, 2}
				for ind, id := range []uint{existingEMRepo.ID, existingEMRepo2.ID} {
					Expect(image.ThirdPartyRepositories[existingRecords[ind]].ID).To(Equal(id))
				}
			})

			It("should do nothing when image has no third-party repos", func() {
				image := models.Image{OrgID: orgID, Name: faker.Name(), ThirdPartyRepositories: []models.ThirdPartyRepo{}}
				err := service.SetImageContentSourcesRepositories(&image)
				Expect(err).ToNot(HaveOccurred())
				Expect(len(image.ThirdPartyRepositories)).To(Equal(0))
			})

			It("should return error when image is nil", func() {
				err := service.SetImageContentSourcesRepositories(nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(services.ImageUnDefinedMsg))
			})
			It("should return error when image org is not defined", func() {
				err := service.SetImageContentSourcesRepositories(
					&models.Image{OrgID: "", Name: faker.Name(), ThirdPartyRepositories: []models.ThirdPartyRepo{existingEMRepo}},
				)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(services.OrgIDNotSetMsg))
			})

			It("should return error when repository.GetRepositoryByUUID fails with unknown error", func() {
				image := models.Image{OrgID: orgID, Name: faker.Name(), ThirdPartyRepositories: []models.ThirdPartyRepo{otherEMRepo}}
				expectedError := errors.New("expected unknown error returned when calling GetRepositoryByUUID")
				mockRepositories.EXPECT().GetRepositoryByURL(image.ThirdPartyRepositories[0].URL).Return(nil, expectedError)

				err := service.SetImageContentSourcesRepositories(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})

			It("should ignore repository if it does not exist in content-sources", func() {
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[0].URL).Return(&csRepos[0], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[1].URL).Return(&csRepos[1], repositories.ErrRepositoryNotFound)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[2].URL).Return(&csRepos[2], nil)

				err := service.SetImageContentSourcesRepositories(&image)
				Expect(err).ToNot(HaveOccurred())
				// the non-existent content-sources repository was ignored
				Expect(len(image.ThirdPartyRepositories)).To(Equal(2))
				imageRepo := image.ThirdPartyRepositories[0]
				// The existing repo has not changed id , url  and UUID
				Expect(imageRepo.ID).To(Equal(existingEMRepo.ID))
				Expect(imageRepo.UUID).To(Equal(existingEMRepo.UUID))
				Expect(imageRepo.URL).To(Equal(existingEMRepo.URL))

				// check that the existing repos has been updated has the expected data
				for ind, csRepoInd := range []int{0, 2} {
					Expect(image.ThirdPartyRepositories[ind].Name).To(Equal(csRepos[csRepoInd].Name))
					Expect(image.ThirdPartyRepositories[ind].UUID).To(Equal(csRepos[csRepoInd].UUID.String()))
					Expect(image.ThirdPartyRepositories[ind].URL).To(Equal(csRepos[csRepoInd].URL))
					Expect(image.ThirdPartyRepositories[ind].DistributionArch).To(Equal(csRepos[csRepoInd].DistributionArch))
					Expect(image.ThirdPartyRepositories[ind].GpgKey).To(Equal(csRepos[csRepoInd].GpgKey))
					Expect(image.ThirdPartyRepositories[ind].PackageCount).To(Equal(csRepos[csRepoInd].PackageCount))
				}
			})
		})

		Context("CreateImage", func() {

			It("CreateImage should call SetImageContentSourcesRepositories successfully", func() {
				expectedError := errors.New("simulate compose commit error as the call should happen before calling ComposeCommit")
				mockImageBuilderClient.EXPECT().ComposeCommit(&image).Return(&image, expectedError)

				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[0].URL).Return(&csRepos[0], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[1].URL).Return(&csRepos[1], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[2].URL).Return(&csRepos[2], nil)

				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
				Expect(len(image.ThirdPartyRepositories)).To(Equal(3))
				// check that the existing repo has been updated and the newly created has the expected data
				for ind := range []int{0, 1, 2} {
					Expect(image.ThirdPartyRepositories[ind].Name).To(Equal(csRepos[ind].Name))
					Expect(image.ThirdPartyRepositories[ind].UUID).To(Equal(csRepos[ind].UUID.String()))
					Expect(image.ThirdPartyRepositories[ind].URL).To(Equal(csRepos[ind].URL))
					Expect(image.ThirdPartyRepositories[ind].DistributionArch).To(Equal(csRepos[ind].DistributionArch))
					Expect(image.ThirdPartyRepositories[ind].GpgKey).To(Equal(csRepos[ind].GpgKey))
					Expect(image.ThirdPartyRepositories[ind].PackageCount).To(Equal(csRepos[ind].PackageCount))
				}
			})

			It("CreateImage should return error when SetImageContentSourcesRepositories fails with unknown error", func() {
				expectedError := errors.New("expected unknown GetRepositoryByUUID error")
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[0].URL).Return(nil, expectedError)

				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})
		})

		Context("UpdateImage", func() {
			var previousImage models.Image
			BeforeEach(func() {
				imageSet := models.ImageSet{OrgID: orgID, Name: image.Name}
				err := db.DB.Create(&imageSet).Error
				Expect(err).ToNot(HaveOccurred())
				previousImage = models.Image{
					OrgID:      orgID,
					Name:       image.Name,
					ImageSetID: &imageSet.ID,
					Commit: &models.Commit{OrgID: orgID,
						Status:           models.ImageStatusSuccess,
						ImageBuildTarURL: faker.URL(),
						OSTreeCommit:     faker.UUIDHyphenated(),
						OSTreeRef:        config.DistributionsRefs["rhel-91"],
						Repo:             &models.Repo{URL: faker.URL(), Status: models.RepoStatusSuccess},
					},
				}
				err = db.DB.Create(&previousImage).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should call SetImageContentSourcesRepositories successfully", func() {
				expectedError := errors.New("simulate compose commit error as the call should happen before calling ComposeCommit")
				mockImageBuilderClient.EXPECT().ComposeCommit(&image).Return(&image, expectedError)

				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[0].URL).Return(&csRepos[0], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[1].URL).Return(&csRepos[1], nil)
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[2].URL).Return(&csRepos[2], nil)

				err := service.UpdateImage(ctx, &image, &previousImage)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
				Expect(len(image.ThirdPartyRepositories)).To(Equal(3))
				// check that the existing repo has been updated and the newly created has the expected data
				for ind := range []int{0, 1, 2} {
					Expect(image.ThirdPartyRepositories[ind].Name).To(Equal(csRepos[ind].Name))
					Expect(image.ThirdPartyRepositories[ind].UUID).To(Equal(csRepos[ind].UUID.String()))
					Expect(image.ThirdPartyRepositories[ind].URL).To(Equal(csRepos[ind].URL))
					Expect(image.ThirdPartyRepositories[ind].DistributionArch).To(Equal(csRepos[ind].DistributionArch))
					Expect(image.ThirdPartyRepositories[ind].GpgKey).To(Equal(csRepos[ind].GpgKey))
					Expect(image.ThirdPartyRepositories[ind].PackageCount).To(Equal(csRepos[ind].PackageCount))
				}
			})

			It("should return error when SetImageContentSourcesRepositories fails with unknown error", func() {
				expectedError := errors.New("expected unknown GetRepositoryByUUID error")
				mockRepositories.EXPECT().GetRepositoryByURL(emRepos[0].URL).Return(nil, expectedError)

				err := service.UpdateImage(ctx, &image, &previousImage)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})
		})
	})

	Describe("send image starts notification", func() {
		Context("when creating an image we should send a notification to topic", func() {
			It("validate content", func() {
				var image *models.Image
				var err error
				imageSet := &models.ImageSet{
					Name:    "test",
					Version: 1,
					OrgID:   common.DefaultOrgID,
				}
				db.DB.Create(imageSet)

				image = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        common.DefaultOrgID,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      common.DefaultOrgID,
				}
				db.DB.Create(image)
				image, err = service.GetImageByID(fmt.Sprint(image.ID))
				Expect(err).ToNot(HaveOccurred())

				notify, err := service.SendImageNotification(image)
				Expect(err).ToNot(HaveOccurred())
				Expect(notify.Version).To(Equal("v1.1.0"))
				Expect(notify.EventType).To(Equal("image-creation"))

			})

		})

		Context("clowder enabled", func() {
			const ClowderEnvConfigName = "ACG_CONFIG"
			var imageSet models.ImageSet
			var image models.Image
			var orgID string
			BeforeEach(func() {
				orgID = faker.UUIDHyphenated()
				err := os.Setenv(ClowderEnvConfigName, "any value here will set the clowder as enabled")
				Expect(err).ToNot(HaveOccurred())
				imageSet = models.ImageSet{
					Name:    faker.UUIDHyphenated(),
					Version: 1,
					OrgID:   orgID,
				}
				err = db.DB.Create(&imageSet).Error
				Expect(err).ToNot(HaveOccurred())

				image = models.Image{
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      orgID,
				}
				err = db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				err := os.Unsetenv(ClowderEnvConfigName)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should produce a notification message for image", func() {
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), nil).Return(nil)
				notify, err := service.SendImageNotification(&image)
				Expect(err).ToNot(HaveOccurred())
				Expect(notify.EventType).To(Equal("image-creation"))
				Expect(notify.OrgID).To(Equal(orgID))
			})
			It("should return error when produce fail", func() {
				expectedError := errors.New("producer produce expected error")
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return(services.NotificationTopic, nil)
				mockProducer.EXPECT().Produce(gomock.Any(), nil).Return(expectedError)
				_, err := service.SendImageNotification(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})

			It("should return error when GetTopic fail", func() {
				expectedError := errors.New("topic-service GetTopic expected error")
				mockProducerService.EXPECT().GetProducerInstance().Return(mockProducer)
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Return("", expectedError)
				// produce function should not be called
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Times(0)
				_, err := service.SendImageNotification(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})

			It("should return error when producer is not defined", func() {
				expectedError := new(services.KafkaProducerInstanceUndefined)
				mockProducerService.EXPECT().GetProducerInstance().Return(nil)
				// GetTopic should not be called
				mockTopicService.EXPECT().GetTopic(services.NotificationTopic).Times(0)
				// produce function should not be called
				mockProducer.EXPECT().Produce(gomock.Any(), gomock.Any()).Times(0)
				_, err := service.SendImageNotification(&image)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})
	})
	Describe("Devices update availability from image set", func() {
		Context("should set device update availability", func() {
			orgID := faker.UUIDHyphenated()
			imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
			db.DB.Create(&imageSet)
			initialImages := []models.Image{
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, OrgID: orgID},
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, OrgID: orgID},
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, OrgID: orgID},
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, OrgID: orgID},
			}
			images := make([]models.Image, 0, len(initialImages))
			for _, image := range initialImages {
				db.DB.Create(&image)
				images = append(images, image)
				fmt.Println("IMG >>>>", image.ID)
			}

			devices := make([]models.Device, 0, len(images))
			for ind, image := range images {
				device := models.Device{OrgID: orgID, ImageID: image.ID, UpdateAvailable: false, UUID: faker.UUIDHyphenated()}
				if ind == len(images)-1 {
					device.UpdateAvailable = true
				}
				db.DB.Create(&device)
				devices = append(devices, device)
			}
			lastDevicesIndex := len(devices) - 1

			OtherImageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
			db.DB.Create(&OtherImageSet)

			otherImage := models.Image{Status: models.ImageStatusSuccess, ImageSetID: &OtherImageSet.ID, OrgID: orgID}
			db.DB.Create(&otherImage)
			OtherDevice := models.Device{OrgID: orgID, ImageID: otherImage.ID, UpdateAvailable: true}
			db.DB.Create(&OtherDevice)

			It("No error occurred without errors when calling function", func() {
				err := service.SetDevicesUpdateAvailabilityFromImageSet(orgID, imageSet.ID)
				Expect(err).To(BeNil())
			})

			It("All devices has UpdateAvailable updated as expected", func() {
				// reload devices fro db
				savedDevices := make([]models.Device, 0, len(devices))
				for _, device := range devices {
					var savedDevice models.Device
					db.DB.First(&savedDevice, device.ID)
					savedDevices = append(savedDevices, savedDevice)
				}
				for ind, device := range savedDevices {
					if ind == lastDevicesIndex {
						Expect(device.UpdateAvailable).To(Equal(false))
					} else {
						Expect(device.UpdateAvailable).To(Equal(true))
					}
				}
			})

			It("Other device not updated as having an other imageSet", func() {
				// reload other device
				var device models.Device
				result := db.DB.First(&device, OtherDevice.ID)
				Expect(result.Error).To(BeNil())
				Expect(OtherDevice.UpdateAvailable).To(Equal(true))
			})

			It("running function for Other imageSet update other device", func() {
				err := service.SetDevicesUpdateAvailabilityFromImageSet(orgID, OtherImageSet.ID)
				Expect(err).To(BeNil())
				// reload other device
				var device models.Device
				result := db.DB.First(&device, OtherDevice.ID)
				Expect(result.Error).To(BeNil())
				Expect(device.UpdateAvailable).To(Equal(false))
			})

			It("should run without errors when no devices", func() {
				imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
				result := db.DB.Create(&imageSet)
				Expect(result.Error).To(BeNil())
				image := models.Image{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, OrgID: orgID}
				result = db.DB.Create(&image)
				Expect(result.Error).To(BeNil())

				err := service.SetDevicesUpdateAvailabilityFromImageSet(orgID, imageSet.ID)
				Expect(err).To(BeNil())
			})
		})
	})
	Describe("update image stop when worker running job stopped responding", func() {
		orgID := faker.UUIDHyphenated()
		imageSet := models.ImageSet{OrgID: orgID, Name: faker.UUIDHyphenated()}
		result := db.DB.Create(&imageSet)
		Expect(result.Error).To(BeNil())
		commit := models.Commit{OrgID: orgID, Status: models.ImageStatusBuilding}
		db.DB.Create(&commit)
		image := models.Image{ImageSetID: &imageSet.ID, OrgID: orgID, CommitID: commit.ID, Commit: &commit}
		re := db.DB.Create(&image)
		Expect(re.Error).To(BeNil())
		expectedErr := fmt.Errorf("running this job stopped responding")
		When("When image-builder failed in GetComposeStatus when worker stopped responding", func() {
			It("image is created with INTERRUPTED status", func() {
				mockImageBuilderClient.EXPECT().GetCommitStatus(&image).Return(&image, expectedErr)
				_, err := service.UpdateImageStatus(&image)

				Expect(err).To(HaveOccurred())
				Expect(image.Status).To(Equal(models.ImageStatusInterrupted))
			})
		})
	})
	Describe("Create image when using getImageSetForNewImage", func() {
		orgID := common.DefaultOrgID
		// requestID := faker.UUIDHyphenated()
		imageName := faker.UUIDHyphenated()
		image := models.Image{OrgID: orgID, Distribution: "rhel-85", Name: imageName}
		expectedErr := fmt.Errorf("failed to create commit for image")
		When("When image-builder ComposeCommit fail", func() {
			It("imageSet is created", func() {
				mockImageBuilderClient.EXPECT().ComposeCommit(&image).Return(&image, expectedErr)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				// But imageSet is Created
				Expect(image.ImageSetID).ToNot(BeNil())
				Expect(*image.ImageSetID > 0).To(BeTrue())
			})

			It("imageSet is reused when no images linked", func() {
				// ensure image imageSetID is not nil and store it
				Expect(image.ImageSetID).ToNot(BeNil())
				imageSetID := *image.ImageSetID
				// set image imageSet to nil
				image.ImageSetID = nil
				mockImageBuilderClient.EXPECT().ComposeCommit(&image).Return(&image, expectedErr)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				// But imageSet is reused
				Expect(image.ImageSetID).ToNot(BeNil())
				Expect(*image.ImageSetID).To(Equal(imageSetID))
			})

			It("imageSet is not reused if images already linked to imageSet", func() {
				// ensure image imageSetID is not nil and store it
				Expect(image.ImageSetID).ToNot(BeNil())
				imageSetID := *image.ImageSetID
				// set image imageSet to nil
				image.ImageSetID = nil
				// create a new image linked with the known imageSet
				result := db.DB.Create(&models.Image{OrgID: orgID, Distribution: "rhel-85", Name: faker.UUIDHyphenated(), ImageSetID: &imageSetID})
				Expect(result.Error).ToNot(HaveOccurred())
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("image set already exists"))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				Expect(image.ImageSetID).To(BeNil())
			})
		})
	})
	Describe("Test ValidateImagePackage function", func() {
		When("When there's no valid arch", func() {
			It("should raise NewBadRequest", func() {
				package_name := faker.UUIDHyphenated()
				image := models.Image{
					Commit: &models.Commit{},
				}
				error := service.ValidateImagePackage(package_name, &image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(BeAssignableToTypeOf(new(apiErrors.BadRequest)))
			})
		})
		When("When there's no valid distribution", func() {
			It("should raise NewBadRequest", func() {
				package_name := faker.UUIDHyphenated()
				image := models.Image{
					Commit: &models.Commit{Arch: "x86_64"},
				}
				error := service.ValidateImagePackage(package_name, &image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(BeAssignableToTypeOf(new(apiErrors.BadRequest)))
			})
		})
		When("When SearchPackage fails to find a package", func() {
			It("should raise an error", func() {
				arch := "x86_64"
				dist := "rhel-90"
				package_name := "emacs"
				image := models.Image{
					Commit:       &models.Commit{Arch: arch},
					Distribution: dist,
				}
				imageBuilder := &models.SearchPackageResult{}
				var s models.SearchPackage
				s.Name = package_name
				imageBuilder.Data = append(imageBuilder.Data, s)
				imageBuilder.Meta.Count = 1
				mockImageBuilderClient.EXPECT().SearchPackage(
					package_name, arch, dist).Return(imageBuilder, new(imageBuilderClient.PackageRequestError))
				error := service.ValidateImagePackage(package_name, &image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(MatchError(new(imageBuilderClient.PackageRequestError)))
			})
		})
		When("When Meta.Count is zero", func() {
			It("should raise PackageNameDoesNotExist", func() {
				arch := "x86_64"
				dist := "rhel-90"
				package_name := "emacs"
				image := models.Image{
					Commit:       &models.Commit{Arch: arch},
					Distribution: dist,
				}
				imageBuilder := &models.SearchPackageResult{}
				var s models.SearchPackage
				s.Name = package_name
				imageBuilder.Data = append(imageBuilder.Data, s)
				imageBuilder.Meta.Count = 0
				mockImageBuilderClient.EXPECT().SearchPackage(
					package_name, arch, dist).Return(imageBuilder, nil)
				error := service.ValidateImagePackage(package_name, &image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(MatchError(new(services.PackageNameDoesNotExist)))
			})
		})
		When("When package name is not found", func() {
			It("should also raise PackageNameDoesNotExist", func() {
				arch := "x86_64"
				dist := "rhel-90"
				package_name := "emacs"
				wrong_package_name := "vim-common"
				image := models.Image{
					Commit:       &models.Commit{Arch: arch},
					Distribution: dist,
				}
				imageBuilder := &models.SearchPackageResult{}
				var s models.SearchPackage
				s.Name = package_name
				imageBuilder.Data = append(imageBuilder.Data, s)
				imageBuilder.Meta.Count = 1
				mockImageBuilderClient.EXPECT().SearchPackage(
					wrong_package_name, arch, dist).Return(imageBuilder, nil)
				error := service.ValidateImagePackage(wrong_package_name, &image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(MatchError(new(services.PackageNameDoesNotExist)))

			})
		})
		When("When package name is found", func() {
			It("should return nil", func() {
				arch := "x86_64"
				dist := "rhel-90"
				package_name := "emacs"
				image := models.Image{
					Commit:       &models.Commit{Arch: arch},
					Distribution: dist,
				}
				imageBuilder := &models.SearchPackageResult{}
				var s models.SearchPackage
				s.Name = package_name
				imageBuilder.Data = append(imageBuilder.Data, s)
				imageBuilder.Meta.Count = 1
				mockImageBuilderClient.EXPECT().SearchPackage(
					package_name, arch, dist).Return(imageBuilder, nil)
				error := service.ValidateImagePackage(package_name, &image)
				Expect(error).ToNot(HaveOccurred())
				Expect(error).To(BeNil())
			})
		})

	})
	Describe("Test ValidateCustomImagePackage function", func() {
		When("When there's no valid request", func() {
			It("should raise NewBadRequest", func() {
				image := models.Image{
					ThirdPartyRepositories: []models.ThirdPartyRepo{},
					CustomPackages:         []models.Package{{Name: ""}},
				}
				expectedError := errors.New("expected error returned by SearchContentPackage")
				mockRepositories.EXPECT().SearchContentPackage(gomock.Any(), gomock.Any()).Return(nil, expectedError)
				error := service.ValidateImageCustomPackage(&image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(MatchError(expectedError))
			})
		})

		When("When SearchPackage fails to find a package", func() {
			It("should raise an error", func() {
				image := models.Image{
					ThirdPartyRepositories: []models.ThirdPartyRepo{},
					CustomPackages:         []models.Package{{Name: ""}},
				}
				mockRepositories.EXPECT().SearchContentPackage(gomock.Any(), gomock.Any()).
					Return(nil, &repositories.PackageRequestError{})
				error := service.ValidateImageCustomPackage(&image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(MatchError(new(repositories.PackageRequestError)))
			})
		})

		When("When package name is not found", func() {
			It("should return nil", func() {
				image := models.Image{
					ThirdPartyRepositories: []models.ThirdPartyRepo{},
					CustomPackages:         []models.Package{{Name: "C"}},
				}
				mockRepositories.EXPECT().SearchContentPackage(gomock.Any(), gomock.Any()).
					Return(nil, nil)
				error := service.ValidateImageCustomPackage(&image)
				Expect(error).To(HaveOccurred())
				Expect(error).To(MatchError(new(services.PackageNameDoesNotExist)))
			})
		})
		When("When package name is found", func() {
			It("should return value", func() {
				rp := []repositories.SearchPackageResponse{{PackageName: "cat", Summary: "cat test"}}
				image := models.Image{
					ThirdPartyRepositories: []models.ThirdPartyRepo{{URL: "http://testcom"}},
					CustomPackages:         []models.Package{{Name: "cat"}},
				}
				mockRepositories.EXPECT().SearchContentPackage(gomock.Any(), gomock.Any()).
					Return(&rp, nil)
				error := service.ValidateImageCustomPackage(&image)
				Expect(error).ToNot(HaveOccurred())
				Expect(error).To(BeNil())
			})
		})

	})
	Describe("Create image when ValidateImagePackage", func() {
		orgID := faker.UUIDHyphenated()
		// requestID := faker.UUIDHyphenated()
		imageName := faker.UUIDHyphenated()
		When("When image-builder SearchPackage succeed", func() {
			It("image create with valid package name", func() {
				arch := &models.Commit{Arch: "x86_64"}
				dist := "rhel-85"
				pkgs := []models.Package{
					{
						Name: "vim-common",
					},
				}
				image := models.Image{OrgID: orgID, Distribution: dist, Name: imageName, Packages: pkgs, Commit: arch}
				expectedErr := fmt.Errorf("failed to create commit for image")
				imageBuilder := &models.SearchPackageResult{}
				var s models.SearchPackage
				s.Name = "vim-common"
				imageBuilder.Data = append(imageBuilder.Data, s)
				imageBuilder.Meta.Count = 1
				mockImageBuilderClient.EXPECT().SearchPackage("vim-common", "x86_64", "rhel-85").Return(imageBuilder, nil)
				mockImageBuilderClient.EXPECT().ComposeCommit(&image).Return(&image, expectedErr)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
			})
		})
		When("When image-builder SearchPackage fail", func() {
			It("image does not create because invalid package name", func() {
				arch := &models.Commit{Arch: "x86_64"}
				dist := "rhel-85"
				pkgs := []models.Package{
					{
						Name: "badrpm",
					},
				}
				image := models.Image{OrgID: orgID, Distribution: dist, Name: imageName, Packages: pkgs, Commit: arch}
				expectedErr := fmt.Errorf("package name doesn't exist")
				imageBuilder := &models.SearchPackageResult{}
				imageBuilder.Meta.Count = 0
				mockImageBuilderClient.EXPECT().SearchPackage("badrpm", "x86_64", "rhel-85").Return(imageBuilder, expectedErr)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				// But imageSet is not Created
				Expect(image.ImageSetID).To(BeNil())
			})
		})
		When("When image-builder Validation fail", func() {
			It("image does not create because empty architecture", func() {
				arch := &models.Commit{Arch: ""}
				dist := "rhel-85"
				pkgs := []models.Package{
					{
						Name: "vim-common",
					},
				}
				image := models.Image{OrgID: orgID, Distribution: dist, Name: imageName, Packages: pkgs, Commit: arch}
				// search function is not called
				mockImageBuilderClient.EXPECT().SearchPackage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("value is not one of the allowed values"))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				// But imageSet is not Created
				Expect(image.ImageSetID).To(BeNil())
			})
		})
		When("When image-builder Validation fail", func() {
			It("image does not create because empty distribution", func() {
				arch := &models.Commit{Arch: "x86_64"}
				dist := ""
				pkgs := []models.Package{
					{
						Name: "vim-common",
					},
				}
				image := models.Image{OrgID: orgID, Distribution: dist, Name: imageName, Packages: pkgs, Commit: arch}
				// search function is not called
				mockImageBuilderClient.EXPECT().SearchPackage(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("value is not one of the allowed values"))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				// But imageSet is not Created
				Expect(image.ImageSetID).To(BeNil())
			})
		})
	})
	Context("update image with invalid package name", func() {
		It("should have an error returned by image builder", func() {
			id, _ := faker.RandomInt(1)
			uid := uint(id[0])
			orgID := faker.UUIDHyphenated()
			imageSet := &models.ImageSet{OrgID: orgID}
			result := db.DB.Save(imageSet)
			arch := &models.Commit{Arch: "x86_64", OrgID: orgID}
			pkgs := []models.Package{
				{
					Name: "badrpm",
				},
			}
			Expect(result.Error).To(Not(HaveOccurred()))
			previousImage := &models.Image{
				Status:       models.ImageStatusSuccess,
				Commit:       &models.Commit{RepoID: &uid, OrgID: orgID},
				Version:      1,
				Distribution: "rhel-85",
				Name:         faker.Name(),
				ImageSetID:   &imageSet.ID,
				OrgID:        orgID,
			}
			image := &models.Image{
				Commit:       arch,
				Distribution: "rhel-85",
				OutputTypes:  []string{models.ImageTypeCommit},
				Version:      2,
				Name:         previousImage.Name,
				Packages:     pkgs,
				OrgID:        orgID,
			}
			result = db.DB.Save(previousImage)
			Expect(result.Error).To(Not(HaveOccurred()))
			expectedErr := fmt.Errorf("package name doesn't exist")
			imageBuilder := &models.SearchPackageResult{}
			imageBuilder.Meta.Count = 0
			mockImageBuilderClient.EXPECT().SearchPackage("badrpm", "x86_64", "rhel-85").Return(imageBuilder, expectedErr)
			actualErr := service.UpdateImage(ctx, image, previousImage)
			Expect(actualErr).To(HaveOccurred())
			Expect(actualErr).To(MatchError(expectedErr))
		})
	})
	Context("Get image set images View", func() {
		orgID := common.DefaultOrgID
		imageSet := models.ImageSet{OrgID: orgID, Name: faker.Name()}
		db.DB.Create(&imageSet)
		image1 := models.Image{
			OrgID: orgID, Name: imageSet.Name, ImageSetID: &imageSet.ID, Version: 1,
			OutputTypes: []string{models.ImageTypeCommit, models.ImageTypeInstaller},
			ImageType:   models.ImageTypeInstaller,
			Status:      models.ImageStatusSuccess,
			Installer:   &models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusSuccess},
			Commit:      &models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), Status: models.ImageStatusSuccess},
		}
		db.DB.Create(&image1)
		image2 := models.Image{
			OrgID: orgID, Name: imageSet.Name, ImageSetID: &imageSet.ID, Version: 2,
			OutputTypes: []string{models.ImageTypeCommit, models.ImageTypeInstaller},
			ImageType:   models.ImageTypeInstaller,
			Status:      models.ImageStatusError,
			Installer:   &models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL(), Status: models.ImageStatusPending},
			Commit:      &models.Commit{OrgID: orgID, OSTreeCommit: faker.UUIDHyphenated(), Status: models.ImageStatusError},
		}
		db.DB.Create(&image2)

		imageSetInner := models.ImageSet{OrgID: orgID, Name: faker.Name()}
		db.DB.Create(&imageSetInner)
		imageInner := models.Image{OrgID: orgID, Name: imageSet.Name, ImageSetID: &imageSetInner.ID, Installer: &models.Installer{OrgID: orgID, ImageBuildISOURL: faker.URL()}}
		db.DB.Create(&imageInner)

		imageSetDB := db.DB.Where("image_set_id = ?", imageSet.ID).Order("created_at DESC")

		It("return the right image set images view count", func() {
			imagesViewsCount, err := service.GetImagesViewCount(imageSetDB)
			Expect(err).ToNot(HaveOccurred())
			Expect(imagesViewsCount).To(Equal(int64(2)))
		})
		It("return the right image set images view", func() {
			imagesViews, err := service.GetImagesView(30, 0, imageSetDB)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(*imagesViews)).To(Equal(2))
			for ind, expectedImage := range []models.Image{image2, image1} {
				imageView := (*imagesViews)[ind]
				Expect(imageView.ID).To(Equal(expectedImage.ID))
				Expect(imageView.Name).To(Equal(expectedImage.Name))
				Expect(imageView.Version).To(Equal(expectedImage.Version))
				Expect(len(imageView.OutputTypes)).To(Equal(len(expectedImage.OutputTypes)))
				Expect(len(imageView.OutputTypes)).To(Equal(2))
				Expect(imageView.ImageType).To(Equal(expectedImage.ImageType))
				Expect(imageView.OutputTypes[0]).To(Equal(models.ImageTypeCommit))
				Expect(imageView.OutputTypes[1]).To(Equal(models.ImageTypeInstaller))
				Expect(imageView.Status).To(Equal(expectedImage.Status))
			}
			Expect((*imagesViews)[0].ImageBuildIsoURL).To(BeEmpty())
			Expect((*imagesViews)[0].CommitCheckSum).To(BeEmpty())
			Expect((*imagesViews)[1].ImageBuildIsoURL).To(Equal(fmt.Sprintf("/api/edge/v1/storage/isos/%d", image1.Installer.ID)))
			Expect((*imagesViews)[1].CommitCheckSum).To(Equal(image1.Commit.OSTreeCommit))

		})
	})
	Describe("delete image", func() {
		When("image is in error state", func() {
			Context("by id", func() {
				It("image and image set is deleted successfully", func() {
					orgID := common.DefaultOrgID
					imageSet := models.ImageSet{OrgID: orgID, Name: faker.Name()}
					db.DB.Create(&imageSet)
					image1 := models.Image{
						OrgID:      orgID,
						Name:       imageSet.Name,
						ImageSetID: &imageSet.ID,
						Version:    1,
						Status:     models.ImageStatusError,
					}
					db.DB.Create(&image1)
					err := service.DeleteImage(&image1)
					Expect(err).To(BeNil())
					var tempImage models.Image
					res := db.DB.First(&tempImage, image1)
					Expect(res.Error.Error()).Should(Equal("record not found"))
					var tempImageSet models.ImageSet
					res = db.DB.First(&tempImageSet, imageSet)
					Expect(res.Error.Error()).Should(Equal("record not found"))
				})
				It("image is deleted successfully", func() {
					orgID := common.DefaultOrgID
					imageSet := models.ImageSet{OrgID: orgID, Name: faker.Name()}
					db.DB.Create(&imageSet)
					image1 := models.Image{
						OrgID:      orgID,
						Name:       imageSet.Name,
						ImageSetID: &imageSet.ID,
						Version:    1,
						Status:     models.ImageStatusError,
					}
					image2 := models.Image{
						OrgID:      orgID,
						Name:       imageSet.Name,
						ImageSetID: &imageSet.ID,
						Version:    1,
						Status:     models.ImageStatusError,
					}
					db.DB.Create(&image1)
					db.DB.Create(&image2)
					err := service.DeleteImage(&image2)
					Expect(err).To(BeNil())
					var tempImage models.Image
					res := db.DB.First(&tempImage, image2)
					Expect(res.Error.Error()).Should(Equal("record not found"))
					res = db.DB.First(&tempImage, image1)
					Expect(res.Error).To(BeNil())
					var tempImageSet models.ImageSet
					res = db.DB.First(&tempImageSet, imageSet)
					Expect(res.Error).To(BeNil())
					Expect(tempImageSet.Version).To(Equal(image1.Version))
				})
			})
		})
		When("image is not in error state", func() {
			Context("by id", func() {
				It("image is not deleted", func() {
					orgID := common.DefaultOrgID
					imageSet := models.ImageSet{OrgID: orgID, Name: faker.Name()}
					db.DB.Create(&imageSet)
					image2 := models.Image{
						OrgID:      orgID,
						Name:       imageSet.Name,
						ImageSetID: &imageSet.ID,
						Version:    1,
						Status:     models.ImageStatusCreated,
					}
					db.DB.Create(&image2)
					err := service.DeleteImage(&image2)
					Expect(err).ToNot(BeNil())
				})
			})
		})
		When("image has not been saved ", func() {
			Context("by id", func() {
				It("delete image errors", func() {
					orgID := common.DefaultOrgID
					imageSet := models.ImageSet{OrgID: orgID, Name: faker.Name()}
					db.DB.Create(&imageSet)
					image2 := models.Image{
						OrgID:      orgID,
						Name:       imageSet.Name,
						ImageSetID: &imageSet.ID,
						Version:    1,
						Status:     models.ImageStatusCreated,
					}
					err := service.DeleteImage(&image2)
					Expect(err).ToNot(BeNil())
				})
			})
		})
		When("image set has not been saved ", func() {
			Context("by id", func() {
				It("delete image errors", func() {
					orgID := common.DefaultOrgID
					id := uint(0)
					imageSet := models.ImageSet{OrgID: orgID, Name: faker.Name()}
					image2 := models.Image{
						OrgID:      orgID,
						Name:       imageSet.Name,
						ImageSetID: &id,
						Version:    1,
						Status:     models.ImageStatusError,
					}
					err := service.DeleteImage(&image2)
					Expect(err.Error()).Should(Equal("record not found"))
				})
			})
		})
	})

	Describe("get devices of image", func() {
		When("image exists with some devices", func() {
			var image1, image2 *models.Image
			var devices []models.Device
			var imageSet *models.ImageSet
			BeforeEach(func() {
				imageSet = &models.ImageSet{
					Name:    "test",
					Version: 2,
					OrgID:   common.DefaultOrgID,
				}
				result := db.DB.Create(imageSet)
				Expect(result.Error).ToNot(HaveOccurred())
				image1 = &models.Image{
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      common.DefaultOrgID,
					Commit: &models.Commit{
						OrgID: common.DefaultOrgID,
						Arch:  "x86_64",
					},
				}
				result = db.DB.Create(image1)
				Expect(result.Error).ToNot(HaveOccurred())
				image2 = &models.Image{
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					OrgID:      common.DefaultOrgID,
					Commit: &models.Commit{
						OrgID: common.DefaultOrgID,
						Arch:  "x86_64",
						InstalledPackages: []models.InstalledPackage{
							{Name: "vim"},
						},
					},
				}
				result = db.DB.Create(image2)
				Expect(result.Error).ToNot(HaveOccurred())

				devices = []models.Device{
					{OrgID: common.DefaultOrgID, UUID: faker.UUIDHyphenated(), ImageID: image2.ID},
					{OrgID: common.DefaultOrgID, UUID: faker.UUIDHyphenated(), ImageID: image2.ID},
				}

			})
			Context("Get Devices count image", func() {
				It("should return device count of image2", func() {
					db.DB.Create(&devices)
					count, err := service.GetImageDevicesCount(image2.ID)
					Expect(err).To(BeNil())
					Expect(count).To(Equal(int64(2)))

				})
				It("should return 0 device of image1", func() {
					db.DB.Create(&devices)
					count, err := service.GetImageDevicesCount(image1.ID)
					Expect(err).To(BeNil())
					Expect(count).To(Equal(int64(0)))
				})

				It("GetUpdateInfo of image2 with 2 system and one installPackage", func() {
					var imageDiff *models.ImageUpdateAvailable
					db.DB.Create(&devices)
					totalPackage := len(image2.Commit.InstalledPackages)
					imageDiff, err := service.GetUpdateInfo(*image2)
					Expect(err).ToNot(HaveOccurred())
					Expect(imageDiff.Image.TotalPackages).To(Equal(totalPackage))
					Expect(imageDiff.Image.TotalDevicesWithImage).To(Equal(int64(2)))
				})
			})

			Context("Should not get Devices count image", func() {
				conf := config.Get()
				BeforeEach(func() {

					conf.Auth = true

				})
				AfterEach(func() {
					conf.Auth = false
				})
				It("GetImageDevicesCount should return error in case that OrgID not found", func() {
					ctx := context.Background()
					ctx = identity.WithIdentity(ctx, identity.XRHID{Identity: identity.Identity{
						OrgID: ""}})
					imageService := services.NewImageService(ctx, log.NewEntry(log.StandardLogger()))

					_, err := imageService.GetImageDevicesCount(image2.ID)
					Expect(err.Error()).To(Equal("cannot find org-id"))
				})
				It("GetUpdateInfo should return error when GetImageDevicesCount return error", func() {
					ctx := context.Background()
					ctx = identity.WithIdentity(ctx, identity.XRHID{Identity: identity.Identity{
						OrgID: ""}})
					imageService := services.NewImageService(ctx, log.NewEntry(log.StandardLogger()))
					_, err1 := imageService.GetUpdateInfo(*image2)
					Expect(err1.Error()).To(Equal("cannot find org-id"))
				})
			})
		})
	})

	Context("AddPackageInfo", func() {
		imageName := faker.Name()
		orgID := common.DefaultOrgID
		imageSet := models.ImageSet{
			Name:    imageName,
			Version: 2,
			OrgID:   orgID,
		}
		resImageSet := db.DB.Create(&imageSet)
		image1 := models.Image{
			Name:       imageName,
			Status:     models.ImageStatusSuccess,
			ImageSetID: &imageSet.ID,
			Version:    1,
			OrgID:      orgID,
			Commit: &models.Commit{
				OrgID: orgID,
				Arch:  "x86_64",
				InstalledPackages: []models.InstalledPackage{
					{Name: "vim"},
				},
			},
		}
		resImage1 := db.DB.Create(&image1)
		image2 := models.Image{
			Name:       imageName,
			Status:     models.ImageStatusSuccess,
			ImageSetID: &imageSet.ID,
			Version:    2,
			OrgID:      orgID,
			Commit: &models.Commit{
				OrgID: orgID,
				Arch:  "x86_64",
				InstalledPackages: []models.InstalledPackage{
					{Name: "mc"},
					{Name: "emacs"},
					{Name: "gcc"},
				},
			},
		}
		resImage2 := db.DB.Create(&image2)
		devices := []models.Device{
			{OrgID: orgID, UUID: faker.UUIDHyphenated(), ImageID: image2.ID},
			{OrgID: orgID, UUID: faker.UUIDHyphenated(), ImageID: image2.ID},
		}
		resDevices := db.DB.Create(devices)

		It("all records created successfully", func() {
			Expect(resImageSet.Error).ToNot(HaveOccurred())
			Expect(resImage1.Error).ToNot(HaveOccurred())
			Expect(resImage2.Error).ToNot(HaveOccurred())
			Expect(resDevices.Error).ToNot(HaveOccurred())
		})

		It("should return the correct values", func() {
			imageDetails, err := service.AddPackageInfo(&image2)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageDetails).ToNot(BeNil())
			totalPackages := len(image2.Commit.InstalledPackages)
			Expect(imageDetails.Packages).To(Equal(totalPackages))
			Expect(imageDetails.UpdateAdded).To(Equal(totalPackages))
			Expect(imageDetails.UpdateRemoved).To(Equal(len(image1.Commit.InstalledPackages)))
			Expect(imageDetails.UpdateUpdated).To(Equal(0))
		})

		It("should return error when GetUpdateInfo return error", func() {
			imageName := faker.Name()
			imageSet := models.ImageSet{
				Name:    imageName,
				Version: 2,
				OrgID:   orgID,
			}
			result := db.DB.Create(&imageSet)
			Expect(result.Error).ToNot(HaveOccurred())
			// when image1 is without commit this should return error from GetUpdateInfo
			image1 := models.Image{
				Name:       imageName,
				Status:     models.ImageStatusSuccess,
				ImageSetID: &imageSet.ID,
				Version:    1,
				OrgID:      orgID,
			}
			result = db.DB.Create(&image1)
			Expect(result.Error).ToNot(HaveOccurred())
			image2 := models.Image{
				Name:       imageName,
				Status:     models.ImageStatusSuccess,
				ImageSetID: &imageSet.ID,
				Version:    2,
				OrgID:      orgID,
				Commit: &models.Commit{
					OrgID: orgID,
					Arch:  "x86_64",
					InstalledPackages: []models.InstalledPackage{
						{Name: "vim"},
					},
				},
			}
			result = db.DB.Create(&image2)
			Expect(result.Error).ToNot(HaveOccurred())
			_, err := service.AddPackageInfo(&image2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(services.ImageCommitNotFoundMsg))
		})
	})

	Context("GetUpdateInfo", func() {
		imageName := faker.Name()
		orgID := common.DefaultOrgID
		imageSet := models.ImageSet{
			Name:    imageName,
			Version: 3,
			OrgID:   orgID,
		}
		resImageSet := db.DB.Create(&imageSet)
		image1 := models.Image{
			Name:       imageName,
			Status:     models.ImageStatusSuccess,
			ImageSetID: &imageSet.ID,
			Version:    1,
			OrgID:      orgID,
			Commit: &models.Commit{
				OrgID: orgID,
				Arch:  "x86_64",
				InstalledPackages: []models.InstalledPackage{
					{Name: "vim"},
				},
			},
		}
		resImage1 := db.DB.Create(&image1)
		image0 := models.Image{
			Name:       imageName,
			Status:     models.ImageStatusError,
			ImageSetID: &imageSet.ID,
			Version:    2,
			OrgID:      orgID,
		}
		resImage0 := db.DB.Create(&image0)
		image2 := models.Image{
			Name:       imageName,
			Status:     models.ImageStatusSuccess,
			ImageSetID: &imageSet.ID,
			Version:    3,
			OrgID:      orgID,
			Commit: &models.Commit{
				OrgID: orgID,
				Arch:  "x86_64",
				InstalledPackages: []models.InstalledPackage{
					{Name: "mc"},
					{Name: "emacs"},
					{Name: "gcc"},
				},
			},
		}
		resImage2 := db.DB.Create(&image2)
		devices := []models.Device{
			{OrgID: orgID, UUID: faker.UUIDHyphenated(), ImageID: image2.ID},
			{OrgID: orgID, UUID: faker.UUIDHyphenated(), ImageID: image2.ID},
		}
		resDevices := db.DB.Create(devices)

		It("all records created successfully", func() {
			Expect(resImageSet.Error).ToNot(HaveOccurred())
			Expect(resImage0.Error).ToNot(HaveOccurred())
			Expect(resImage1.Error).ToNot(HaveOccurred())
			Expect(resImage2.Error).ToNot(HaveOccurred())
			Expect(resDevices.Error).ToNot(HaveOccurred())
		})

		It("should return the correct values", func() {
			imageUpdateAvailable, err := service.GetUpdateInfo(image2)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageUpdateAvailable).ToNot(BeNil())
			totalPackages := len(image2.Commit.InstalledPackages)
			Expect(imageUpdateAvailable.Image.TotalPackages).To(Equal(totalPackages))
			Expect(imageUpdateAvailable.Image.TotalDevicesWithImage).To(Equal(int64(len(devices))))
			Expect(len(imageUpdateAvailable.PackageDiff.Removed)).To(Equal(totalPackages))
			Expect(len(imageUpdateAvailable.PackageDiff.Added)).To(Equal(len(image1.Commit.InstalledPackages)))
			Expect(len(imageUpdateAvailable.PackageDiff.Upgraded)).To(Equal(0))
		})

		It("should return nil update when image has not success status", func() {
			imageUpdateAvailable, err := service.GetUpdateInfo(image0)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageUpdateAvailable).To(BeNil())
		})

		It("should return nil update when image has not been updated", func() {
			imageUpdateAvailable, err := service.GetUpdateInfo(image1)
			Expect(err).ToNot(HaveOccurred())
			Expect(imageUpdateAvailable).To(BeNil())
		})
	})

	Context("CreateRepoForImage", func() {
		var image *models.Image
		var orgID string

		BeforeEach(func() {

			orgID = faker.UUIDHyphenated()
			image = &models.Image{
				OrgID:     orgID,
				Name:      faker.UUIDHyphenated(),
				RequestID: faker.UUIDHyphenated(),
				Installer: &models.Installer{OrgID: orgID},
				Commit: &models.Commit{
					OrgID: orgID,
					Repo:  &models.Repo{}},
			}
			err := db.DB.Create(image).Error
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return successfully if AWS repo status is success", func() {
			expectedRepo := models.Repo{
				URL:        faker.URL(),
				Status:     models.RepoStatusSuccess,
				PulpURL:    faker.URL(),
				PulpStatus: models.RepoStatusError,
			}
			err := db.DB.Create(&expectedRepo).Error
			Expect(err).ToNot(HaveOccurred())
			mockRepoBuilder.EXPECT().ImportRepo(ctx, gomock.AssignableToTypeOf(&models.Repo{})).Return(&expectedRepo, nil)
			_, err = service.CreateRepoForImage(context.Background(), image)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return successfully if Pulp repo status is success", func() {
			expectedRepo := models.Repo{
				URL:        faker.URL(),
				Status:     models.RepoStatusError,
				PulpURL:    faker.URL(),
				PulpStatus: models.RepoStatusSuccess,
			}
			err := db.DB.Create(&expectedRepo).Error
			Expect(err).ToNot(HaveOccurred())
			mockRepoBuilder.EXPECT().ImportRepo(ctx, gomock.AssignableToTypeOf(&models.Repo{})).Return(&expectedRepo, nil)
			_, err = service.CreateRepoForImage(context.Background(), image)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error if AWS AND Pulp repo status is not success", func() {
			expectedRepo := models.Repo{
				URL:        faker.URL(),
				Status:     models.RepoStatusError,
				PulpURL:    faker.URL(),
				PulpStatus: models.RepoStatusError,
			}
			expectedError := errors.New("No repo has been created")
			mockRepoBuilder.EXPECT().ImportRepo(ctx, gomock.AssignableToTypeOf(&models.Repo{})).Return(&expectedRepo, expectedError)
			_, err := service.CreateRepoForImage(context.Background(), image)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		})
	})

	Context("SetErrorStatusOnImage", func() {
		var image *models.Image
		var orgID string

		BeforeEach(func() {
			orgID = faker.UUIDHyphenated()
			image = &models.Image{
				OrgID:     orgID,
				Name:      faker.UUIDHyphenated(),
				RequestID: faker.UUIDHyphenated(),
				Installer: &models.Installer{OrgID: orgID},
				Commit:    &models.Commit{OrgID: orgID},
			}
			err := db.DB.Create(image).Error
			Expect(err).ToNot(HaveOccurred())
		})

		It("should set all statuses as error", func() {
			someError := errors.New("some error happened when building image")
			service.SetErrorStatusOnImage(someError, image)
			// reload image from db
			err := db.DB.Preload("Commit").Preload("Installer").First(image, image.ID).Error
			Expect(err).ToNot(HaveOccurred())
			Expect(image.Status).To(Equal(models.ImageStatusError))
			Expect(image.Commit.Status).To(Equal(models.ImageStatusError))
			Expect(image.Installer.Status).To(Equal(models.ImageStatusError))
		})

	})

	Context("ProcessImage", func() {
		var image models.Image
		var orgID string
		var initialDefaultLoopDelay time.Duration

		BeforeEach(func() {
			orgID = faker.UUIDHyphenated()
			initialDefaultLoopDelay = services.DefaultLoopDelay
			services.DefaultLoopDelay = 1 * time.Second
			image = models.Image{
				OrgID:       orgID,
				Name:        faker.UUIDHyphenated(),
				OutputTypes: []string{models.ImageTypeCommit},
				Status:      models.ImageStatusBuilding,
				RequestID:   faker.UUIDHyphenated(),
				Installer:   &models.Installer{OrgID: orgID},
				Commit: &models.Commit{
					OrgID:  orgID,
					Status: models.ImageStatusBuilding,
				},
			}
			err := db.DB.Create(&image).Error
			Expect(err).ToNot(HaveOccurred())
		})
		AfterEach(func() {
			// restore the default loop delay
			services.DefaultLoopDelay = initialDefaultLoopDelay
		})

		It("should not restore a deleted image when building", func() {
			wg := sync.WaitGroup{}
			wg.Add(1)
			mockImageBuilderClient.EXPECT().GetCommitStatus(gomock.AssignableToTypeOf(&models.Image{})).DoAndReturn(func(builderImage *models.Image) (*models.Image, error) {
				defer wg.Done()
				builderImage.Commit.Status = models.ImageStatusBuilding
				builderImage.Status = models.ImageStatusBuilding
				// delete the image while building the image
				err := db.DB.Delete(&models.Image{}, builderImage.ID).Error
				Expect(err).ToNot(HaveOccurred())
				// return back with no error, to simulate building still going
				return builderImage, nil
			}).Times(1)

			err := service.ProcessImage(context.Background(), &image, false)
			Expect(err).ToNot(HaveOccurred())

			wg.Wait()
			// sleep a fraction of second to give a chance to the remaining code after GetCommitStatus to execute
			time.Sleep(100 * time.Millisecond)
			// reload the image from database and ensure it's still in deleted state
			err = db.DB.First(&image, image.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
		})
	})

	Context("CreateInstallerForImage", func() {
		var image *models.Image
		var orgID string
		var initialDefaultLoopDelay time.Duration

		BeforeEach(func() {
			orgID = faker.UUIDHyphenated()
			initialDefaultLoopDelay = services.DefaultLoopDelay
			services.DefaultLoopDelay = 1 * time.Second
			image = &models.Image{
				OrgID:     orgID,
				Name:      faker.UUIDHyphenated(),
				RequestID: faker.UUIDHyphenated(),
				Installer: &models.Installer{OrgID: orgID, Username: faker.UUIDHyphenated(), SSHKey: "ssh-rsa dd:00:eeff:10"},
				Commit: &models.Commit{
					OrgID:            orgID,
					OSTreeCommit:     faker.UUIDHyphenated(),
					ImageBuildTarURL: faker.URL(),
					Status:           models.ImageStatusSuccess,
				},
			}
			err := db.DB.Create(image).Error
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			services.BuildCommand = exec.Command
			// restore the default loop delay
			services.DefaultLoopDelay = initialDefaultLoopDelay
		})

		// as ginkgo does not support mocking exec.Command
		// for CreateInstallerForImage successfully test (when feature.ImageCreateISOEDA is disabled), please look at TestCreateInstallerForImageSuccessful

		It("should run ComposeInstaller successfully and run GetInstallerStatus fails", func() {
			mockImageBuilderClient.EXPECT().ComposeInstaller(ctx, image).Return(image, nil)
			expectedError := errors.New("expected installer status error")
			mockImageBuilderClient.EXPECT().GetInstallerStatus(image).DoAndReturn(
				func(builderImage *models.Image) (*models.Image, error) {
					builderImage.Status = models.ImageStatusError
					builderImage.Installer.Status = models.ImageStatusError
					return builderImage, expectedError
				})

			installerImage, err := service.CreateInstallerForImage(context.Background(), image)
			Expect(err).ToNot(HaveOccurred())
			Expect(installerImage).To(Equal(image))

			installerStatusErr := service.ProcessInstaller(context.Background(), image)
			Expect(installerStatusErr).To(HaveOccurred())
			Expect(installerStatusErr).To(Equal(expectedError))
			Expect(image.Status).To(Equal(models.ImageStatusError))
			Expect(image.Installer.Status).To(Equal(models.ImageStatusError))
		})

		It("should return error when ComposeInstaller fails", func() {
			expectedError := errors.New("expected ComposeInstaller error")
			mockImageBuilderClient.EXPECT().ComposeInstaller(ctx, image).Return(nil, expectedError)

			_, err := service.CreateInstallerForImage(context.Background(), image)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		})

		It("should not restore a deleted image when building", func() {
			mockImageBuilderClient.EXPECT().ComposeInstaller(ctx, image).Return(image, nil)
			mockImageBuilderClient.EXPECT().GetInstallerStatus(image).DoAndReturn(func(builderImage *models.Image) (*models.Image, error) {
				builderImage.Installer.Status = models.ImageStatusBuilding
				builderImage.Status = models.ImageStatusBuilding
				// delete the image while building installer
				err := db.DB.Debug().Delete(&models.Image{}, builderImage.ID).Error
				Expect(err).ToNot(HaveOccurred())
				// return back with no error, to simulate building still going
				return builderImage, nil
			}).Times(1)

			installerImage, err := service.CreateInstallerForImage(context.Background(), image)
			Expect(err).ToNot(HaveOccurred())
			Expect(installerImage).To(Equal(image))

			installerStatusErr := service.ProcessInstaller(context.Background(), image)
			Expect(installerStatusErr).To(HaveOccurred())
			Expect(installerStatusErr).To(MatchError(new(services.ImageNotFoundError)))

			// reload the image from database and ensure it's still in deleted state
			err = db.DB.First(&image, image.ID).Error
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(gorm.ErrRecordNotFound))
		})
	})
})

func TestCreateInstallerForImageSuccessfully(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx := context.Background()

	currentDir, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	// set the templates path
	originalTemplatesPath := config.Get().TemplatesPath
	config.Get().TemplatesPath = path.Join(currentDir, "..", "..", "templates")
	defer func(templatePath string) {
		// restore template path
		config.Get().TemplatesPath = templatePath
	}(originalTemplatesPath)

	orgID := faker.UUIDHyphenated()
	image := &models.Image{
		OrgID:     orgID,
		Name:      faker.UUIDHyphenated(),
		Installer: &models.Installer{OrgID: orgID, Username: faker.UUIDHyphenated(), SSHKey: "ssh-rsa dd:00:eeff:10"},
		Commit: &models.Commit{
			OrgID:            orgID,
			OSTreeCommit:     faker.UUIDHyphenated(),
			ImageBuildTarURL: faker.URL(),
			Status:           models.ImageStatusSuccess,
		},
	}
	err = db.DB.Create(image).Error
	g.Expect(err).ToNot(HaveOccurred())

	ctrl := gomock.NewController(GinkgoT())
	mockImageBuilderClient := mock_imagebuilder.NewMockClientInterface(ctrl)
	mockFilesService := mock_services.NewMockFilesService(ctrl)
	mockUploader := mock_files.NewMockUploader(ctrl)
	service := services.ImageService{
		Service:      services.NewService(context.Background(), log.NewEntry(log.StandardLogger())),
		ImageBuilder: mockImageBuilderClient,
		FilesService: mockFilesService,
	}
	defer func() {
		// restore the original exec command builder
		services.BuildCommand = exec.Command
		ctrl.Finish()
	}()

	// create a server to serve the iso, for testing we return an empty file.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emptyFileReader := bytes.NewReader([]byte{})
		_, _ = io.Copy(w, emptyFileReader)
	}))
	defer ts.Close()

	mockImageBuilderClient.EXPECT().ComposeInstaller(ctx, image).Return(image, nil)
	mockImageBuilderClient.EXPECT().GetInstallerStatus(image).DoAndReturn(func(builderImage *models.Image) (*models.Image, error) {
		// simulate that Installer status was successful and set the appropriate statuses and data
		builderImage.Status = models.ImageStatusSuccess
		builderImage.Installer.Status = models.ImageStatusSuccess
		builderImage.Installer.ImageBuildISOURL = ts.URL
		return builderImage, nil
	})

	expectedUploadUrl := faker.URL()
	mockFilesService.EXPECT().GetUploader().Return(mockUploader)
	mockUploader.EXPECT().UploadFile(gomock.Any(), gomock.Any()).Return(expectedUploadUrl, nil)

	installerImage, err := service.CreateInstallerForImage(context.Background(), image)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(installerImage).To(Equal(image))

	installerStatusErr := service.ProcessInstaller(context.Background(), image)

	g.Expect(installerStatusErr).ToNot(HaveOccurred())
	g.Expect(image.Status).To(Equal(models.ImageStatusSuccess))
	g.Expect(image.Installer.Status).To(Equal(models.ImageStatusSuccess))
	g.Expect(image.Installer.ImageBuildISOURL).To(Equal(expectedUploadUrl))
}
