package services_test

import (
	"context"
	"fmt"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	imageBuilderClient "github.com/redhatinsights/edge-api/pkg/clients/imagebuilder"
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
					OrgID:   common.DefaultOrgID,
				}
				result := db.DB.Create(imageSet)
				Expect(result.Error).ToNot(HaveOccurred())
				imageV1 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        common.DefaultOrgID,
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
					},
					Status:     models.ImageStatusError,
					ImageSetID: &imageSet.ID,
					Version:    2,
					OrgID:      common.DefaultOrgID,
				}
				db.DB.Create(imageV2.Commit)
				db.DB.Create(imageV2)
				imageV3 = &models.Image{
					Commit: &models.Commit{
						OSTreeCommit: faker.UUIDHyphenated(),
						OrgID:        common.DefaultOrgID,
					},
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Version:    3,
					OrgID:      common.DefaultOrgID,
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
				actualErr := service.UpdateImage(image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
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
				actualErr := service.UpdateImage(image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.ChangesRefs).To(BeFalse())
				Expect(image.Commit.OSTreeParentCommit).To(Equal(parentRepo.URL))
				Expect(image.Commit.OSTreeParentRef).To(Equal("rhel/8/x86_64/edge"))
				Expect(image.Commit.OSTreeRef).To(Equal("rhel/8/x86_64/edge"))
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
				actualErr := service.UpdateImage(image, previousImage)

				Expect(actualErr).To(HaveOccurred())
				Expect(actualErr).To(MatchError(expectedErr))
				Expect(image.Commit.ChangesRefs).To(BeTrue())
				Expect(image.Commit.OSTreeParentCommit).To(Equal(parentRepo.URL))
				Expect(image.Commit.OSTreeParentRef).To(Equal("rhel/8/x86_64/edge"))
				Expect(image.Commit.OSTreeRef).To(Equal("rhel/9/x86_64/edge"))
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
			It("should give an error", func() {
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
				device := models.Device{OrgID: orgID, ImageID: image.ID, UpdateAvailable: false}
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
	Describe("Create image when using getImageSetForNewImage", func() {
		orgID := faker.UUIDHyphenated()
		//requestID := faker.UUIDHyphenated()
		arch := &models.Commit{Arch: "x86_64"}
		imageName := faker.UUIDHyphenated()
		image := models.Image{OrgID: orgID, Distribution: "rhel-85", Name: imageName, Commit: arch}
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
	Describe("Create image when ValidateImagePackage", func() {
		orgID := faker.UUIDHyphenated()
		//requestID := faker.UUIDHyphenated()
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
				//expectedErr := fmt.Errorf("failed to create commit for image")
				expectedErr := fmt.Errorf("image builder search packages request error")

				imageBuilder := &imageBuilderClient.SearchPackageResult{}
				var s imageBuilderClient.SearchPackage
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
				expectedErr := fmt.Errorf("image builder search packages request error")
				//				expectedErr := fmt.Errorf("package name doesn't exist")
				imageBuilder := &imageBuilderClient.SearchPackageResult{}
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
		When("When image-builder SearchPackage fail", func() {
			It("image does not create because empty architecture", func() {
				arch := &models.Commit{Arch: ""}
				dist := "rhel-85"
				pkgs := []models.Package{
					{
						Name: "vim-common",
					},
				}
				image := models.Image{OrgID: orgID, Distribution: dist, Name: imageName, Packages: pkgs, Commit: arch}
				expectedErr := fmt.Errorf("architecture and/or distribution are not set")
				imageBuilder := &imageBuilderClient.SearchPackageResult{}
				mockImageBuilderClient.EXPECT().SearchPackage("vim-common", "", "rhel-85").Return(imageBuilder, expectedErr)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
				// image is not created
				Expect(image.ID).To(Equal(uint(0)))
				// But imageSet is not Created
				Expect(image.ImageSetID).To(BeNil())
			})
		})
		When("When image-builder SearchPackage fail", func() {
			It("image does not create because empty distribution", func() {
				arch := &models.Commit{Arch: "x86_64"}
				dist := ""
				pkgs := []models.Package{
					{
						Name: "vim-common",
					},
				}
				image := models.Image{OrgID: orgID, Distribution: dist, Name: imageName, Packages: pkgs, Commit: arch}
				expectedErr := fmt.Errorf("architecture and/or distribution are not set")
				imageBuilder := &imageBuilderClient.SearchPackageResult{}
				mockImageBuilderClient.EXPECT().SearchPackage("vim-common", "x86_64", "").Return(imageBuilder, expectedErr)
				err := service.CreateImage(&image)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(expectedErr.Error()))
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
			imageBuilder := &imageBuilderClient.SearchPackageResult{}
			imageBuilder.Meta.Count = 0
			mockImageBuilderClient.EXPECT().SearchPackage("badrpm", "x86_64", "rhel-85").Return(imageBuilder, expectedErr)
			actualErr := service.UpdateImage(image, previousImage)
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
})
