package storage_test

import (
	"errors"
	"strings"
	"time"

	"github.com/redhatinsights/edge-api/cmd/cleanup/storage"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Storage", func() {

	Context("GetPathFromURL", func() {
		It("should return url path", func() {
			expectedPath := "/example/repo"
			url := "https://repos.example.com" + expectedPath
			urlPath, err := storage.GetPathFromURL(url)
			Expect(err).ToNot(HaveOccurred())
			Expect(urlPath).To(Equal(expectedPath))
		})

		It("should return error when url fails to be parsed", func() {
			expectedPath := "/example/repo"
			url := "https\t://repos.example.com\n" + expectedPath
			_, err := storage.GetPathFromURL(url)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid control character in URL"))
		})
	})

	Context("AWS storage", func() {
		var ctrl *gomock.Controller
		var s3Client *files.S3Client
		var s3ClientAPI *mock_files.MockS3ClientAPI
		var s3FolderDeleter *mock_files.MockBatchFolderDeleterAPI
		var initialTimeDuration time.Duration
		var configDeleteAttempts int

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			s3ClientAPI = mock_files.NewMockS3ClientAPI(ctrl)
			s3FolderDeleter = mock_files.NewMockBatchFolderDeleterAPI(ctrl)
			initialTimeDuration = storage.DefaultTimeDuration
			storage.DefaultTimeDuration = 1 * time.Millisecond
			configDeleteAttempts = int(config.Get().DeleteFilesAttempts)
			s3Client = &files.S3Client{
				Client:        s3ClientAPI,
				FolderDeleter: s3FolderDeleter,
			}
		})

		AfterEach(func() {
			ctrl.Finish()
			storage.DefaultTimeDuration = initialTimeDuration
		})

		It("should delete aws s3 folder", func() {
			folderPath := "/test/folder/to/delete"
			s3FolderDeleter.EXPECT().Delete(config.Get().BucketName, strings.TrimPrefix(folderPath, "/")).Return(nil)
			err := storage.DeleteAWSFolder(s3Client, strings.TrimPrefix(folderPath, "/"))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when aws folder deleter returns error with all the attempts ", func() {
			folderPath := "/test/folder/to/delete"
			expectedError := errors.New("expected error returned by aws s3 folder deleter")
			// important to expect that this should be called cleanupimages.DefaultDeleteAttempts times
			s3FolderDeleter.EXPECT().Delete(
				config.Get().BucketName, strings.TrimPrefix(folderPath, "/"),
			).Return(expectedError).Times(configDeleteAttempts)
			err := storage.DeleteAWSFolder(s3Client, folderPath)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		})

		It("should not return error after a successful delete folder retry", func() {
			folderPath := "/test/folder/to/delete"
			expectedError := errors.New("expected error returned by aws s3 folder deleter")
			// expect that error was returned (cleanupimages.DefaultDeleteAttempts -1) times
			s3FolderDeleter.EXPECT().Delete(
				config.Get().BucketName, strings.TrimPrefix(folderPath, "/"),
			).Return(expectedError).Times(configDeleteAttempts - 1)
			// expect that the latest allowed time was a successful delete
			s3FolderDeleter.EXPECT().Delete(
				config.Get().BucketName, strings.TrimPrefix(folderPath, "/"),
			).Return(nil).Times(1)

			err := storage.DeleteAWSFolder(s3Client, folderPath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should delete aws s3 file", func() {
			filePath := "/test/file/to/delete"
			s3ClientAPI.EXPECT().DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(config.Get().BucketName),
				Key:    aws.String(filePath),
			}).Return(nil, nil)
			err := storage.DeleteAWSFile(s3Client, filePath)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when aws delete object returns error with all attempts", func() {
			filePath := "/test/file/to/delete"
			expectedError := errors.New("expected error returned by aws s3 file deleter")
			// important to expect that this should be called cleanupimages.DefaultDeleteAttempts times
			s3ClientAPI.EXPECT().DeleteObject(
				&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(filePath),
				}).Return(nil, expectedError).Times(configDeleteAttempts)
			err := storage.DeleteAWSFile(s3Client, filePath)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		})

		It("should not return error after a successful delete object retry", func() {
			filePath := "/test/file/to/delete"
			expectedError := errors.New("expected error returned by aws s3 file deleter")
			// expect that error was returned (cleanupimages.DefaultDeleteAttempts -1) times
			s3ClientAPI.EXPECT().DeleteObject(
				&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(filePath),
				}).Return(nil, expectedError).Times(configDeleteAttempts - 1)
			// expect that the latest allowed time was a successful delete
			s3ClientAPI.EXPECT().DeleteObject(
				&s3.DeleteObjectInput{
					Bucket: aws.String(config.Get().BucketName),
					Key:    aws.String(filePath),
				}).Return(nil, nil).Times(1)
			err := storage.DeleteAWSFile(s3Client, filePath)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
