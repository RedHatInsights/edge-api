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
	"github.com/redhatinsights/edge-api/pkg/routes/common"
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
					Account: common.DefaultAccount,
				}
				result := db.DB.Create(imageSet)
				Expect(result.Error).ToNot(HaveOccurred())
				imageV1 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					Account:    common.DefaultAccount,
				}
				result = db.DB.Create(imageV1.Commit)
				Expect(result.Error).ToNot(HaveOccurred())
				result = db.DB.Create(imageV1)
				Expect(result.Error).ToNot(HaveOccurred())
				imageV2 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
					},
					Status:     models.ImageStatusError,
					ImageSetID: &imageSet.ID,
					Version:    2,
					Account:    common.DefaultAccount,
				}
				db.DB.Create(imageV2.Commit)
				db.DB.Create(imageV2)
				imageV3 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    3,
					Account:    common.DefaultAccount,
				}
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
			})
			Context("when rollback image doesnt exists", func() {
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
				account := faker.UUIDHyphenated()
				imageSet := &models.ImageSet{Account: account}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					Account:    account,
					Status:     models.ImageStatusSuccess,
					Commit:     &models.Commit{RepoID: &uid},
					Version:    1,
					Name:       faker.Name(),
					ImageSetID: &imageSet.ID,
				}
				image := &models.Image{
					Account:     account,
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
				account := faker.UUIDHyphenated()
				imageSet := &models.ImageSet{Account: account}
				result := db.DB.Save(imageSet)
				Expect(result.Error).To(Not(HaveOccurred()))
				previousImage := &models.Image{
					Account:    account,
					Status:     models.ImageStatusSuccess,
					Commit:     &models.Commit{RepoID: &uid},
					Version:    1,
					Name:       faker.Name(),
					ImageSetID: &imageSet.ID,
				}
				image := &models.Image{
					Account:     account,
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
		Context("when checking if the image version we are trying to update is latest", func() {
			account := faker.UUIDHyphenated()
			imageSet := models.ImageSet{Account: account}
			db.DB.Save(&imageSet)
			image := models.Image{Account: account, ImageSetID: &imageSet.ID, Version: 1, Name: "image-same-name"}
			db.DB.Save(&image)
			image2 := models.Image{Account: account, ImageSetID: &imageSet.ID, Version: 2, Name: "image-same-name"}
			db.DB.Save(&image2)
			image3 := models.Image{Account: account, ImageSetID: &imageSet.ID, Version: 3, Name: "image-same-name"}
			db.DB.Save(&image3)
			// foreign image without account
			image4 := models.Image{ImageSetID: &imageSet.ID, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image4)
			// foreign image without image-set
			image5 := models.Image{Account: account, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image5)

			// foreign image from another account and image-set, is here to ensure we are analysing the correct collection
			account2 := faker.UUIDHyphenated()
			account2ImageSet := models.ImageSet{Account: account}
			db.DB.Save(&account2ImageSet)
			image6 := models.Image{Account: account2, ImageSetID: &account2ImageSet.ID, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image6)
			// foreign image from another image-set, is here to ensure we are analysing the correct collection
			imageSet2 := models.ImageSet{Account: account}
			db.DB.Save(&imageSet2)
			image7 := models.Image{Account: account, ImageSetID: &imageSet2.ID, Version: 4, Name: "image-same-name"}
			db.DB.Save(&image7)

			It("the image has to be defined", func() {
				err := service.CheckIfIsLatestVersion(&models.Image{Account: account, ImageSetID: &imageSet.ID, Version: 5, Name: "image-same-name"})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.ImageUnDefined)))
			})

			It("the image account must be be defined", func() {

				err := service.CheckIfIsLatestVersion(&image4)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(new(services.AccountNotSet)))
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
	Describe("validate images packages account", func() {
		Context("when creating an image using third party repository", func() {
			It("should validate the images with empty repos", func() {
				var repos []models.ThirdPartyRepo
				account := "00000"
				err := services.ValidateAllImageReposAreFromAccount(account, repos)
				Expect(err).ToNot(HaveOccurred())

			})
			It("should give an error", func() {
				var repos []models.ThirdPartyRepo
				account := ""
				err := services.ValidateAllImageReposAreFromAccount(account, repos)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("repository information is not valid"))
			})

			It("should validate the images with repos within same account", func() {
				account := "00000"
				repo1 := models.ThirdPartyRepo{Account: account, Name: faker.UUIDHyphenated(), URL: "https://repo1.simple.com"}
				result := db.DB.Create(&repo1)
				Expect(result.Error).ToNot(HaveOccurred())
				repo2 := models.ThirdPartyRepo{Account: account, Name: faker.UUIDHyphenated(), URL: "https://repo2.simple.com"}
				result = db.DB.Create(&repo2)
				Expect(result.Error).ToNot(HaveOccurred())
				err := services.ValidateAllImageReposAreFromAccount(account, []models.ThirdPartyRepo{repo1, repo2})
				Expect(err).ToNot(HaveOccurred())

			})
			It("should not validate the images with repos from different accounts", func() {
				account1 := "1111111"
				account2 := "2222222"
				repo1 := models.ThirdPartyRepo{Account: account1, Name: faker.UUIDHyphenated(), URL: "https://repo1.simple.com"}
				result := db.DB.Create(&repo1)
				Expect(result.Error).ToNot(HaveOccurred())
				repo2 := models.ThirdPartyRepo{Account: account2, Name: faker.UUIDHyphenated(), URL: "https://repo2.simple.com"}
				result = db.DB.Create(&repo2)
				Expect(result.Error).ToNot(HaveOccurred())
				err := services.ValidateAllImageReposAreFromAccount(account1, []models.ThirdPartyRepo{repo1, repo2})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError("some repositories were not found"))
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
					Account: common.DefaultAccount,
				}
				db.DB.Create(imageSet)

				image = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    1,
					Account:    common.DefaultAccount,
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
	})
	Describe("Devices update availability from image set", func() {
		Context("should set device update availability", func() {
			account := faker.UUIDHyphenated()
			imageSet := models.ImageSet{Account: account, Name: faker.UUIDHyphenated()}
			db.DB.Create(&imageSet)
			initialImages := []models.Image{
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, Account: account},
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, Account: account},
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, Account: account},
				{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, Account: account},
			}
			images := make([]models.Image, 0, len(initialImages))
			for _, image := range initialImages {
				db.DB.Create(&image)
				images = append(images, image)
				fmt.Println("IMG >>>>", image.ID)
			}

			devices := make([]models.Device, 0, len(images))
			for ind, image := range images {
				device := models.Device{Account: account, ImageID: image.ID, UpdateAvailable: false}
				if ind == len(images)-1 {
					device.UpdateAvailable = true
				}
				db.DB.Create(&device)
				devices = append(devices, device)
			}
			lastDevicesIndex := len(devices) - 1

			OtherImageSet := models.ImageSet{Account: account, Name: faker.UUIDHyphenated()}
			db.DB.Create(&OtherImageSet)

			otherImage := models.Image{Status: models.ImageStatusSuccess, ImageSetID: &OtherImageSet.ID, Account: account}
			db.DB.Create(&otherImage)
			OtherDevice := models.Device{Account: account, ImageID: otherImage.ID, UpdateAvailable: true}
			db.DB.Create(&OtherDevice)

			It("No error occurred without errors when calling function", func() {
				err := service.SetDevicesUpdateAvailabilityFromImageSet(account, imageSet.ID)
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
				err := service.SetDevicesUpdateAvailabilityFromImageSet(account, OtherImageSet.ID)
				Expect(err).To(BeNil())
				// reload other device
				var device models.Device
				result := db.DB.First(&device, OtherDevice.ID)
				Expect(result.Error).To(BeNil())
				Expect(device.UpdateAvailable).To(Equal(false))
			})

			It("should run without errors when no devices", func() {
				imageSet := models.ImageSet{Account: account, Name: faker.UUIDHyphenated()}
				result := db.DB.Create(&imageSet)
				Expect(result.Error).To(BeNil())
				image := models.Image{Status: models.ImageStatusSuccess, ImageSetID: &imageSet.ID, Account: account}
				result = db.DB.Create(&image)
				Expect(result.Error).To(BeNil())

				err := service.SetDevicesUpdateAvailabilityFromImageSet(account, imageSet.ID)
				Expect(err).To(BeNil())
			})
		})
	})
})
