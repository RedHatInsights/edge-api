package services_test

import (
	"archive/tar"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_services"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var testFile = "test.txt"
var testTarFile = "test.tar"

var _ = Describe("RepoBuilder Service Test", func() {
	var ctrl *gomock.Controller
	var service services.RepoBuilderInterface
	ctx := context.Background()
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		service = services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))

	})
	AfterEach(func() {
		ctrl.Finish()
	})
	Describe("#ExtractVersionRepo", func() {

		When("is valid", func() {
			It("should extract the tar file", func() {
				commit := &models.Commit{}

				filePath := fmt.Sprintf("/tmp/tar_extract_test_%d", time.Now().Unix())
				defer func(path string) {
					_ = os.Remove(path)
				}(filePath)
				filePathExtraction := filepath.Join(filePath, filePath)

				err := os.MkdirAll(filePathExtraction, 0755)
				Expect(err).ToNot(HaveOccurred())
				testFilePath, _ := createTestFile(filePath)
				testTarFilePath := filepath.Join(filePath, testTarFile)
				err = createTarball(testTarFilePath, testFilePath)

				Expect(err).ToNot(HaveOccurred())

				err = service.ExtractVersionRepo(commit, testTarFilePath, filePath)

				Expect(err).ToNot(HaveOccurred())

				fileContent, err := readTestFile(filePathExtraction)
				Expect(err).ToNot(HaveOccurred())
				Expect(fileContent).To(Equal("Some content to test"))
			})
		})

		When("when invalid", func() {
			var ctrl *gomock.Controller
			var repoBuilder services.RepoBuilder
			var mockFilesService *mock_services.MockFilesService
			var mockExtractor *mock_files.MockExtractor
			var filePath string
			var tarFilePath string
			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				mockFilesService = mock_services.NewMockFilesService(ctrl)
				mockExtractor = mock_files.NewMockExtractor(ctrl)
				ctx := context.Background()
				logger := log.NewEntry(log.StandardLogger())
				repoBuilder = services.RepoBuilder{
					Service:      services.NewService(ctx, logger),
					FilesService: mockFilesService,
					Log:          logger,
				}
				filePath = filepath.Join(os.TempDir(), fmt.Sprintf("tar_extract_test_%d", time.Now().Unix()))
				tarFilePath = filepath.Join(os.TempDir(), fmt.Sprintf("tar_files_test_%d", time.Now().Unix()), filePath, "repo.tar")
				err := os.MkdirAll(filePath, 0755)
				Expect(err).ToNot(HaveOccurred())
			})
			AfterEach(func() {
				ctrl.Finish()
				_ = os.RemoveAll(filePath)
			})

			It("should fail when commit is nil", func() {
				err := repoBuilder.ExtractVersionRepo(nil, testTarFile, filePath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("invalid Commit Provided: nil pointer"))
			})

			It("should fail when tar file does not exists", func() {
				commit := &models.Commit{}
				err := repoBuilder.ExtractVersionRepo(commit, tarFilePath, filePath)
				Expect(err).To(HaveOccurred())
				expectedErrorMessage := fmt.Sprintf("open %s: no such file or directory", tarFilePath)
				Expect(err.Error()).To(Equal(expectedErrorMessage))
			})

			It("should fail when tar extractor fail", func() {
				commit := &models.Commit{}

				testFilePath, err := createTestFile(filePath)
				Expect(err).ToNot(HaveOccurred())
				testTarFilePath := filepath.Join(filePath, testTarFile)
				err = createTarball(testTarFilePath, testFilePath)
				Expect(err).ToNot(HaveOccurred())

				expectedError := errors.New("extract error")
				mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
				mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), filePath).Return(expectedError)
				err = repoBuilder.ExtractVersionRepo(commit, testTarFilePath, filePath)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})
	})

	Describe("#DownloadVersionRepo", func() {
		var mockFilesService *mock_services.MockFilesService
		var mockDownloaderService *mock_services.MockDownloader
		var downloadService services.RepoBuilder
		var fileURL = "https://repos.fedorapeople.org/pulp/pulp/demo_repos/zoo/bear-4.1-1.noarch.rpm"
		var fileDest = "/tmp/download/"
		var fileName = "repo.tar"
		BeforeEach(func() {
			mockFilesService = mock_services.NewMockFilesService(ctrl)
			mockDownloaderService = mock_services.NewMockDownloader(ctrl)
			var ctx = context.Background()
			downloadService = services.RepoBuilder{
				Service:      services.NewService(ctx, log.NewEntry(log.StandardLogger())),
				FilesService: mockFilesService,
				Log:          &log.Entry{},
			}

		})
		When("is valid internal url", func() {
			It("should download the repo", func() {
				commit := &models.Commit{ExternalURL: false,
					ImageBuildTarURL: fileURL}
				mockDownloaderService.EXPECT().DownloadToPath(commit.ImageBuildTarURL, filepath.Join(fileDest, fileName)).Return(nil)
				mockFilesService.EXPECT().GetDownloader().Return(mockDownloaderService)
				n, err := downloadService.DownloadVersionRepo(commit, fileDest)

				Expect(err).ToNot(HaveOccurred())
				Expect(n).ToNot(BeNil())
				Expect(n).To(Equal(fmt.Sprintf("%v%v", fileDest, "repo.tar")))
			})

			It("should download the repo with ImageBuildHash", func() {
				commit := &models.Commit{ExternalURL: false, ImageBuildTarURL: fileURL, ImageBuildHash: faker.UUIDHyphenated()}
				tarFileName := commit.ImageBuildHash + ".tar"
				tarFilePath := filepath.Join(fileDest, tarFileName)
				mockDownloaderService.EXPECT().DownloadToPath(commit.ImageBuildTarURL, tarFilePath).Return(nil)
				mockFilesService.EXPECT().GetDownloader().Return(mockDownloaderService)
				destinationFilePath, err := downloadService.DownloadVersionRepo(commit, fileDest)

				Expect(err).ToNot(HaveOccurred())
				Expect(destinationFilePath).ToNot(BeEmpty())
				Expect(destinationFilePath).To(Equal(tarFilePath))
			})

			It("should return error when DownloadToPath fails", func() {
				commit := &models.Commit{ExternalURL: false, ImageBuildTarURL: fileURL}
				expectedError := errors.New("downloading from internal url failed")

				mockDownloaderService.EXPECT().DownloadToPath(commit.ImageBuildTarURL, filepath.Join(fileDest, fileName)).Return(expectedError)
				mockFilesService.EXPECT().GetDownloader().Return(mockDownloaderService)

				_, err := downloadService.DownloadVersionRepo(commit, fileDest)

				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})

			It("should return error when commit is nil", func() {
				expectedError := errors.New("invalid Commit Provided: nil pointer")

				_, err := downloadService.DownloadVersionRepo(nil, fileDest)

				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})

		When("is valid external url", func() {
			It("should download the repo", func() {
				commit := &models.Commit{ExternalURL: true, ImageBuildTarURL: fileURL}
				n, err := downloadService.DownloadVersionRepo(commit, fileDest)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(fmt.Sprintf("%v%v", fileDest, fileName)))
				err = os.RemoveAll(fileDest)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return error when the downloads fails", func() {
				commit := &models.Commit{ExternalURL: true, ImageBuildTarURL: "file url does not exist :("}
				_, err := downloadService.DownloadVersionRepo(commit, fileDest)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("first path segment in URL cannot contain colon"))
			})
		})
	})

	Describe("#UploadVersionRepo", func() {
		var ctrl *gomock.Controller
		var repoBuilder services.RepoBuilder
		var mockFilesService *mock_services.MockFilesService
		var mockUploader *mock_files.MockUploader
		var filesDirPath string
		var tarFilePath string
		var commit *models.Commit
		var orgID string
		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockFilesService = mock_services.NewMockFilesService(ctrl)
			mockUploader = mock_files.NewMockUploader(ctrl)
			ctx := context.Background()
			logger := log.NewEntry(log.StandardLogger())
			repoBuilder = services.RepoBuilder{
				Service:      services.NewService(ctx, logger),
				FilesService: mockFilesService,
				Log:          logger,
			}
			orgID = faker.UUIDHyphenated()
			commit = &models.Commit{OrgID: orgID, ExternalURL: true, Repo: &models.Repo{URL: faker.URL()}}
			err := db.DB.Create(commit).Error
			Expect(err).ToNot(HaveOccurred())
			filesDirPath = filepath.Join(os.TempDir(), fmt.Sprintf("tar_upload_test_%d", time.Now().Unix()))
			tarFilePath = filepath.Join(filesDirPath, "repo.tar")
			err = os.MkdirAll(filesDirPath, 0755)
			Expect(err).ToNot(HaveOccurred())
			_, err = os.Create(tarFilePath)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
			_ = os.RemoveAll(filesDirPath)
		})

		It("should upload tar file successfully", func() {
			expectedUploadPath := filepath.Clean(fmt.Sprintf("v2/%s/tar/%v/%s", orgID, *commit.RepoID, tarFilePath))
			uploadURL := faker.URL() + expectedUploadPath

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadFile(tarFilePath, expectedUploadPath).Return(uploadURL, nil)

			err := repoBuilder.UploadVersionRepo(commit, tarFilePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(commit.ExternalURL).To(BeFalse())
			Expect(commit.ImageBuildTarURL).To(Equal(uploadURL))
		})

		It("should return err when upload fails", func() {
			expectedUploadPath := filepath.Clean(fmt.Sprintf("v2/%s/tar/%v/%s", orgID, *commit.RepoID, tarFilePath))
			expectedError := errors.New("upload error occurred when uploading tar file")

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadFile(tarFilePath, expectedUploadPath).Return("", expectedError)

			err := repoBuilder.UploadVersionRepo(commit, tarFilePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})

		It("should fail if commit is nil", func() {
			err := repoBuilder.UploadVersionRepo(nil, tarFilePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid Commit Provided: nil pointer"))
		})

		It("should fail if commit repoID  is nil", func() {
			err := repoBuilder.UploadVersionRepo(&models.Commit{}, tarFilePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid Commit.RepoID Provided: nil pointer"))
		})
	})

	Describe("#ImportRepo", func() {
		var ctrl *gomock.Controller
		var repoBuilder services.RepoBuilder
		var mockFilesService *mock_services.MockFilesService
		var mockExtractor *mock_files.MockExtractor
		var mockDownloader *mock_services.MockDownloader
		var mockUploader *mock_files.MockUploader
		var commit *models.Commit
		var orgID string
		var repoTempPath string
		var repoWorkPath string
		var repoTarFilePath string
		var initialRepoTempPath string

		BeforeEach(func() {
			initialRepoTempPath = config.Get().RepoTempPath
			repoTempPath = filepath.Join(os.TempDir(), fmt.Sprintf("repo_temp_test_%d", time.Now().Unix()))
			err := os.MkdirAll(repoTempPath, 0755)
			Expect(err).ToNot(HaveOccurred())
			config.Get().RepoTempPath = repoTempPath

			ctrl = gomock.NewController(GinkgoT())
			mockFilesService = mock_services.NewMockFilesService(ctrl)
			mockUploader = mock_files.NewMockUploader(ctrl)
			mockExtractor = mock_files.NewMockExtractor(ctrl)
			mockDownloader = mock_services.NewMockDownloader(ctrl)
			ctx := context.Background()
			logger := log.NewEntry(log.StandardLogger())
			repoBuilder = services.RepoBuilder{
				Service:      services.NewService(ctx, logger),
				FilesService: mockFilesService,
				Log:          logger,
			}
			orgID = faker.UUIDHyphenated()
			commit = &models.Commit{OrgID: orgID, ExternalURL: false, ImageBuildTarURL: faker.URL(), Repo: &models.Repo{URL: faker.URL()}}
			err = db.DB.Create(commit).Error
			Expect(err).ToNot(HaveOccurred())
			repoWorkPath = filepath.Join(repoTempPath, strconv.FormatUint(uint64(commit.Repo.ID), 10))
			err = os.MkdirAll(repoWorkPath, 0755)
			Expect(err).ToNot(HaveOccurred())
			repoTarFilePath = filepath.Join(repoWorkPath, "repo.tar")
			_, err = os.Create(repoTarFilePath)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
			config.Get().RepoTempPath = initialRepoTempPath
			_ = os.RemoveAll(repoTempPath)
		})

		It("should import repo successfully", func() {
			expectedUploadPath := filepath.Clean(fmt.Sprintf("v2/%s/tar/%v/%s", orgID, *commit.RepoID, repoTarFilePath))
			uploadTarURL := faker.URL() + expectedUploadPath
			expectedRepoURL := faker.URL()

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadFile(repoTarFilePath, expectedUploadPath).Return(uploadTarURL, nil)

			mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
			mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), repoWorkPath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadRepo(
				filepath.Join(repoWorkPath, "repo"),
				strconv.FormatUint(uint64(commit.Repo.ID), 10),
				"public-read",
			).Return(expectedRepoURL, nil)

			repo, err := repoBuilder.ImportRepo(commit.Repo)
			Expect(err).ToNot(HaveOccurred())
			Expect(repo).ToNot(BeNil())
			Expect(repo.URL).To(Equal(expectedRepoURL))
			Expect(repo.Status).To(Equal(models.RepoStatusSuccess))
		})

		It("should return error when repo commit not found", func() {
			_, err := repoBuilder.ImportRepo(&models.Repo{Model: models.Model{ID: 99999999999}})
			Expect(err).To(HaveOccurred())
			Expect(err).To(Equal(gorm.ErrRecordNotFound))
		})

		It("should return error when tar repo download fail", func() {
			expectedError := errors.New("tar repo download failure")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(commit.ImageBuildTarURL, repoTarFilePath).Return(expectedError)

			_, err := repoBuilder.ImportRepo(commit.Repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error downloading repo"))
		})

		It("should return error when tar repo upload fail", func() {
			expectedUploadPath := filepath.Clean(fmt.Sprintf("v2/%s/tar/%v/%s", orgID, *commit.RepoID, repoTarFilePath))
			expectedError := errors.New("tar repo upload failure")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadFile(repoTarFilePath, expectedUploadPath).Return("", expectedError)

			_, err := repoBuilder.ImportRepo(commit.Repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})

		It("should return error when extract tar repo fail", func() {
			expectedUploadPath := filepath.Clean(fmt.Sprintf("v2/%s/tar/%v/%s", orgID, *commit.RepoID, repoTarFilePath))
			uploadTarURL := faker.URL() + expectedUploadPath
			expectedError := errors.New("tar repo extract failure")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadFile(repoTarFilePath, expectedUploadPath).Return(uploadTarURL, nil)

			mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
			mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), repoWorkPath).Return(expectedError)

			_, err := repoBuilder.ImportRepo(commit.Repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})

		It("should return error when upload repo fail", func() {
			expectedUploadPath := filepath.Clean(fmt.Sprintf("v2/%s/tar/%v/%s", orgID, *commit.RepoID, repoTarFilePath))
			uploadTarURL := faker.URL() + expectedUploadPath
			expectedError := errors.New("repo upload failure")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadFile(repoTarFilePath, expectedUploadPath).Return(uploadTarURL, nil)

			mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
			mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), repoWorkPath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadRepo(
				filepath.Join(repoWorkPath, "repo"),
				strconv.FormatUint(uint64(commit.Repo.ID), 10),
				"public-read",
			).Return("", expectedError)

			_, err := repoBuilder.ImportRepo(commit.Repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})
	})

	Describe("BuildUpdateRepo", func() {
		var ctrl *gomock.Controller
		var repoBuilder services.RepoBuilder
		var mockFilesService *mock_services.MockFilesService
		var mockExtractor *mock_files.MockExtractor
		var mockDownloader *mock_services.MockDownloader
		var mockUploader *mock_files.MockUploader
		var update *models.UpdateTransaction
		var orgID string
		var repoTempPath string
		var updateTempDirPath string
		var updateWorkPath string
		var repoTarFilePath string
		var initialRepoTempPath string

		BeforeEach(func() {
			initialRepoTempPath = config.Get().RepoTempPath
			repoTempPath = filepath.Join(os.TempDir(), fmt.Sprintf("repo_temp_test_%d", time.Now().Unix()))
			err := os.MkdirAll(repoTempPath, 0755)
			Expect(err).ToNot(HaveOccurred())
			config.Get().RepoTempPath = repoTempPath

			updateTempDirPath = filepath.Join(repoTempPath, "upd")
			err = os.MkdirAll(updateTempDirPath, 0755)
			Expect(err).ToNot(HaveOccurred())

			ctrl = gomock.NewController(GinkgoT())
			mockFilesService = mock_services.NewMockFilesService(ctrl)
			mockUploader = mock_files.NewMockUploader(ctrl)
			mockExtractor = mock_files.NewMockExtractor(ctrl)
			mockDownloader = mock_services.NewMockDownloader(ctrl)
			ctx := context.Background()
			logger := log.NewEntry(log.StandardLogger())
			repoBuilder = services.RepoBuilder{
				Service:      services.NewService(ctx, logger),
				FilesService: mockFilesService,
				Log:          logger,
			}
			orgID = faker.UUIDHyphenated()
			update = &models.UpdateTransaction{
				OrgID: orgID,
				Repo:  &models.Repo{},
				Commit: &models.Commit{
					OrgID:            orgID,
					ExternalURL:      false,
					ImageBuildTarURL: faker.URL(),
				},
				// gomega does not support the scenarios with mocking the exec.command,
				// scenarios with OldCommits will be tested using go testing in TestBuildUpdateRepoWithOldCommits functions
				OldCommits: []models.Commit{},
			}
			err = db.DB.Create(update).Error
			Expect(err).ToNot(HaveOccurred())
			updateWorkPath = filepath.Join(updateTempDirPath, strconv.FormatUint(uint64(update.ID), 10))
			err = os.MkdirAll(updateWorkPath, 0755)
			Expect(err).ToNot(HaveOccurred())
			repoTarFilePath = filepath.Join(updateWorkPath, "repo.tar")
			_, err = os.Create(repoTarFilePath)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			ctrl.Finish()
			config.Get().RepoTempPath = initialRepoTempPath
			err := os.RemoveAll(repoTempPath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should build update repo successfully without OldCommits", func() {
			expectedRepoURL := faker.URL()

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
			mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadRepo(
				filepath.Join(updateWorkPath, "repo"),
				strconv.FormatUint(uint64(update.ID), 10),
				"private",
			).Return(expectedRepoURL, nil)

			updateTransaction, err := repoBuilder.BuildUpdateRepo(update.ID)
			Expect(err).ToNot(HaveOccurred())
			Expect(updateTransaction).ToNot(BeNil())
			Expect(updateTransaction.Repo.URL).To(Equal(expectedRepoURL))
			Expect(updateTransaction.Repo.Status).To(Equal(models.RepoStatusSuccess))
		})

		It("should return error when update does not exist", func() {
			_, err := repoBuilder.BuildUpdateRepo(999999999)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(services.UpdateNotFoundErrorMsg))
		})

		It("should return error when update commit is nil", func() {
			update = &models.UpdateTransaction{OrgID: orgID}
			err := db.DB.Create(update).Error
			Expect(err).ToNot(HaveOccurred())

			_, err = repoBuilder.BuildUpdateRepo(update.ID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("invalid models.UpdateTransaction.Commit Provided: nil pointer"))
		})

		It("should return error when update repo is nil", func() {
			update = &models.UpdateTransaction{
				OrgID: orgID,
				Commit: &models.Commit{
					OrgID:            orgID,
					ExternalURL:      false,
					ImageBuildTarURL: faker.URL(),
				},
				Repo: nil,
			}
			err := db.DB.Create(update).Error
			Expect(err).ToNot(HaveOccurred())

			_, err = repoBuilder.BuildUpdateRepo(update.ID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("repo unavailable"))
		})

		It("should return error when tar repo downloads fails", func() {
			expectedError := errors.New("error occurred when downloading repo tar file")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, repoTarFilePath).Return(expectedError)

			_, err := repoBuilder.BuildUpdateRepo(update.ID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})

		It("should return error when tar repo extract fails", func() {
			expectedError := errors.New("error occurred when extracting repo tar file")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
			mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(expectedError)

			_, err := repoBuilder.BuildUpdateRepo(update.ID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})

		It("should return error when update repo upload fails", func() {
			expectedError := errors.New("error occurred when uploading update repo file")

			mockFilesService.EXPECT().GetDownloader().Return(mockDownloader)
			mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, repoTarFilePath).Return(nil)

			mockFilesService.EXPECT().GetExtractor().Return(mockExtractor)
			mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(nil)

			mockFilesService.EXPECT().GetUploader().Return(mockUploader)
			mockUploader.EXPECT().UploadRepo(
				filepath.Join(updateWorkPath, "repo"),
				strconv.FormatUint(uint64(update.ID), 10),
				"private",
			).Return("", expectedError)

			_, err := repoBuilder.BuildUpdateRepo(update.ID)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedError.Error()))
		})
	})
})

func TestBuildUpdateRepoWithOldCommits(t *testing.T) {
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	// enable oldCommits feature flag by setting the corresponding env variable
	err = os.Setenv(feature.BuildUpdateRepoWithOldCommits.EnvVar, "True")
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string) {
		// restore the standard command builder
		services.BuildCommand = exec.Command
		// restore the current dir path
		err = os.Chdir(dirPath)
		g.Expect(err).ToNot(HaveOccurred())
		// unset oldCommits feature flag
		err := os.Unsetenv(feature.BuildUpdateRepoWithOldCommits.EnvVar)
		g.Expect(err).ToNot(HaveOccurred())
	}(currentDir)

	initialRepoTempPath := config.Get().RepoTempPath
	repoTempPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_temp_test_%d", time.Now().Unix()))
	err = os.MkdirAll(repoTempPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	ctrl := gomock.NewController(t)

	defer func(mockController *gomock.Controller, createdRepoTempPath string, configRepoTempPath string) {
		mockController.Finish()
		config.Get().RepoTempPath = configRepoTempPath
		_ = os.RemoveAll(createdRepoTempPath)
	}(ctrl, repoTempPath, initialRepoTempPath)

	// set repo temp path to test predefined location
	config.Get().RepoTempPath = repoTempPath

	updateTempDirPath := filepath.Join(repoTempPath, "upd")
	err = os.MkdirAll(updateTempDirPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	mockFilesService := mock_services.NewMockFilesService(ctrl)
	mockUploader := mock_files.NewMockUploader(ctrl)
	mockExtractor := mock_files.NewMockExtractor(ctrl)
	mockDownloader := mock_services.NewMockDownloader(ctrl)
	ctx := context.Background()
	logger := log.NewEntry(log.StandardLogger())
	repoBuilder := services.RepoBuilder{
		Service:      services.NewService(ctx, logger),
		FilesService: mockFilesService,
		Log:          logger,
	}
	orgID := faker.UUIDHyphenated()
	update := &models.UpdateTransaction{
		OrgID: orgID,
		Repo:  &models.Repo{},
		Commit: &models.Commit{
			OrgID:            orgID,
			OSTreeCommit:     faker.UUIDHyphenated(),
			ExternalURL:      false,
			ImageBuildTarURL: faker.URL(),
			OSTreeRef:        config.OstreeRefRHEL9,
		},
		OldCommits: []models.Commit{
			{
				OrgID:            orgID,
				OSTreeCommit:     faker.UUIDHyphenated(),
				ImageBuildTarURL: faker.URL(),
				ExternalURL:      false,
				OSTreeRef:        config.OstreeRefRHEL9,
			},
		},
	}
	err = db.DB.Create(update).Error
	g.Expect(err).ToNot(HaveOccurred())

	updateWorkPath := filepath.Join(updateTempDirPath, strconv.FormatUint(uint64(update.ID), 10))
	updateRepoPath := filepath.Join(updateWorkPath, "repo")
	err = os.MkdirAll(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateRepoTarFilePath := filepath.Join(updateWorkPath, "repo.tar")
	_, err = os.Create(updateRepoTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitWorkPath := filepath.Join(updateWorkPath, "staging", update.OldCommits[0].OSTreeCommit)
	err = os.MkdirAll(updateOldCommitWorkPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitTarFilePath := filepath.Join(updateOldCommitWorkPath, "repo.tar")
	_, err = os.Create(updateOldCommitTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitRepoPath := filepath.Join(updateOldCommitWorkPath, "repo")
	expectedRepoURL := faker.URL()

	// GetDownloader should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetDownloader().Return(mockDownloader).Times(2)
	mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, updateRepoTarFilePath).Return(nil)
	mockDownloader.EXPECT().DownloadToPath(update.OldCommits[0].ImageBuildTarURL, updateOldCommitTarFilePath).Return(nil)

	// GetExtractor should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetExtractor().Return(mockExtractor).Times(2)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(nil)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateOldCommitWorkPath).Return(nil)

	mockFilesService.EXPECT().GetUploader().Return(mockUploader)
	mockUploader.EXPECT().UploadRepo(updateRepoPath, strconv.FormatUint(uint64(update.ID), 10), "private").Return(expectedRepoURL, nil)

	oldCommitRevision := update.OldCommits[0].OSTreeCommit
	updateCommitRevision := update.Commit.OSTreeCommit

	expectedExecCalls := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
		ExpectedCommand     string
		ExpectExecuted      bool
	}{
		{
			name:                "run ostree command rev-parse for update commit successfully",
			TestHelper:          NewMockTestExecHelper(t, updateCommitRevision, 0),
			ExpectedOutput:      updateCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, update.Commit.OSTreeRef),
		},
		{
			name:                "run ostree command rev-parse for old commit successfully",
			TestHelper:          NewMockTestExecHelper(t, oldCommitRevision, 0),
			ExpectedOutput:      oldCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateOldCommitRepoPath, update.OldCommits[0].OSTreeRef),
		},
		{
			name:                "run ostree command pull-local successfully",
			TestHelper:          NewMockTestExecHelper(t, "pull-local", 0),
			ExpectedOutput:      "pull-local",
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree pull-local --repo %s %s %s", updateRepoPath, updateOldCommitRepoPath, oldCommitRevision),
		},
		{
			name:                "run ostree command static-delta successfully",
			TestHelper:          NewMockTestExecHelper(t, "static-delta", 0),
			ExpectedOutput:      "static-delta",
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree static-delta generate --repo %s --from %s --to %s", updateRepoPath, oldCommitRevision, updateCommitRevision),
		},
	}
	// chain TestExecHelper, so that each mock can initiate the next exec command helper
	for ind := range expectedExecCalls {
		if ind < (len(expectedExecCalls) - 1) {
			expectedExecCalls[ind].TestHelper.Next = &expectedExecCalls[ind+1].TestHelper
		}
	}
	// set the first exec command helper mock
	services.BuildCommand = expectedExecCalls[0].TestHelper.MockExecCommand

	updateTransaction, err := repoBuilder.BuildUpdateRepo(update.ID)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(updateTransaction).ToNot(BeNil())
	g.Expect(updateTransaction.Repo.URL).To(Equal(expectedRepoURL))
	g.Expect(updateTransaction.Repo.Status).To(Equal(models.RepoStatusSuccess))

	// ensure all the commands was generated and run as expected
	for _, testCase := range expectedExecCalls {
		t.Run(testCase.name, func(t *testing.T) {
			g.Expect(testCase.TestHelper.Executed).To(BeTrue())
			g.Expect(testCase.TestHelper.ExistStatus).To(Equal(testCase.ExpectedExistStatus))
			g.Expect(testCase.TestHelper.Output).To(Equal(testCase.ExpectedOutput))
			g.Expect(testCase.TestHelper.Command).To(Equal(testCase.ExpectedCommand))
		})
	}
}

func TestBuildUpdateRepoWithOldCommitsStaticDeltaError(t *testing.T) {
	// should return error when old commit ostree static-delta fails
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	// enable oldCommits feature flag by setting the corresponding env variable
	err = os.Setenv(feature.BuildUpdateRepoWithOldCommits.EnvVar, "True")
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string) {
		// restore the standard command builder
		services.BuildCommand = exec.Command
		// restore the current dir path
		err = os.Chdir(dirPath)
		g.Expect(err).ToNot(HaveOccurred())
		// unset oldCommits feature flag
		err := os.Unsetenv(feature.BuildUpdateRepoWithOldCommits.EnvVar)
		g.Expect(err).ToNot(HaveOccurred())
	}(currentDir)

	initialRepoTempPath := config.Get().RepoTempPath
	repoTempPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_temp_test_%d", time.Now().Unix()))
	err = os.MkdirAll(repoTempPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	ctrl := gomock.NewController(t)

	defer func(mockController *gomock.Controller, createdRepoTempPath string, configRepoTempPath string) {
		mockController.Finish()
		config.Get().RepoTempPath = configRepoTempPath
		_ = os.RemoveAll(createdRepoTempPath)
	}(ctrl, repoTempPath, initialRepoTempPath)

	// set repo temp path to test predefined location
	config.Get().RepoTempPath = repoTempPath

	updateTempDirPath := filepath.Join(repoTempPath, "upd")
	err = os.MkdirAll(updateTempDirPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	mockFilesService := mock_services.NewMockFilesService(ctrl)
	mockUploader := mock_files.NewMockUploader(ctrl)
	mockExtractor := mock_files.NewMockExtractor(ctrl)
	mockDownloader := mock_services.NewMockDownloader(ctrl)
	ctx := context.Background()
	logger := log.NewEntry(log.StandardLogger())
	repoBuilder := services.RepoBuilder{
		Service:      services.NewService(ctx, logger),
		FilesService: mockFilesService,
		Log:          logger,
	}
	orgID := faker.UUIDHyphenated()
	update := &models.UpdateTransaction{
		OrgID: orgID,
		Repo:  &models.Repo{},
		Commit: &models.Commit{
			OrgID:            orgID,
			OSTreeCommit:     faker.UUIDHyphenated(),
			ExternalURL:      false,
			ImageBuildTarURL: faker.URL(),
			OSTreeRef:        config.OstreeRefRHEL9,
		},
		OldCommits: []models.Commit{
			{
				OrgID:            orgID,
				OSTreeCommit:     faker.UUIDHyphenated(),
				ImageBuildTarURL: faker.URL(),
				ExternalURL:      false,
				OSTreeRef:        config.OstreeRefRHEL9,
			},
		},
	}
	err = db.DB.Create(update).Error
	g.Expect(err).ToNot(HaveOccurred())

	updateWorkPath := filepath.Join(updateTempDirPath, strconv.FormatUint(uint64(update.ID), 10))
	updateRepoPath := filepath.Join(updateWorkPath, "repo")
	err = os.MkdirAll(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateRepoTarFilePath := filepath.Join(updateWorkPath, "repo.tar")
	_, err = os.Create(updateRepoTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitWorkPath := filepath.Join(updateWorkPath, "staging", update.OldCommits[0].OSTreeCommit)
	err = os.MkdirAll(updateOldCommitWorkPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitTarFilePath := filepath.Join(updateOldCommitWorkPath, "repo.tar")
	_, err = os.Create(updateOldCommitTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitRepoPath := filepath.Join(updateOldCommitWorkPath, "repo")
	expectedRepoURL := faker.URL()

	// GetDownloader should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetDownloader().Return(mockDownloader).Times(2)
	mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, updateRepoTarFilePath).Return(nil)
	mockDownloader.EXPECT().DownloadToPath(update.OldCommits[0].ImageBuildTarURL, updateOldCommitTarFilePath).Return(nil)

	// GetExtractor should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetExtractor().Return(mockExtractor).Times(2)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(nil)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateOldCommitWorkPath).Return(nil)

	// we do not expect the repo to be uploaded
	mockFilesService.EXPECT().GetUploader().Return(mockUploader).Times(0)
	mockUploader.EXPECT().UploadRepo(updateRepoPath, strconv.FormatUint(uint64(update.ID), 10), "private").Return(expectedRepoURL, nil).Times(0)

	oldCommitRevision := update.OldCommits[0].OSTreeCommit
	updateCommitRevision := update.Commit.OSTreeCommit

	expectedExecCalls := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
		ExpectedCommand     string
		ExpectExecuted      bool
	}{
		{
			name:                "run ostree command rev-parse for update commit successfully",
			TestHelper:          NewMockTestExecHelper(t, updateCommitRevision, 0),
			ExpectedOutput:      updateCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, update.Commit.OSTreeRef),
		},
		{
			name:                "run ostree command rev-parse for old commit successfully",
			TestHelper:          NewMockTestExecHelper(t, oldCommitRevision, 0),
			ExpectedOutput:      oldCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateOldCommitRepoPath, update.OldCommits[0].OSTreeRef),
		},
		{
			name:                "run ostree command pull-local successfully",
			TestHelper:          NewMockTestExecHelper(t, "pull-local", 0),
			ExpectedOutput:      "pull-local",
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree pull-local --repo %s %s %s", updateRepoPath, updateOldCommitRepoPath, oldCommitRevision),
		},
		{
			name:                "run ostree command static-delta successfully",
			TestHelper:          NewMockTestExecHelper(t, "static-delta", 1),
			ExpectedOutput:      "static-delta",
			ExpectedExistStatus: 1,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree static-delta generate --repo %s --from %s --to %s", updateRepoPath, oldCommitRevision, updateCommitRevision),
		},
	}
	// chain TestExecHelper, so that each mock can initiate the next exec command helper
	for ind := range expectedExecCalls {
		if ind < (len(expectedExecCalls) - 1) {
			expectedExecCalls[ind].TestHelper.Next = &expectedExecCalls[ind+1].TestHelper
		}
	}
	// set the first exec command helper mock
	services.BuildCommand = expectedExecCalls[0].TestHelper.MockExecCommand

	_, err = repoBuilder.BuildUpdateRepo(update.ID)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(fmt.Sprintf("exit status %d", expectedExecCalls[3].ExpectedExistStatus)))

	// ensure all the commands was generated and run as expected
	for _, testCase := range expectedExecCalls {
		t.Run(testCase.name, func(t *testing.T) {
			g.Expect(testCase.TestHelper.Executed).To(BeTrue())
			g.Expect(testCase.TestHelper.ExistStatus).To(Equal(testCase.ExpectedExistStatus))
			g.Expect(testCase.TestHelper.Output).To(Equal(testCase.ExpectedOutput))
			g.Expect(testCase.TestHelper.Command).To(Equal(testCase.ExpectedCommand))
		})
	}
}

func TestBuildUpdateRepoWithOldCommitsExtractVersionRepoError(t *testing.T) {
	// should return error when old commit ExtractVersionRepo fails
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	// enable oldCommits feature flag by setting the corresponding env variable
	err = os.Setenv(feature.BuildUpdateRepoWithOldCommits.EnvVar, "True")
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string) {
		// restore the current dir path
		err = os.Chdir(dirPath)
		g.Expect(err).ToNot(HaveOccurred())
		// unset oldCommits feature flag
		err := os.Unsetenv(feature.BuildUpdateRepoWithOldCommits.EnvVar)
		g.Expect(err).ToNot(HaveOccurred())
	}(currentDir)

	initialRepoTempPath := config.Get().RepoTempPath
	repoTempPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_temp_test_%d", time.Now().Unix()))
	err = os.MkdirAll(repoTempPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	ctrl := gomock.NewController(t)

	defer func(mockController *gomock.Controller, createdRepoTempPath string, configRepoTempPath string) {
		mockController.Finish()
		config.Get().RepoTempPath = configRepoTempPath
		_ = os.RemoveAll(createdRepoTempPath)
	}(ctrl, repoTempPath, initialRepoTempPath)

	// set repo temp path to test predefined location
	config.Get().RepoTempPath = repoTempPath

	updateTempDirPath := filepath.Join(repoTempPath, "upd")
	err = os.MkdirAll(updateTempDirPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	mockFilesService := mock_services.NewMockFilesService(ctrl)
	mockUploader := mock_files.NewMockUploader(ctrl)
	mockExtractor := mock_files.NewMockExtractor(ctrl)
	mockDownloader := mock_services.NewMockDownloader(ctrl)
	ctx := context.Background()
	logger := log.NewEntry(log.StandardLogger())
	repoBuilder := services.RepoBuilder{
		Service:      services.NewService(ctx, logger),
		FilesService: mockFilesService,
		Log:          logger,
	}
	orgID := faker.UUIDHyphenated()
	update := &models.UpdateTransaction{
		OrgID: orgID,
		Repo:  &models.Repo{},
		Commit: &models.Commit{
			OrgID:            orgID,
			OSTreeCommit:     faker.UUIDHyphenated(),
			ExternalURL:      false,
			ImageBuildTarURL: faker.URL(),
			OSTreeRef:        config.OstreeRefRHEL9,
		},
		OldCommits: []models.Commit{
			{
				OrgID:            orgID,
				OSTreeCommit:     faker.UUIDHyphenated(),
				ImageBuildTarURL: faker.URL(),
				ExternalURL:      false,
				OSTreeRef:        config.OstreeRefRHEL9,
			},
		},
	}
	err = db.DB.Create(update).Error
	g.Expect(err).ToNot(HaveOccurred())

	updateWorkPath := filepath.Join(updateTempDirPath, strconv.FormatUint(uint64(update.ID), 10))
	updateRepoPath := filepath.Join(updateWorkPath, "repo")
	err = os.MkdirAll(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateRepoTarFilePath := filepath.Join(updateWorkPath, "repo.tar")
	_, err = os.Create(updateRepoTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitWorkPath := filepath.Join(updateWorkPath, "staging", update.OldCommits[0].OSTreeCommit)
	err = os.MkdirAll(updateOldCommitWorkPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitTarFilePath := filepath.Join(updateOldCommitWorkPath, "repo.tar")
	_, err = os.Create(updateOldCommitTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	expectedOldCommitRepoExtractError := errors.New("OldCommit ExtractVersionRepo error occurred")

	// GetDownloader should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetDownloader().Return(mockDownloader).Times(2)
	mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, updateRepoTarFilePath).Return(nil)
	mockDownloader.EXPECT().DownloadToPath(update.OldCommits[0].ImageBuildTarURL, updateOldCommitTarFilePath).Return(nil)

	// GetExtractor should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetExtractor().Return(mockExtractor).Times(2)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(nil)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateOldCommitWorkPath).Return(expectedOldCommitRepoExtractError)

	// we do not expect the repo to be uploaded
	mockFilesService.EXPECT().GetUploader().Return(mockUploader).Times(0)
	mockUploader.EXPECT().UploadRepo(updateRepoPath, strconv.FormatUint(uint64(update.ID), 10), "private").Times(0)

	// we do not expect any command to be run

	_, err = repoBuilder.BuildUpdateRepo(update.ID)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(Equal(expectedOldCommitRepoExtractError))
}

func TestBuildUpdateRepoWithOldCommitsDownloadVersionRepoError(t *testing.T) {
	// should return error when ExtractVersionRepo fails
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	g.Expect(err).ToNot(HaveOccurred())
	// enable oldCommits feature flag by setting the corresponding env variable
	err = os.Setenv(feature.BuildUpdateRepoWithOldCommits.EnvVar, "True")
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string) {
		// restore the current dir path
		err = os.Chdir(dirPath)
		g.Expect(err).ToNot(HaveOccurred())
		// unset oldCommits feature flag
		err := os.Unsetenv(feature.BuildUpdateRepoWithOldCommits.EnvVar)
		g.Expect(err).ToNot(HaveOccurred())
	}(currentDir)

	initialRepoTempPath := config.Get().RepoTempPath
	repoTempPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_temp_test_%d", time.Now().Unix()))
	err = os.MkdirAll(repoTempPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	ctrl := gomock.NewController(t)

	defer func(mockController *gomock.Controller, createdRepoTempPath string, configRepoTempPath string) {
		mockController.Finish()
		config.Get().RepoTempPath = configRepoTempPath
		_ = os.RemoveAll(createdRepoTempPath)
	}(ctrl, repoTempPath, initialRepoTempPath)

	// set repo temp path to test predefined location
	config.Get().RepoTempPath = repoTempPath

	updateTempDirPath := filepath.Join(repoTempPath, "upd")
	err = os.MkdirAll(updateTempDirPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	mockFilesService := mock_services.NewMockFilesService(ctrl)
	mockUploader := mock_files.NewMockUploader(ctrl)
	mockExtractor := mock_files.NewMockExtractor(ctrl)
	mockDownloader := mock_services.NewMockDownloader(ctrl)
	ctx := context.Background()
	logger := log.NewEntry(log.StandardLogger())
	repoBuilder := services.RepoBuilder{
		Service:      services.NewService(ctx, logger),
		FilesService: mockFilesService,
		Log:          logger,
	}
	orgID := faker.UUIDHyphenated()
	update := &models.UpdateTransaction{
		OrgID: orgID,
		Repo:  &models.Repo{},
		Commit: &models.Commit{
			OrgID:            orgID,
			OSTreeCommit:     faker.UUIDHyphenated(),
			ExternalURL:      false,
			ImageBuildTarURL: faker.URL(),
			OSTreeRef:        config.OstreeRefRHEL9,
		},
		OldCommits: []models.Commit{
			{
				OrgID:            orgID,
				OSTreeCommit:     faker.UUIDHyphenated(),
				ImageBuildTarURL: faker.URL(),
				ExternalURL:      false,
				OSTreeRef:        config.OstreeRefRHEL9,
			},
		},
	}
	err = db.DB.Create(update).Error
	g.Expect(err).ToNot(HaveOccurred())

	updateWorkPath := filepath.Join(updateTempDirPath, strconv.FormatUint(uint64(update.ID), 10))
	updateRepoPath := filepath.Join(updateWorkPath, "repo")
	err = os.MkdirAll(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateRepoTarFilePath := filepath.Join(updateWorkPath, "repo.tar")
	_, err = os.Create(updateRepoTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitWorkPath := filepath.Join(updateWorkPath, "staging", update.OldCommits[0].OSTreeCommit)
	err = os.MkdirAll(updateOldCommitWorkPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())

	updateOldCommitTarFilePath := filepath.Join(updateOldCommitWorkPath, "repo.tar")
	_, err = os.Create(updateOldCommitTarFilePath)
	g.Expect(err).ToNot(HaveOccurred())

	expectedOldCommitRepoDownloadError := errors.New("OldCommit DownloadVersionRepo error occurred")

	// GetDownloader should be called two time , one time with the update target commit and one time with old commit
	mockFilesService.EXPECT().GetDownloader().Return(mockDownloader).Times(2)
	mockDownloader.EXPECT().DownloadToPath(update.Commit.ImageBuildTarURL, updateRepoTarFilePath).Return(nil)
	mockDownloader.EXPECT().DownloadToPath(update.OldCommits[0].ImageBuildTarURL, updateOldCommitTarFilePath).Return(expectedOldCommitRepoDownloadError)

	// GetExtractor should be called one time with the update target commit, and should not be called with old commit
	mockFilesService.EXPECT().GetExtractor().Return(mockExtractor).Times(1)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateWorkPath).Return(nil)
	mockExtractor.EXPECT().Extract(gomock.AssignableToTypeOf(&os.File{}), updateOldCommitWorkPath).Times(0)

	// we do not expect the repo to be uploaded
	mockFilesService.EXPECT().GetUploader().Return(mockUploader).Times(0)
	mockUploader.EXPECT().UploadRepo(updateRepoPath, strconv.FormatUint(uint64(update.ID), 10), "private").Times(0)

	// we do not expect any exec.Command to be run

	_, err = repoBuilder.BuildUpdateRepo(update.ID)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring(expectedOldCommitRepoDownloadError.Error()))
}

func TestRepoRevParse(t *testing.T) {
	g := NewGomegaWithT(t)
	defer func() {
		services.BuildCommand = exec.Command
	}()
	ExpectedCommitID := faker.UUIDHyphenated()
	testCases := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
	}{
		{
			name:                "should run command successfully",
			TestHelper:          NewMockTestExecHelper(t, ExpectedCommitID, 0),
			ExpectedOutput:      ExpectedCommitID,
			ExpectedExistStatus: 0,
		},
		{
			name:                "should return error when command failed",
			TestHelper:          NewMockTestExecHelper(t, faker.UUIDHyphenated(), 1),
			ExpectedOutput:      "",
			ExpectedExistStatus: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			services.BuildCommand = testCase.TestHelper.MockExecCommand
			path := faker.UUIDHyphenated()
			ref := faker.UUIDHyphenated()
			ExpectedCommand := fmt.Sprintf("ostree rev-parse --repo %s %s", path, ref)

			commit, err := services.RepoRevParse(path, ref)

			g.Expect(testCase.TestHelper.Command).To(Equal(ExpectedCommand))
			if testCase.ExpectedExistStatus == 0 {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
			}
			g.Expect(commit).To(Equal(testCase.ExpectedOutput))
		})
	}
}

func TestRepoPullLocalStaticDeltas(t *testing.T) {
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	currentCommandBuilder := services.BuildCommand
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string, commandBuilder func(name string, arg ...string) *exec.Cmd) {
		// restore the initial command builder
		services.BuildCommand = commandBuilder
		// restore the initial directory
		_ = os.Chdir(dirPath)
	}(currentDir, currentCommandBuilder)

	ctx := context.Background()
	RepoBuilder := services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	updateCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}
	oldCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}

	updateRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_update_test_%d", time.Now().Unix()))
	err = os.Mkdir(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	defer func(dirPath string) {
		_ = os.RemoveAll(dirPath)
	}(updateRepoPath)

	oldRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_old_test_%d", time.Now().Unix()))

	updateCommitRevision := faker.UUIDHyphenated()
	oldCommitRevision := faker.UUIDHyphenated()

	expectedCallsCases := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
		ExpectedCommand     string
		ExpectExecuted      bool
	}{
		{
			name:                "should run ostree command rev-parse for update commit successfully",
			TestHelper:          NewMockTestExecHelper(t, updateCommitRevision, 0),
			ExpectedOutput:      updateCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, updateCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command rev-parse for old commit successfully",
			TestHelper:          NewMockTestExecHelper(t, oldCommitRevision, 0),
			ExpectedOutput:      oldCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", oldRepoPath, oldCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command pull-local successfully",
			TestHelper:          NewMockTestExecHelper(t, "pull-local", 0),
			ExpectedOutput:      "pull-local",
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree pull-local --repo %s %s %s", updateRepoPath, oldRepoPath, oldCommitRevision),
		},
		{
			name:                "should run ostree command static-delta successfully",
			TestHelper:          NewMockTestExecHelper(t, "static-delta", 0),
			ExpectedOutput:      "static-delta",
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree static-delta generate --repo %s --from %s --to %s", updateRepoPath, oldCommitRevision, updateCommitRevision),
		},
	}
	// chain TestHelper, so that each mock can initiate the next exec command helper
	for ind := range expectedCallsCases {
		if ind < (len(expectedCallsCases) - 1) {
			expectedCallsCases[ind].TestHelper.Next = &expectedCallsCases[ind+1].TestHelper
		}
	}
	// set the first exec command helper mock
	services.BuildCommand = expectedCallsCases[0].TestHelper.MockExecCommand

	err = RepoBuilder.RepoPullLocalStaticDeltas(&updateCommit, &oldCommit, updateRepoPath, oldRepoPath)
	g.Expect(err).ToNot(HaveOccurred())

	for _, testCase := range expectedCallsCases {
		t.Run(testCase.name, func(t *testing.T) {
			g.Expect(testCase.TestHelper.Executed).To(BeTrue())
			g.Expect(testCase.TestHelper.ExistStatus).To(Equal(testCase.ExpectedExistStatus))
			g.Expect(testCase.TestHelper.Output).To(Equal(testCase.ExpectedOutput))
			if testCase.ExpectedCommand != "" {
				g.Expect(testCase.TestHelper.Command).To(Equal(testCase.ExpectedCommand))
			}
		})
	}
}

func TestRepoPullLocalStaticDeltasUpdatePathDoesNotExist(t *testing.T) {
	g := NewGomegaWithT(t)
	updateRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_update_test_%d", time.Now().Unix()))

	oldRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_old_test_%d", time.Now().Unix()))
	ctx := context.Background()
	RepoBuilder := services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	updateCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}
	oldCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}

	err := RepoBuilder.RepoPullLocalStaticDeltas(&updateCommit, &oldCommit, updateRepoPath, oldRepoPath)
	g.Expect(err).To(HaveOccurred())
	expectedErrorMessage := fmt.Sprintf("chdir %s: no such file or directory", updateRepoPath)
	g.Expect(err.Error()).To(Equal(expectedErrorMessage))
}

func TestRepoPullLocalStaticDeltasFailsWhenUpdateRevisionFail(t *testing.T) {
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	currentCommandBuilder := services.BuildCommand
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string, commandBuilder func(name string, arg ...string) *exec.Cmd) {
		// restore the initial command builder
		services.BuildCommand = commandBuilder
		// restore the initial directory
		_ = os.Chdir(dirPath)
	}(currentDir, currentCommandBuilder)

	ctx := context.Background()
	RepoBuilder := services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	updateCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}
	oldCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}

	updateRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_update_test_%d", time.Now().Unix()))
	err = os.Mkdir(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	defer func(dirPath string) {
		_ = os.RemoveAll(dirPath)
	}(updateRepoPath)
	oldRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_old_test_%d", time.Now().Unix()))

	testExecHelper := NewMockTestExecHelper(t, "", 1)

	// set the exec command helper mock
	services.BuildCommand = testExecHelper.MockExecCommand
	expectedCommand := fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, updateCommit.OSTreeRef)

	err = RepoBuilder.RepoPullLocalStaticDeltas(&updateCommit, &oldCommit, updateRepoPath, oldRepoPath)
	g.Expect(err).To(HaveOccurred())
	expectedErrorMessage := fmt.Sprintf("exit status %d", testExecHelper.ExistStatus)
	g.Expect(err.Error()).To(Equal(expectedErrorMessage))
	g.Expect(testExecHelper.Command).To(Equal(expectedCommand))
}

func TestRepoPullLocalStaticDeltasFailsWhenOldRevisionFail(t *testing.T) {
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	currentCommandBuilder := services.BuildCommand
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string, commandBuilder func(name string, arg ...string) *exec.Cmd) {
		// restore the initial command builder
		services.BuildCommand = commandBuilder
		// restore the initial directory
		_ = os.Chdir(dirPath)
	}(currentDir, currentCommandBuilder)

	ctx := context.Background()
	RepoBuilder := services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	updateCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}
	oldCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}

	updateRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_update_test_%d", time.Now().Unix()))
	err = os.Mkdir(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	defer func(dirPath string) {
		_ = os.RemoveAll(dirPath)
	}(updateRepoPath)

	oldRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_old_test_%d", time.Now().Unix()))

	updateCommitRevision := faker.UUIDHyphenated()
	expectedCallsCases := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
		ExpectedCommand     string
		ExpectExecuted      bool
	}{
		{
			name:                "should run ostree command rev-parse for update commit successfully",
			TestHelper:          NewMockTestExecHelper(t, updateCommitRevision, 0),
			ExpectedOutput:      updateCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, updateCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command rev-parse for old commit successfully",
			TestHelper:          NewMockTestExecHelper(t, "", 1),
			ExpectedOutput:      "",
			ExpectedExistStatus: 1,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", oldRepoPath, oldCommit.OSTreeRef),
		},
	}
	// chain TestHelper, so that each mock can initiate the next exec command helper
	for ind := range expectedCallsCases {
		if ind < (len(expectedCallsCases) - 1) {
			expectedCallsCases[ind].TestHelper.Next = &expectedCallsCases[ind+1].TestHelper
		}
	}
	// set the first exec command helper mock
	services.BuildCommand = expectedCallsCases[0].TestHelper.MockExecCommand

	expectedErrorMessage := fmt.Sprintf("exit status %d", expectedCallsCases[1].TestHelper.ExistStatus)

	err = RepoBuilder.RepoPullLocalStaticDeltas(&updateCommit, &oldCommit, updateRepoPath, oldRepoPath)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(expectedErrorMessage))

	for _, testCase := range expectedCallsCases {
		t.Run(testCase.name, func(t *testing.T) {
			g.Expect(testCase.TestHelper.Executed).To(BeTrue())
			g.Expect(testCase.TestHelper.ExistStatus).To(Equal(testCase.ExpectedExistStatus))
			g.Expect(testCase.TestHelper.Output).To(Equal(testCase.ExpectedOutput))
			if testCase.ExpectedCommand != "" {
				g.Expect(testCase.TestHelper.Command).To(Equal(testCase.ExpectedCommand))
			}
		})
	}
}

func TestRepoPullLocalStaticDeltasFailsWhenPullLocalFail(t *testing.T) {
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	currentCommandBuilder := services.BuildCommand
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string, commandBuilder func(name string, arg ...string) *exec.Cmd) {
		// restore the initial command builder
		services.BuildCommand = commandBuilder
		// restore the initial directory
		_ = os.Chdir(dirPath)
	}(currentDir, currentCommandBuilder)

	ctx := context.Background()
	RepoBuilder := services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	updateCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}
	oldCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}

	updateRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_update_test_%d", time.Now().Unix()))
	err = os.Mkdir(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	defer func(dirPath string) {
		_ = os.RemoveAll(dirPath)
	}(updateRepoPath)

	oldRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_old_test_%d", time.Now().Unix()))

	updateCommitRevision := faker.UUIDHyphenated()
	oldCommitRevision := faker.UUIDHyphenated()

	expectedCallsCases := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
		ExpectedCommand     string
		ExpectExecuted      bool
	}{
		{
			name:                "should run ostree command rev-parse for update commit successfully",
			TestHelper:          NewMockTestExecHelper(t, updateCommitRevision, 0),
			ExpectedOutput:      updateCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, updateCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command rev-parse for old commit successfully",
			TestHelper:          NewMockTestExecHelper(t, oldCommitRevision, 0),
			ExpectedOutput:      oldCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", oldRepoPath, oldCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command pull-local successfully",
			TestHelper:          NewMockTestExecHelper(t, "pull-local", 2),
			ExpectedOutput:      "pull-local",
			ExpectedExistStatus: 2,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree pull-local --repo %s %s %s", updateRepoPath, oldRepoPath, oldCommitRevision),
		},
	}
	// chain TestHelper, so that each mock can initiate the next exec command helper
	for ind := range expectedCallsCases {
		if ind < (len(expectedCallsCases) - 1) {
			expectedCallsCases[ind].TestHelper.Next = &expectedCallsCases[ind+1].TestHelper
		}
	}
	// set the first exec command helper mock
	services.BuildCommand = expectedCallsCases[0].TestHelper.MockExecCommand
	expectedErrorMessage := fmt.Sprintf("exit status %d", expectedCallsCases[2].TestHelper.ExistStatus)

	err = RepoBuilder.RepoPullLocalStaticDeltas(&updateCommit, &oldCommit, updateRepoPath, oldRepoPath)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(expectedErrorMessage))

	for _, testCase := range expectedCallsCases {
		t.Run(testCase.name, func(t *testing.T) {
			g.Expect(testCase.TestHelper.Executed).To(BeTrue())
			g.Expect(testCase.TestHelper.ExistStatus).To(Equal(testCase.ExpectedExistStatus))
			g.Expect(testCase.TestHelper.Output).To(Equal(testCase.ExpectedOutput))
			if testCase.ExpectedCommand != "" {
				g.Expect(testCase.TestHelper.Command).To(Equal(testCase.ExpectedCommand))
			}
		})
	}
}

func TestRepoPullLocalStaticDeltasFailsWhenStaticDeltaFails(t *testing.T) {
	g := NewGomegaWithT(t)
	currentDir, err := os.Getwd()
	currentCommandBuilder := services.BuildCommand
	g.Expect(err).ToNot(HaveOccurred())

	defer func(dirPath string, commandBuilder func(name string, arg ...string) *exec.Cmd) {
		// restore the initial command builder
		services.BuildCommand = commandBuilder
		// restore the initial directory
		_ = os.Chdir(dirPath)
	}(currentDir, currentCommandBuilder)

	ctx := context.Background()
	RepoBuilder := services.NewRepoBuilder(ctx, log.NewEntry(log.StandardLogger()))
	updateCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}
	oldCommit := models.Commit{OSTreeRef: faker.UUIDHyphenated()}

	updateRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_update_test_%d", time.Now().Unix()))
	err = os.Mkdir(updateRepoPath, 0755)
	g.Expect(err).ToNot(HaveOccurred())
	defer func(dirPath string) {
		_ = os.RemoveAll(dirPath)
	}(updateRepoPath)

	oldRepoPath := filepath.Join(os.TempDir(), fmt.Sprintf("repo_old_test_%d", time.Now().Unix()))

	updateCommitRevision := faker.UUIDHyphenated()
	oldCommitRevision := faker.UUIDHyphenated()

	expectedCallsCases := []struct {
		name                string
		TestHelper          MockTestExecHelper
		ExpectedOutput      string
		ExpectedExistStatus int
		ExpectedCommand     string
		ExpectExecuted      bool
	}{
		{
			name:                "should run ostree command rev-parse for update commit successfully",
			TestHelper:          NewMockTestExecHelper(t, updateCommitRevision, 0),
			ExpectedOutput:      updateCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", updateRepoPath, updateCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command rev-parse for old commit successfully",
			TestHelper:          NewMockTestExecHelper(t, oldCommitRevision, 0),
			ExpectedOutput:      oldCommitRevision,
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("ostree rev-parse --repo %s %s", oldRepoPath, oldCommit.OSTreeRef),
		},
		{
			name:                "should run ostree command pull-local successfully",
			TestHelper:          NewMockTestExecHelper(t, "pull-local", 0),
			ExpectedOutput:      "pull-local",
			ExpectedExistStatus: 0,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree pull-local --repo %s %s %s", updateRepoPath, oldRepoPath, oldCommitRevision),
		},
		{
			name:                "should run ostree command static-delta successfully",
			TestHelper:          NewMockTestExecHelper(t, "static-delta", 3),
			ExpectedOutput:      "static-delta",
			ExpectedExistStatus: 3,
			ExpectedCommand:     fmt.Sprintf("/usr/bin/ostree static-delta generate --repo %s --from %s --to %s", updateRepoPath, oldCommitRevision, updateCommitRevision),
		},
	}
	// chain TestHelper, so that each mock can initiate the next exec command helper
	for ind := range expectedCallsCases {
		if ind < (len(expectedCallsCases) - 1) {
			expectedCallsCases[ind].TestHelper.Next = &expectedCallsCases[ind+1].TestHelper
		}
	}
	// set the first exec command helper mock
	services.BuildCommand = expectedCallsCases[0].TestHelper.MockExecCommand
	expectedErrorMessage := fmt.Sprintf("exit status %d", expectedCallsCases[3].TestHelper.ExistStatus)

	err = RepoBuilder.RepoPullLocalStaticDeltas(&updateCommit, &oldCommit, updateRepoPath, oldRepoPath)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal(expectedErrorMessage))

	for _, testCase := range expectedCallsCases {
		t.Run(testCase.name, func(t *testing.T) {
			g.Expect(testCase.TestHelper.Executed).To(BeTrue())
			g.Expect(testCase.TestHelper.ExistStatus).To(Equal(testCase.ExpectedExistStatus))
			g.Expect(testCase.TestHelper.Output).To(Equal(testCase.ExpectedOutput))
			if testCase.ExpectedCommand != "" {
				g.Expect(testCase.TestHelper.Command).To(Equal(testCase.ExpectedCommand))
			}
		})
	}
}

// func NewMockTestExecHelper(t *testing.T, output string, exitStatus int) MockTestExecHelper {
func NewMockTestExecHelper(t *testing.T, output string, exitStatus int) MockTestExecHelper {
	return MockTestExecHelper{Test: t, Output: output, ExistStatus: exitStatus, ExecuteOnlyOnce: true}
}

type MockTestExecHelper struct {
	Test                *testing.T
	Output              string
	ExistStatus         int
	Next                *MockTestExecHelper
	Command             string
	LastExecutedCommand string
	Executed            bool
	ExecuteOnlyOnce     bool
}

// MockExecCommand this will be executed instead of exec.Command
// this will replace the real command with our own "TestProcessHelper"
func (th *MockTestExecHelper) MockExecCommand(command string, args ...string) *exec.Cmd {
	originalCommand := []string{command}
	originalCommand = append(originalCommand, args...)
	th.Command = strings.Join(originalCommand, " ")
	if th.ExecuteOnlyOnce && th.Executed {
		th.Test.Fatalf("MockTestExecHelper executed command: %s , Last command: %s", th.Command, th.LastExecutedCommand)
	}
	cs := []string{"-test.run=TestProcessHelper", "--", th.Command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...) // nolint: gosec
	cmd.Env = []string{
		"TEST_PROCESS_HELPER=1",
		"STDOUT=" + th.Output,
		"EXIT_STATUS=" + strconv.Itoa(th.ExistStatus),
	}
	th.Executed = true
	th.LastExecutedCommand = th.Command
	if th.Next != nil {
		// if Next set the next mock Build command
		services.BuildCommand = th.Next.MockExecCommand
	}
	return cmd
}

// TestProcessHelper this will be executed in its own process instead of the real command
func TestProcessHelper(_ *testing.T) {
	if os.Getenv("TEST_PROCESS_HELPER") != "1" {
		return
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s", os.Getenv("STDOUT"))
	i, _ := strconv.Atoi(os.Getenv("EXIT_STATUS"))
	os.Exit(i)
}

func createTarball(tarballFilePath string, filePath string) error {
	file, err := os.Create(tarballFilePath)
	if err != nil {
		return fmt.Errorf("could not create tarball file '%s', got error '%s'", tarballFilePath, err.Error())
	}
	defer file.Close()

	tarWriter := tar.NewWriter(file)
	defer tarWriter.Close()

	err = addFileToTarWriter(filePath, tarWriter)
	if err != nil {
		return fmt.Errorf("could not add file '%s', to tarball, got error '%s'", filePath, err.Error())
	}

	return nil
}

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open file '%s', got error '%s'", filePath, err.Error())
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("could not get stat for file '%s', got error '%s'", filePath, err.Error())
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return fmt.Errorf("could not write header for file '%s', got error '%s'", filePath, err.Error())
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return fmt.Errorf("could not copy the file '%s' data to the tarball, got error '%s'", filePath, err.Error())
	}

	return nil
}

func createTestFile(filePath string) (string, error) {
	testFilePath := filepath.Join(filePath, testFile)
	f, err := os.Create(testFilePath)
	if err != nil {
		return "", fmt.Errorf("could not create the file on '%s', got error '%s'", filePath, err.Error())
	}
	defer f.Close()

	_, err = f.WriteString("Some content to test")
	if err != nil {
		return "", fmt.Errorf("could not write on the file '%s', got error '%s'", filePath, err.Error())
	}

	return testFilePath, nil
}

func readTestFile(filePath string) (string, error) {
	testFilePath := filepath.Join(filePath, testFile)
	f, err := os.Open(testFilePath)
	if err != nil {
		return "", fmt.Errorf("could not open the file on '%s', got error '%s'", filePath, err.Error())
	}
	defer f.Close()

	r := bufio.NewReader(f)
	fileContent, _, err := r.ReadLine()
	if err != nil {
		return "", fmt.Errorf("could not read the file '%s', got error '%s'", filePath, err.Error())
	}

	return string(fileContent), nil
}
