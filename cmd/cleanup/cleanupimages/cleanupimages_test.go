package cleanupimages_test

import (
	"errors"
	"os"

	"github.com/redhatinsights/edge-api/cmd/cleanup/cleanupimages"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Cleanup images", func() {

	Context("CleanUPImages feature flag is disabled", func() {

		BeforeEach(func() {
			err := os.Unsetenv(feature.CleanUPImages.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not run clean up of images when feature flag is disabled", func() {
			image := models.Image{Name: faker.UUIDHyphenated()}
			Expect(image.DeletedAt.Value()).To(BeNil())
			err := cleanupimages.CleanUpAllImages(nil)
			Expect(err).To(MatchError(cleanupimages.ErrImagesCleanUPNotAvailable))
		})
	})

	Context("CleanUPImages feature flag is enabled", func() {

		BeforeEach(func() {

			err := os.Setenv(feature.CleanUPImages.EnvVar, "true")
			Expect(err).NotTo(HaveOccurred())

		})

		AfterEach(func() {
			err := os.Unsetenv(feature.CleanUPImages.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("DeleteImage", func() {
			var orgID string
			var existingImage models.Image
			var image models.Image
			var imageSet models.ImageSet
			var customRepos []models.ThirdPartyRepo
			var updateTransaction models.UpdateTransaction

			BeforeEach(func() {
				if orgID != "" {
					// do setup only once
					return
				}
				orgID = faker.UUIDHyphenated()
				existingImage := models.Image{OrgID: orgID, Name: faker.Name()}
				err := db.DB.Create(&existingImage).Error
				Expect(err).ToNot(HaveOccurred())

				imageSet = models.ImageSet{OrgID: orgID, Name: faker.Name()}
				err = db.DB.Create(&imageSet).Error
				Expect(err).ToNot(HaveOccurred())

				customRepos = []models.ThirdPartyRepo{
					{OrgID: orgID, Name: faker.Name(), URL: faker.URL()},
					{OrgID: orgID, Name: faker.Name(), URL: faker.URL()},
				}
				err = db.DB.Create(&customRepos).Error
				Expect(err).ToNot(HaveOccurred())

				image = models.Image{
					Name:       imageSet.Name,
					OrgID:      orgID,
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,
					Commit: &models.Commit{
						OrgID:             orgID,
						Status:            models.ImageStatusSuccess,
						Repo:              &models.Repo{Status: models.ImageStatusSuccess, URL: faker.URL()},
						InstalledPackages: []models.InstalledPackage{{Name: "nano"}, {Name: "git"}},
					},
					Installer:              &models.Installer{OrgID: orgID, Status: models.ImageStatusSuccess, ImageBuildISOURL: faker.URL()},
					Packages:               []models.Package{{Name: "nano"}, {Name: "git"}},
					CustomPackages:         []models.Package{{Name: "BAT"}, {Name: "CAT"}},
					ThirdPartyRepositories: customRepos,
				}
				err = db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())

				updateTransaction = models.UpdateTransaction{
					OrgID:  orgID,
					Commit: image.Commit,
				}
				err = db.DB.Create(&updateTransaction).Error
				Expect(err).ToNot(HaveOccurred())

				err = db.DB.Delete(&image).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not DeleteImages that are not soft deleted", func() {
				candidateImage := cleanupimages.CandidateImage{ImageID: existingImage.ID, ImageDeletedAt: existingImage.DeletedAt}
				err := cleanupimages.DeleteImage(&candidateImage)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupimages.ErrImageNotCleanUPCandidate))
			})

			It("should DeleteImages that are soft deleted", func() {
				candidatesImages, err := cleanupimages.GetCandidateImages(db.Org(orgID, "images"))
				Expect(err).ToNot(HaveOccurred())
				Expect(len(candidatesImages)).To(Equal(1))
				candidateImage := candidatesImages[0]
				Expect(candidateImage.ImageID).To(Equal(image.ID))
				err = cleanupimages.DeleteImage(&candidateImage)
				Expect(err).ToNot(HaveOccurred())

				// ensure image deleted
				var deletedImage models.Image
				err = db.DB.Unscoped().First(&deletedImage, image.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// ensure imageSet deleted, as have only one image that was deleted
				var deletedImageSet models.ImageSet
				err = db.DB.Unscoped().First(&deletedImageSet, imageSet.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})
		})

		Context("AWS storage", func() {
			var ctrl *gomock.Controller
			var s3Client *files.S3Client
			var s3ClientAPI *mock_files.MockS3ClientAPI
			var s3FolderDeleter *mock_files.MockBatchFolderDeleterAPI

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				s3ClientAPI = mock_files.NewMockS3ClientAPI(ctrl)
				s3FolderDeleter = mock_files.NewMockBatchFolderDeleterAPI(ctrl)
				s3Client = &files.S3Client{
					Client:        s3ClientAPI,
					FolderDeleter: s3FolderDeleter,
				}
			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("should delete aws s3 folder", func() {
				folderPath := "/test/folder/to/delete"
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, folderPath).Return(nil)
				err := cleanupimages.DeleteAWSFolder(s3Client, folderPath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error when aws folder deleter returns error", func() {
				folderPath := "/test/folder/to/delete"
				expectedError := errors.New("expected error returned by aws s3 folder deleter")
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, folderPath).Return(expectedError)
				err := cleanupimages.DeleteAWSFolder(s3Client, folderPath)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})

			It("should delete aws s3 file", func() {
				filePath := "/test/file/to/delete"
				// s3ClientAPI.EXPECT().DeleteObject(config.Get().BucketName, filePath).Return(nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(filePath),
				}).Return(nil, nil)
				err := cleanupimages.DeleteAWSFile(s3Client, filePath)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error when aws delete object returns error", func() {
				filePath := "/test/file/to/delete"
				expectedError := errors.New("expected error returned by aws s3 file deleter")
				s3ClientAPI.EXPECT().DeleteObject(
					&s3.DeleteObjectInput{
						Bucket: aws.String(config.Get().BucketName),
						Key:    aws.String(filePath),
					}).Return(nil, expectedError)
				err := cleanupimages.DeleteAWSFile(s3Client, filePath)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(expectedError))
			})
		})

		Context("CleanUP image", func() {
			It("should not clean up images that are not soft deleted", func() {
				err := cleanupimages.CleanUpImage(nil, &cleanupimages.CandidateImage{
					ImageID: uint(100), ImageStatus: models.ImageStatusSuccess, ImageDeletedAt: gorm.DeletedAt{},
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupimages.ErrImageNotCleanUPCandidate))
			})

			It("should not clean up images that are not soft deleted and with not with error status", func() {
				err := cleanupimages.CleanUpImage(nil, &cleanupimages.CandidateImage{
					ImageID: uint(100), ImageStatus: models.ImageStatusInterrupted, ImageDeletedAt: gorm.DeletedAt{},
				})
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupimages.ErrImageNotCleanUPCandidate))
			})
		})

		Context("CleanUpAllImages", func() {
			var ctrl *gomock.Controller
			var s3Client *files.S3Client
			var s3ClientAPI *mock_files.MockS3ClientAPI
			var s3FolderDeleter *mock_files.MockBatchFolderDeleterAPI

			var orgID string

			// image should not be deleted, and it's content should not be cleared
			var existingImage models.Image
			// image should be deleted, and it's content should be cleared
			var image models.Image
			// image should not be deleted, and it's content should be cleared
			var errImage models.Image
			var imageSet models.ImageSet

			var imageTarPath string
			var imageISOPath string
			var imageRepoPath string

			var errImageTarPath string
			var errImageRepoPath string

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				s3ClientAPI = mock_files.NewMockS3ClientAPI(ctrl)
				s3FolderDeleter = mock_files.NewMockBatchFolderDeleterAPI(ctrl)
				s3Client = &files.S3Client{
					Client:        s3ClientAPI,
					FolderDeleter: s3FolderDeleter,
				}

				orgID = faker.UUIDHyphenated()

				imageSet = models.ImageSet{OrgID: orgID, Name: faker.Name()}
				err := db.DB.Create(&imageSet).Error
				Expect(err).ToNot(HaveOccurred())

				existingImage = models.Image{OrgID: orgID, Name: faker.Name(), ImageSetID: &imageSet.ID, Status: models.ImageStatusSuccess}
				err = db.DB.Create(&existingImage).Error
				Expect(err).ToNot(HaveOccurred())

				imageTarPath = "/image/tar/file/path/" + faker.UUIDHyphenated()
				imageISOPath = "/image/iso/file/path/" + faker.UUIDHyphenated()
				imageRepoPath = "/image/repo/path/" + faker.UUIDHyphenated()

				image = models.Image{
					Name:       imageSet.Name,
					OrgID:      orgID,
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,

					Commit: &models.Commit{
						OrgID:  orgID,
						Status: models.ImageStatusSuccess,
						Repo: &models.Repo{
							Status: models.ImageStatusSuccess,
							URL:    "https://buket.example.com" + imageRepoPath,
						},
						ImageBuildTarURL: "https://buket.example.com" + imageTarPath,
					},
					Installer: &models.Installer{
						OrgID:            orgID,
						Status:           models.ImageStatusSuccess,
						ImageBuildISOURL: "https://buket.example.com" + imageISOPath,
					},
				}
				err = db.DB.Create(&image).Error
				Expect(err).ToNot(HaveOccurred())

				// soft delete image
				err = db.DB.Delete(&image).Error
				Expect(err).ToNot(HaveOccurred())

				errImageTarPath = "/errImage/tar/file/path/" + faker.UUIDHyphenated()
				errImageRepoPath = "/errImage/repo/path/" + faker.UUIDHyphenated()

				errImage = models.Image{
					Name:       imageSet.Name,
					OrgID:      orgID,
					Status:     models.ImageStatusError,
					ImageSetID: &imageSet.ID,

					Commit: &models.Commit{
						OrgID:  orgID,
						Status: models.ImageStatusSuccess,
						Repo: &models.Repo{
							Status: models.ImageStatusSuccess,
							URL:    "https://buket.example.com" + errImageRepoPath,
						},
						ImageBuildTarURL: "https://buket.example.com" + errImageTarPath,
					},
					Installer: &models.Installer{
						OrgID:  orgID,
						Status: models.ImageStatusError,
					},
				}
				err = db.DB.Create(&errImage).Error
				Expect(err).ToNot(HaveOccurred())

			})

			AfterEach(func() {
				ctrl.Finish()
			})

			It("should delete images and clear s3 content as expected", func() {
				// expect all image success aws content to be deleted
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageTarPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageISOPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, imageRepoPath).Return(nil)

				// expect all errImage success content to be deleted
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(errImageTarPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, errImageRepoPath).Return(nil)

				err := cleanupimages.CleanUpAllImages(s3Client)
				Expect(err).ToNot(HaveOccurred())

				// expect imageSet with many images should still exist
				err = db.DB.First(&models.ImageSet{}, imageSet.ID).Error
				Expect(err).ToNot(HaveOccurred())

				// expect an exitingImage to still exiting
				err = db.DB.First(&models.Image{}, existingImage.ID).Error
				Expect(err).ToNot(HaveOccurred())

				// expect image do not exist anymore
				err = db.DB.Unscoped().First(&models.Image{}, image.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// expect the image with error status to still exist
				err = db.DB.Joins("Commit").Preload("Commit.Repo").First(&errImage, errImage.ID).Error
				Expect(err).ToNot(HaveOccurred())
				// The commit and Repo status is set to cleared
				Expect(errImage.Commit.Status).To(Equal(models.ImageStatusStorageCleaned))
				Expect(errImage.Commit.Repo.Status).To(Equal(models.ImageStatusStorageCleaned))
			})
		})
	})
})
