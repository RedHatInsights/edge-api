// FIXME: golangci-lint
// nolint:revive,typecheck
package files_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Downloader Test", func() {
	var logEntry log.FieldLogger

	Describe("local downloader", func() {
		var downloader files.Downloader
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			config.Init()
			cfg := config.Get()
			cfg.Local = true
			downloader = files.NewDownloader(logEntry)
		})
		When("downloader is created", func() {
			It("return HTTPDownloader", func() {
				Expect(downloader).To(Not(BeNil()))
				_, ok := downloader.(*files.HTTPDownloader)
				Expect(ok).To(BeTrue())
			})
		})

	})

	Describe("s3 downloader", func() {
		var downloader files.Downloader
		cfg := config.Get()
		var initialLocal bool
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			config.Init()
			initialLocal = cfg.Local
			cfg.Local = false

			downloader = files.NewDownloader(logEntry)
		})
		AfterEach(func() {
			cfg.Local = initialLocal
		})
		When("downloader is created", func() {
			It("return S3Downloader", func() {
				Expect(downloader).To(Not(BeNil()))
				downloader, ok := downloader.(*files.S3Downloader)
				Expect(ok).To(BeTrue())
				Expect(downloader.Client).ToNot(BeNil())
			})
		})

		Context("S3Downloader", func() {
			var ctrl *gomock.Controller
			var s3Client *mock_files.MockS3ClientInterface
			var downloader files.Downloader

			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				s3Client = mock_files.NewMockS3ClientInterface(ctrl)
				downloader = files.NewS3Downloader(logEntry, s3Client)
			})
			AfterEach(func() {
				ctrl.Finish()
			})

			It("should DownloadToPath successfully", func() {
				remotePath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()
				fileName := fmt.Sprintf("downloader-%d.target-file", time.Now().UnixNano())
				targetFilePath := filepath.Join(os.TempDir(), fileName)

				defer func(filePath string) {
					os.Remove(filePath)
				}(targetFilePath)

				s3Client.EXPECT().Download(gomock.Any(), cfg.BucketName, remotePath).Return(int64(100), nil)

				err := downloader.DownloadToPath(remotePath, targetFilePath)
				Expect(err).ToNot(HaveOccurred())
				// ensure file exists
				info, err := os.Stat(targetFilePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(info.IsDir()).To(BeFalse())
			})

			It("should return error when remote path is invalid", func() {
				remotePath := faker.UUIDHyphenated() + "/\t" + faker.UUIDHyphenated()
				fileName := fmt.Sprintf("downloader-%d.target-file", time.Now().UnixNano())
				targetFilePath := filepath.Join(os.TempDir(), fileName)

				err := downloader.DownloadToPath(remotePath, targetFilePath)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid control character in URL"))
				// ensure file does not exist
				_, err = os.Stat(targetFilePath)
				Expect(err).To(HaveOccurred())
			})

			It("DownloadToPath should return error when s3client failed", func() {
				expectedError := errors.New("s3 download error")
				remotePath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()
				fileName := fmt.Sprintf("downloader-%d.target-file", time.Now().UnixNano())
				targetFilePath := filepath.Join(os.TempDir(), fileName)

				defer func(filePath string) {
					os.Remove(filePath)
				}(targetFilePath)

				s3Client.EXPECT().Download(gomock.Any(), cfg.BucketName, remotePath).Return(int64(0), expectedError)

				err := downloader.DownloadToPath(remotePath, targetFilePath)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})

			It("DownloadToPath should return error when file create failed", func() {
				remotePath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()
				fileName := fmt.Sprintf("downloader-%d.target-file", time.Now().UnixNano())
				targetFilePath := filepath.Join(os.TempDir(), fileName)
				// create a dir so that DownLoadToPath will not able to create a file with that name
				err := os.Mkdir(targetFilePath, os.ModePerm)
				Expect(err).ToNot(HaveOccurred())

				defer func(filePath string) {
					os.Remove(filePath)
				}(targetFilePath)

				// s3Client DowLoad should not be called
				s3Client.EXPECT().Download(gomock.Any(), gomock.Any(), gomock.Any()).Return(int64(0), nil).Times(0)

				err = downloader.DownloadToPath(remotePath, targetFilePath)
				Expect(err).To(HaveOccurred())
				expectedErrorMessage := fmt.Sprintf("open %s: is a directory", targetFilePath)
				Expect(err.Error()).To(ContainSubstring(expectedErrorMessage))
			})
		})
	})
})
