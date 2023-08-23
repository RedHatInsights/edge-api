package cleanuporphancommits_test

import (
	"errors"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/cmd/cleanup/cleanuporphancommits"
	"github.com/redhatinsights/edge-api/cmd/cleanup/storage"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Cleanup orphan commits", func() {

	Context("CleanUPOrphanCommits feature flag is disabled", func() {
		BeforeEach(func() {
			err := os.Unsetenv(feature.CleanUPOrphanCommits.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not run clean up of orphan commits when feature flag is disabled", func() {
			err := cleanuporphancommits.CleanupAllOrphanCommits(nil, nil)
			Expect(err).To(MatchError(cleanuporphancommits.ErrCleanupOrphanCommitsNotAvailable))
		})
	})

	Context("CleanUPOrphanCommits feature flag is enabled", func() {
		BeforeEach(func() {
			err := os.Setenv(feature.CleanUPOrphanCommits.EnvVar, "true")
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := os.Unsetenv(feature.CleanUPOrphanCommits.EnvVar)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("CleanupAllOrphanCommits", func() {
			var ctrl *gomock.Controller
			var s3Client *files.S3Client
			var s3ClientAPI *mock_files.MockS3ClientAPI
			var s3FolderDeleter *mock_files.MockBatchFolderDeleterAPI

			var initialTimeDuration time.Duration
			var configDeleteAttempts int

			var orgID string
			var orphanCommit1RepoPath string
			var orphanCommit1 models.Commit
			var orphanCommit2 models.Commit

			var commitWithImage models.Commit
			var image models.Image

			var commitWithUpdate models.Commit
			var update models.UpdateTransaction

			var commitWithImageAndUpdate models.Commit
			var image2 models.Image
			var update2 models.UpdateTransaction

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
				configDeleteAttempts = int(config.Get().DeleteFilesAttempts)

				// setup data only once
				if orgID == "" {
					orgID = faker.UUIDHyphenated()
					// orphan commit with repo
					orphanCommit1RepoPath = "path/to/repo"
					orphanCommit1 = models.Commit{
						OrgID: orgID,
						Repo: &models.Repo{
							Status: models.RepoStatusSuccess,
							URL:    "https://repos.example.com/" + orphanCommit1RepoPath,
						},
						InstalledPackages: []models.InstalledPackage{
							{Name: "git"},
							{Name: "mc"},
						},
					}
					err := db.DB.Create(&orphanCommit1).Error
					Expect(err).ToNot(HaveOccurred())
					// orphan commit without repo
					orphanCommit2 = models.Commit{
						OrgID: orgID,
					}
					err = db.DB.Create(&orphanCommit2).Error
					Expect(err).ToNot(HaveOccurred())

					commitWithImage = models.Commit{OrgID: orgID}
					err = db.DB.Create(&commitWithImage).Error
					Expect(err).ToNot(HaveOccurred())

					image = models.Image{OrgID: orgID, Name: faker.Name(), CommitID: commitWithImage.ID}
					err = db.DB.Create(&image).Error
					Expect(err).ToNot(HaveOccurred())

					commitWithUpdate = models.Commit{OrgID: orgID}
					err = db.DB.Create(&commitWithUpdate).Error
					Expect(err).ToNot(HaveOccurred())

					update = models.UpdateTransaction{OrgID: orgID, CommitID: commitWithUpdate.ID}
					err = db.DB.Create(&update).Error
					Expect(err).ToNot(HaveOccurred())

					commitWithImageAndUpdate = models.Commit{OrgID: orgID}
					err = db.DB.Create(&commitWithImageAndUpdate).Error
					Expect(err).ToNot(HaveOccurred())
					image2 = models.Image{OrgID: orgID, Name: faker.Name(), CommitID: commitWithImageAndUpdate.ID}
					err = db.DB.Create(&image2).Error
					Expect(err).ToNot(HaveOccurred())

					update2 = models.UpdateTransaction{OrgID: orgID, CommitID: commitWithImageAndUpdate.ID}
					err = db.DB.Create(&update2).Error
					Expect(err).ToNot(HaveOccurred())
				}
			})

			AfterEach(func() {
				ctrl.Finish()
				storage.DefaultTimeDuration = initialTimeDuration
			})

			It("orphan commit with repo exist", func() {
				err := db.DB.Unscoped().First(&orphanCommit1).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("orphan commit repo exist", func() {
				err := db.DB.Unscoped().First(&orphanCommit1.Repo).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("orphan commit without repo exist", func() {
				err := db.DB.Unscoped().First(&orphanCommit2).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error when folder deleting fails", func() {
				expectedError := errors.New("expected error for s3 folder deletion")
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, orphanCommit1RepoPath).Return(expectedError).Times(configDeleteAttempts)
				err := cleanuporphancommits.CleanupAllOrphanCommits(s3Client, db.DB)
				Expect(err).To(MatchError(cleanuporphancommits.ErrCleanUpAllOrphanCommitsInterrupted))
			})

			It("should cleanup orphan commits successfully", func() {
				// simulate aws s3 folder deletion to be successful at last attempts
				expectedError := errors.New("expected error for s3 folder deletion")
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, orphanCommit1RepoPath).Return(expectedError).Times(configDeleteAttempts - 1)
				s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, orphanCommit1RepoPath).Return(nil).Times(1)
				err := cleanuporphancommits.CleanupAllOrphanCommits(s3Client, db.DB)
				Expect(err).ToNot(HaveOccurred())
			})

			It("orphan commit with repo does not exist", func() {
				err := db.DB.Unscoped().First(&orphanCommit1).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})

			It("orphan commit repo does not exist", func() {
				err := db.DB.Unscoped().First(&orphanCommit1.Repo).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})

			It("orphan commit without repo does not exist", func() {
				err := db.DB.Unscoped().First(&orphanCommit2).Error
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(gorm.ErrRecordNotFound))
			})

			It("commit with image still exist", func() {
				err := db.DB.Unscoped().First(&commitWithImage).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("commit with update still exist", func() {
				err := db.DB.Unscoped().First(&commitWithUpdate).Error
				Expect(err).ToNot(HaveOccurred())
			})

			It("commit with image and update still exist", func() {
				err := db.DB.Unscoped().First(&commitWithImageAndUpdate).Error
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
