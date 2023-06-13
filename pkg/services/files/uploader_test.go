// FIXME: golangci-lint
// nolint:errcheck,revive,typecheck
package files_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"

	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Uploader Test", func() {
	var logEntry *log.Entry
	var account string
	var acl = "private"
	Describe("local uploader", func() {
		var uploader files.Uploader
		BeforeEach(func() {
			logEntry = log.NewEntry(log.StandardLogger())
			config.Init()
			cfg := config.Get()
			cfg.Local = true
			account = "000000"
			uploader = files.NewUploader(logEntry)
		})

		When("uploader is created", func() {
			It("return uploader", func() {
				Expect(uploader).To(Not(BeNil()))
			})

			It("should create LocalUploader", func() {
				_, ok := uploader.(*files.LocalUploader)
				Expect(ok).To(BeTrue())
			})
		})
		When("upload repo is called", func() {
			It("returns src and does nothing", func() {
				src := "/tmp/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account, acl)
				Expect(err).ToNot(HaveOccurred())
				Expect(uploadPath).To(Equal(src))
			})
		})
		When("upload file is called", func() {
			It("returns src and does nothing", func() {
				src := "/tmp/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account, acl)
				Expect(err).ToNot(HaveOccurred())
				Expect(uploadPath).To(Equal(src))
			})
		})
		When("base folder is invalid", func() {
			It("returns error", func() {
				src := "/invalid-base-folder/tmp-repo"
				uploadPath, err := uploader.UploadRepo(src, account, acl)
				Expect(err).To(HaveOccurred())
				Expect(uploadPath).To(Equal(""))
			})
		})
		When("upload file", func() {
			var path, filename string
			BeforeEach(func() {
				filename = "random-filename.txt"
				path = "/tmp"
				path = fmt.Sprintf("%s/%s", path, filename)
				f, err := os.Create(path)
				if err != nil {
					log.Fatal(err)
				}
				defer f.Close()
				os.Create(path)
			})
			AfterEach(func() {
				os.Remove(path)
			})
			It("doesnt returns error", func() {
				destfile := "random-file.txt"
				newFilePath, err := uploader.UploadFile(path, destfile)

				Expect(newFilePath).To(Equal(fmt.Sprintf("/tmp/%s", destfile)))
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("S3Uploader", func() {

		Context("create S3Uploader", func() {
			var initialLocal bool
			var uploader files.Uploader

			BeforeEach(func() {
				conf := config.Get()
				initialLocal = conf.Local
				conf.Local = false

				logEntry = log.NewEntry(log.StandardLogger())
				uploader = files.NewUploader(logEntry)
			})

			AfterEach(func() {
				config.Get().Local = initialLocal
			})

			It("should create S3Uploader", func() {
				Expect(uploader).To(Not(BeNil()))
				s3Uploader, ok := uploader.(*files.S3Uploader)
				Expect(ok).To(BeTrue())
				Expect(s3Uploader.Client).ToNot(BeNil())
				_, ok = (s3Uploader.Client).(*files.S3Client)
				Expect(ok).To(BeTrue())
				Expect(s3Uploader.Bucket).To(Equal(config.Get().BucketName))
			})
		})

		Context("S3Uploader", func() {
			var ctrl *gomock.Controller
			var s3Client *mock_files.MockS3ClientInterface
			var s3Uploader *files.S3Uploader
			acl := "private"
			conf := config.Get()
			orgID := common.DefaultOrgID
			var initialRepoTempPath string

			BeforeEach(func() {
				config.Init()
				logEntry = log.NewEntry(log.StandardLogger())
				ctrl = gomock.NewController(GinkgoT())
				s3Client = mock_files.NewMockS3ClientInterface(ctrl)
				s3Uploader = files.NewS3Uploader(logEntry, s3Client)
				initialRepoTempPath = conf.RepoTempPath
				// conf.RepoTempPath = filepath.Join(os.TempDir(), "uploader-"+faker.UUIDHyphenated())
				_ = os.Mkdir(conf.RepoTempPath, os.ModePerm)
				// Expect(err).ToNot(HaveOccurred())
			})

			AfterEach(func() {
				ctrl.Finish()
				// _ = os.RemoveAll(conf.RepoTempPath)
				conf.RepoTempPath = initialRepoTempPath
			})

			Context("UploadFile", func() {
				It("should UploadFile successfully ", func() {
					uploadPath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()

					file, err := os.CreateTemp(os.TempDir(), "uploader-*.source-file ")
					Expect(err).ToNot(HaveOccurred())

					defer func(file *os.File) {
						_ = file.Close()
						_ = os.Remove(file.Name())
					}(file)

					s3Client.EXPECT().PutObject(gomock.AssignableToTypeOf(&os.File{}), config.Get().BucketName, uploadPath, acl).Return(nil, nil)
					uploadURL, err := s3Uploader.UploadFile(file.Name(), uploadPath)
					Expect(err).ToNot(HaveOccurred())
					expectedURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Uploader.Bucket, conf.BucketRegion, uploadPath)
					Expect(uploadURL).To(Equal(expectedURL))
				})

				It("should UploadFileWithACL successfully with empty acl", func() {
					uploadPath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()

					file, err := os.CreateTemp(os.TempDir(), "uploader-*.source-file ")
					Expect(err).ToNot(HaveOccurred())

					defer func(file *os.File) {
						_ = file.Close()
						_ = os.Remove(file.Name())
					}(file)

					s3Client.EXPECT().PutObject(gomock.AssignableToTypeOf(&os.File{}), config.Get().BucketName, uploadPath, acl).Return(nil, nil)

					uploadURL, err := s3Uploader.UploadFileWithACL(file.Name(), uploadPath, "")
					Expect(err).ToNot(HaveOccurred())
					expectedURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s3Uploader.Bucket, conf.BucketRegion, uploadPath)
					Expect(uploadURL).To(Equal(expectedURL))
				})

				It("UploadFile should return error when sourcePath does not exists", func() {
					uploadPath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()

					sourceFileName := fmt.Sprintf("uploader-%d.target-file", time.Now().UnixNano())
					sourceFilePath := filepath.Join(os.TempDir(), sourceFileName)

					// should not call S3Client PuObject
					s3Client.EXPECT().PutObject(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
					_, err := s3Uploader.UploadFile(sourceFilePath, uploadPath)
					expectedErrorMessage := fmt.Sprintf("%s: no such file or directory", sourceFilePath)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(expectedErrorMessage))
				})

				It("UploadFile should return error when PutObject fails", func() {
					expectedError := errors.New("s3Client. PutObject failed to upload file")
					uploadPath := faker.UUIDHyphenated() + "/" + faker.UUIDHyphenated()

					file, err := os.CreateTemp(os.TempDir(), "uploader-*.source-file ")
					Expect(err).ToNot(HaveOccurred())

					defer func(file *os.File) {
						_ = file.Close()
						_ = os.Remove(file.Name())
					}(file)

					s3Client.EXPECT().PutObject(gomock.AssignableToTypeOf(&os.File{}), config.Get().BucketName, uploadPath, acl).Return(nil, expectedError)
					_, err = s3Uploader.UploadFile(file.Name(), uploadPath)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(err))
				})
			})

			Context("UploadRepo", func() {
				It("should upload repo ", func() {
					fileName := fmt.Sprintf("uploader-%d.file", time.Now().UnixNano())
					repoDirName := fmt.Sprintf("uploader-%d.repo", time.Now().UnixNano())
					sourceRepoDirPath := filepath.Join(conf.RepoTempPath, repoDirName)
					sourceFilePath := filepath.Join(sourceRepoDirPath, fileName)

					// create repo dir
					err := os.Mkdir(sourceRepoDirPath, os.ModePerm)
					Expect(err).ToNot(HaveOccurred())
					// create a repo file to upload
					file, err := os.Create(sourceFilePath)
					Expect(err).ToNot(HaveOccurred())
					defer func(file *os.File, dirPath string) {
						_ = file.Close()
						_ = os.Remove(file.Name())
						_ = os.Remove(dirPath)
					}(file, sourceRepoDirPath)
					wg := sync.WaitGroup{}
					wg.Add(1)
					expectedFilePath := fmt.Sprintf("%s/%s", orgID, strings.TrimPrefix(sourceFilePath, conf.RepoTempPath))
					s3Client.EXPECT().PutObject(gomock.AssignableToTypeOf(&os.File{}), conf.BucketName, expectedFilePath, acl).
						DoAndReturn(func(arg0, arg1, arg2, arg3 interface{}) (interface{}, error) {
							defer wg.Done()
							return nil, nil
						})
					// when acl is empty the default private acl will be used
					targetURL, err := s3Uploader.UploadRepo(sourceRepoDirPath, orgID, "")
					wg.Wait()
					Expect(err).ToNot(HaveOccurred())
					expectedTargetURL := fmt.Sprintf(
						"https://%s.s3.%s.amazonaws.com/%s/%s",
						conf.BucketName, conf.BucketRegion, orgID, strings.TrimPrefix(sourceRepoDirPath, conf.RepoTempPath),
					)
					Expect(targetURL).To(Equal(expectedTargetURL))
				})
				It("should upload repo on second attempt after failure ", func() {
					fileName := fmt.Sprintf("uploader-%d.file", time.Now().UnixNano())
					repoDirName := fmt.Sprintf("uploader-%d.repo", time.Now().UnixNano())
					sourceRepoDirPath := filepath.Join(conf.RepoTempPath, repoDirName)
					sourceFilePath := filepath.Join(sourceRepoDirPath, fileName)

					// create repo dir
					err := os.Mkdir(sourceRepoDirPath, os.ModePerm)
					Expect(err).ToNot(HaveOccurred())
					// create a repo file to upload
					file, err := os.Create(sourceFilePath)
					Expect(err).ToNot(HaveOccurred())
					defer func(file *os.File, dirPath string) {
						_ = file.Close()
						_ = os.Remove(file.Name())
						_ = os.Remove(dirPath)
					}(file, sourceRepoDirPath)
					wg := sync.WaitGroup{}
					wg.Add(1)
					expectedFilePath := fmt.Sprintf("%s/%s", orgID, strings.TrimPrefix(sourceFilePath, conf.RepoTempPath))
					s3Client.EXPECT().PutObject(gomock.AssignableToTypeOf(&os.File{}), conf.BucketName, expectedFilePath, acl).
						DoAndReturn(func(arg0, arg1, arg2, arg3 interface{}) (interface{}, error) {
							return nil, errors.New("Error uploading file")
						})
					s3Client.EXPECT().PutObject(gomock.AssignableToTypeOf(&os.File{}), conf.BucketName, expectedFilePath, acl).
						DoAndReturn(func(arg0, arg1, arg2, arg3 interface{}) (interface{}, error) {
							defer wg.Done()
							return nil, nil
						})
					// when acl is empty the default private acl will be used
					targetURL, err := s3Uploader.UploadRepo(sourceRepoDirPath, orgID, "")
					wg.Wait()
					Expect(err).ToNot(HaveOccurred())
					expectedTargetURL := fmt.Sprintf(
						"https://%s.s3.%s.amazonaws.com/%s/%s",
						conf.BucketName, conf.BucketRegion, orgID, strings.TrimPrefix(sourceRepoDirPath, conf.RepoTempPath),
					)
					Expect(targetURL).To(Equal(expectedTargetURL))
				})
			})
		})
	})
})
