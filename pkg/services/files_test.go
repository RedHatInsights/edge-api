// FIXME: golangci-lint
// nolint:errcheck,revive,typecheck
package services_test

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/pkg/services"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"
)

var _ = Describe("File Service Test", func() {
	var logEntry log.FieldLogger
	Describe("local file service", func() {
		var service services.FilesService

		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			cfg := config.Get()
			cfg.Local = true
			service = services.NewFilesService(logEntry)
		})
		When("file service is created", func() {
			It("return service", func() {
				Expect(service).To(Not(BeNil()))
			})
		})
		When("get file", func() {
			var path, filename, data string
			BeforeEach(func() {
				data = "i am a file data"
				filename = "test"
				path = fmt.Sprintf("/tmp/%s", filename)
				f, err := os.Create(path)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()
				os.WriteFile(path, []byte(data), fs.ModeAppend)
			})
			AfterEach(func() {
				os.Remove(path)
			})

			It("returns file", func() {
				file, err := service.GetFile(filename)
				Expect(err).To(BeNil())

				b, err := io.ReadAll(file)
				Expect(err).To(BeNil())
				Expect(string(b)).To(Equal(data))
			})
		})
		When("GetSignedURL", func() {
			It("return the same URL", func() {
				url := faker.URL()
				signedURL, err := service.GetSignedURL(url)
				Expect(err).ToNot(HaveOccurred())
				Expect(signedURL).To(Equal(url))
			})
		})
	})
	Describe("aws file service", func() {
		var cfg *config.EdgeConfig
		var initialLocal bool
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			cfg = config.Get()
			initialLocal = cfg.Local
			cfg.Local = false
		})
		AfterEach(func() {
			cfg.Local = initialLocal
		})
		When("aws file service is created", func() {
			var service services.FilesService
			BeforeEach(func() {
				service = services.NewFilesService(logEntry)
			})
			It("return service", func() {
				Expect(service).To(Not(BeNil()))
				s3FilesService, ok := service.(*services.S3FilesService)
				Expect(ok).To(BeTrue())
				Expect(s3FilesService).ToNot(BeNil())
			})
		})

		Context("S3FilesService", func() {
			var ctrl *gomock.Controller
			var s3Client *mock_files.MockS3ClientInterface
			var s3FilesService *services.S3FilesService
			BeforeEach(func() {
				ctrl = gomock.NewController(GinkgoT())
				s3Client = mock_files.NewMockS3ClientInterface(ctrl)
				s3FilesService = &services.S3FilesService{
					Client: s3Client,
					Bucket: config.Get().BucketName,
				}

			})
			AfterEach(func() {
				ctrl.Finish()
			})

			Context("GetSignedURL", func() {
				It("should GetSignedURL successfully", func() {
					resourcePath := faker.URL()
					expectedSignedURL := faker.URL()

					s3Client.EXPECT().GetSignedURL(cfg.BucketName, resourcePath, 120*time.Minute).Return(expectedSignedURL, nil)

					signedURL, err := s3FilesService.GetSignedURL(resourcePath)
					Expect(err).ToNot(HaveOccurred())
					Expect(signedURL).To(Equal(expectedSignedURL))
				})

				It("GetSignedURL return error when client fail", func() {
					resourcePath := faker.URL()
					expectedError := errors.New("s3 GetSignedURL error")

					s3Client.EXPECT().GetSignedURL(cfg.BucketName, resourcePath, 120*time.Minute).Return("", expectedError)

					signedURL, err := s3FilesService.GetSignedURL(resourcePath)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(expectedError))
					Expect(signedURL).To(BeEmpty())
				})

				Context("GetFile", func() {
					It("should GetFile Successfully", func() {
						resourcePath := faker.URL()
						expectedFile := io.NopCloser(strings.NewReader("file content"))
						expectedClientOutput := &s3.GetObjectOutput{
							Body: expectedFile,
						}

						s3Client.EXPECT().GetObject(cfg.BucketName, resourcePath).Return(expectedClientOutput, nil)

						file, err := s3FilesService.GetFile(resourcePath)
						Expect(err).ToNot(HaveOccurred())
						Expect(file).To(Equal(expectedFile))
					})

					It("GetFile should return something wrong happened error for unknown error", func() {
						resourcePath := faker.URL()
						expectedError := errors.New("unknown s3 object error")

						s3Client.EXPECT().GetObject(cfg.BucketName, resourcePath).Return(nil, expectedError)

						_, err := s3FilesService.GetFile(resourcePath)
						Expect(err).To(HaveOccurred())
						Expect(err).To(Equal(expectedError))
					})

					It("GetFile should return something wrong happened error for unknown s3 GetObject error", func() {
						resourcePath := faker.URL()
						returnedError := awserr.New(s3.ErrCodeNoSuchUpload, "s3 object error out of GetFile context", nil)
						expectedErrorMessage := "something wrong happened while reading from the S3 bucket"

						s3Client.EXPECT().GetObject(cfg.BucketName, resourcePath).Return(nil, returnedError)

						_, err := s3FilesService.GetFile(resourcePath)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(expectedErrorMessage))
					})

					It("GetFile should return not found error for s3.ErrCodeNoSuchKey s3 GetObject error", func() {
						resourcePath := faker.URL()
						returnedError := awserr.New(s3.ErrCodeNoSuchKey, "s3 object not found", nil)
						expectedErrorMessage := fmt.Sprintf("the object %s was not found on the S3 bucket", resourcePath)

						s3Client.EXPECT().GetObject(cfg.BucketName, resourcePath).Return(nil, returnedError)

						_, err := s3FilesService.GetFile(resourcePath)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(expectedErrorMessage))
					})

					It("GetFile should return not found for s3.ErrCodeInvalidObjectState s3 GetObject error", func() {
						resourcePath := faker.URL()
						returnedError := awserr.New(s3.ErrCodeInvalidObjectState, "s3 object not found because of invalid state", nil)
						expectedErrorMessage := fmt.Sprintf("the object %s was not found on the S3 bucket because of an invalid state", resourcePath)

						s3Client.EXPECT().GetObject(cfg.BucketName, resourcePath).Return(nil, returnedError)

						_, err := s3FilesService.GetFile(resourcePath)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(Equal(expectedErrorMessage))
					})
				})
			})
		})
	})
})
