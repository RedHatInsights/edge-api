package cleanupimages_test

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/redhatinsights/edge-api/cmd/cleanup/cleanupimages"
	"github.com/redhatinsights/edge-api/cmd/cleanup/storage"
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
			// image should be deleted, and it's content should be cleared, but update transaction is not deleted
			var image2 models.Image
			var update models.UpdateTransaction
			// image should not be deleted, and it's content should be cleared
			var errImage models.Image
			var imageSet models.ImageSet

			var imageTarPath string
			var imageISOPath string
			var imageRepoPath string

			var image2TarPath string
			var image2ISOPath string
			var image2RepoPath string

			var errImageTarPath string
			var errImageRepoPath string

			var initialTimeDuration time.Duration
			var confiDeleteAttempts int

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				s3ClientAPI = mock_files.NewMockS3ClientAPI(ctrl)
				s3FolderDeleter = mock_files.NewMockBatchFolderDeleterAPI(ctrl)
				s3Client = &files.S3Client{
					Client:        s3ClientAPI,
					FolderDeleter: s3FolderDeleter,
				}

				initialTimeDuration = storage.DefaultTimeDuration
				storage.DefaultTimeDuration = 1 * time.Millisecond
				confiDeleteAttempts = int(config.Get().DeleteFilesAttempts)

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

				image2TarPath = "/image2/tar/file/path/" + faker.UUIDHyphenated()
				image2ISOPath = "/image2/iso/file/path/" + faker.UUIDHyphenated()
				image2RepoPath = "/image2/repo/path/" + faker.UUIDHyphenated()

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

				image2 = models.Image{
					Name:       imageSet.Name,
					OrgID:      orgID,
					Status:     models.ImageStatusSuccess,
					ImageSetID: &imageSet.ID,

					Commit: &models.Commit{
						OrgID:  orgID,
						Status: models.ImageStatusSuccess,
						Repo: &models.Repo{
							Status: models.ImageStatusSuccess,
							URL:    "https://buket.example.com" + image2RepoPath,
						},
						ImageBuildTarURL: "https://buket.example.com" + image2TarPath,
					},
					Installer: &models.Installer{
						OrgID:            orgID,
						Status:           models.ImageStatusSuccess,
						ImageBuildISOURL: "https://buket.example.com" + image2ISOPath,
					},
				}
				err = db.DB.Create(&image2).Error
				Expect(err).ToNot(HaveOccurred())
				update = models.UpdateTransaction{OrgID: orgID, CommitID: image2.Commit.ID}
				err = db.DB.Create(&update).Error
				Expect(err).ToNot(HaveOccurred())

				// soft delete images
				err = db.DB.Delete(&image).Error
				Expect(err).ToNot(HaveOccurred())
				err = db.DB.Delete(&image2).Error
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
				storage.DefaultTimeDuration = initialTimeDuration
			})

			It("should delete images and clear s3 content as expected", func() {
				// expect all image success aws content to be deleted
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageTarPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(image2TarPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageISOPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(image2ISOPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(imageRepoPath, "/")).Return(nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(image2RepoPath, "/")).Return(nil)

				// expect all errImage success content to be deleted
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(errImageTarPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(errImageRepoPath, "/")).Return(nil)

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
				// expect image commit does not exist
				err = db.DB.Unscoped().First(&models.Commit{}, image.Commit.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))

				// expect image2 do not exist anymore
				err = db.DB.Unscoped().First(&models.Image{}, image2.ID).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
				// expect image2 commit to still exist
				err = db.DB.Unscoped().First(&models.Commit{}, image2.Commit.ID).Error
				Expect(err).ToNot(HaveOccurred())

				// expect update transaction not deleted
				err = db.DB.Unscoped().First(&models.UpdateTransaction{}, update.ID).Error
				Expect(err).ToNot(HaveOccurred())

				// expect the image with error status to still exist
				err = db.DB.Joins("Commit").Preload("Commit.Repo").First(&errImage, errImage.ID).Error
				Expect(err).ToNot(HaveOccurred())
				// The commit and Repo status is set to cleared
				Expect(errImage.Commit.Status).To(Equal(models.ImageStatusStorageCleaned))
				Expect(errImage.Commit.Repo.Status).To(Equal(models.ImageStatusStorageCleaned))
			})

			It("should interrupt when any folder delete error persists", func() {
				expectedError := errors.New("expected folder delete error")
				defer func() {
					// teardown: remove image to not conflict with other tests
					candidateImage := cleanupimages.CandidateImage{ImageID: image.ID, ImageDeletedAt: image.DeletedAt}
					_ = cleanupimages.DeleteImage(&candidateImage)
				}()

				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageTarPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(image2TarPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageISOPath),
				}).Return(nil, nil).Times(0)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(image2ISOPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(imageRepoPath, "/")).Return(expectedError).Times(confiDeleteAttempts)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(image2RepoPath, "/")).Return(nil)

				// expect all errImage success content to be deleted
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(errImageTarPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(errImageRepoPath, "/")).Return(nil)

				err := cleanupimages.CleanUpAllImages(s3Client)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupimages.ErrCleanUpAllImagesInterrupted))
			})

			It("should interrupt when any file delete error persists", func() {
				expectedError := errors.New("expected file delete error")
				defer func() {
					// teardown: remove image to not conflict with other tests
					candidateImage := cleanupimages.CandidateImage{ImageID: image.ID, ImageDeletedAt: image.DeletedAt}
					_ = cleanupimages.DeleteImage(&candidateImage)
				}()
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageTarPath),
				}).Return(nil, expectedError).Times(confiDeleteAttempts)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(image2TarPath),
				}).Return(nil, nil)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(imageISOPath),
				}).Return(nil, nil).Times(0)
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(image2ISOPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(imageRepoPath, "/")).Return(nil).Times(0)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(image2RepoPath, "/")).Return(nil)

				// expect all errImage success content to be deleted
				s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(errImageTarPath),
				}).Return(nil, nil)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(errImageRepoPath, "/")).Return(nil)

				err := cleanupimages.CleanUpAllImages(s3Client)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(cleanupimages.ErrCleanUpAllImagesInterrupted))
			})
		})
	})
})
