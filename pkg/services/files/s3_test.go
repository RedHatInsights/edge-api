package files_test

import (
	"bytes"
	"errors"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	"github.com/redhatinsights/edge-api/pkg/services/mock_files"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bxcodec/faker/v3"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("S3 Test", func() {

	Describe("Debug True", func() {
		var initialDebug bool
		var initialAccessKey string
		var initialSecretKey string
		var cfg *config.EdgeConfig
		BeforeEach(func() {
			cfg = config.Get()
			initialDebug = cfg.Debug
			cfg.Debug = true
			initialAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
			initialSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		})
		AfterEach(func() {
			cfg.Local = initialDebug
			os.Setenv("AWS_ACCESS_KEY_ID", initialAccessKey)
			os.Setenv("AWS_SECRET_ACCESS_KEY", initialSecretKey)
		})
		It("Get new s3 session successfully", func() {
			accessKey := faker.UUIDHyphenated()
			accessID := faker.UUIDHyphenated()

			os.Setenv("AWS_ACCESS_KEY_ID", accessID)
			os.Setenv("AWS_SECRET_ACCESS_KEY", accessKey)

			session := files.GetNewS3Session()
			Expect(session).ToNot(BeNil())
			credentials, err := session.Config.Credentials.Get()
			Expect(err).To(BeNil())
			Expect(credentials.AccessKeyID).To(Equal(accessID))
			Expect(credentials.SecretAccessKey).To(Equal(accessKey))
			Expect(credentials.ProviderName).To(Equal("EnvConfigCredentials"))
		})
	})

	Describe("Debug False", func() {
		var initialDebug bool
		var initialAccessKey string
		var initialSecretKey string

		var cfg *config.EdgeConfig

		BeforeEach(func() {
			cfg = config.Get()
			initialDebug = cfg.Debug
			initialAccessKey = cfg.AccessKey
			initialSecretKey = cfg.SecretKey
			cfg.Debug = false
			cfg.AccessKey = faker.UUIDHyphenated()
			cfg.SecretKey = faker.UUIDHyphenated()
		})
		AfterEach(func() {
			cfg.Local = initialDebug
			cfg.AccessKey = initialAccessKey
			cfg.SecretKey = initialSecretKey
		})

		It("Get new s3 session successfully", func() {
			session := files.GetNewS3Session()
			Expect(session).ToNot(BeNil())
			credentials, err := session.Config.Credentials.Get()
			Expect(err).To(BeNil())
			Expect(credentials.AccessKeyID).To(Equal(cfg.AccessKey))
			Expect(credentials.SecretAccessKey).To(Equal(cfg.SecretKey))
			Expect(credentials.ProviderName).To(Equal("StaticProvider"))
		})
	})

	Describe("GetNewS3Client", func() {
		It("should get new client successfully", func() {
			s3Client := files.GetNewS3Client()
			Expect(s3Client).ToNot(BeNil())
			Expect(s3Client.Client).ToNot(BeNil())
			Expect(s3Client.Downloader).ToNot(BeNil())
			Expect(s3Client.Uploader).ToNot(BeNil())
			Expect(s3Client.RequestPreSigner).ToNot(BeNil())
		})
	})

	Describe("S3ClientAPI", func() {
		var ctrl *gomock.Controller
		var s3ClientAPI *mock_files.MockS3ClientAPI
		var s3DownloaderAPI *mock_files.MockS3DownloaderAPI
		var s3UploaderAPI *mock_files.MockS3UploaderAPI
		var requestPreSigner *mock_files.MockRequestPreSignerAPI
		var client files.S3Client
		var s3RequestPresignerAPI *mock_files.MockS3RequestAPI

		bucket := faker.UUIDHyphenated()
		key := faker.UUIDHyphenated()
		acl := faker.UUIDHyphenated()
		expire := 3 * time.Hour

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			s3ClientAPI = mock_files.NewMockS3ClientAPI(ctrl)
			s3DownloaderAPI = mock_files.NewMockS3DownloaderAPI(ctrl)
			s3UploaderAPI = mock_files.NewMockS3UploaderAPI(ctrl)
			requestPreSigner = mock_files.NewMockRequestPreSignerAPI(ctrl)
			client = files.S3Client{
				Client:           s3ClientAPI,
				Downloader:       s3DownloaderAPI,
				Uploader:         s3UploaderAPI,
				RequestPreSigner: requestPreSigner,
			}
			s3RequestPresignerAPI = mock_files.NewMockS3RequestAPI(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("GetObject", func() {
			It("should get Object", func() {
				expectedVersionID := faker.UUIDHyphenated()

				s3ClientAPI.EXPECT().GetObject(
					&s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
				).Return(&s3.GetObjectOutput{VersionId: aws.String(expectedVersionID)}, nil)

				output, err := client.GetObject(bucket, key)
				Expect(err).To(BeNil())
				Expect(aws.StringValue(output.VersionId)).To(Equal(expectedVersionID))
			})

			It("should return error when get Object error", func() {
				expectedError := errors.New("GetObject error")

				s3ClientAPI.EXPECT().GetObject(
					&s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
				).Return(nil, expectedError)

				output, err := client.GetObject(bucket, key)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
				Expect(output).To(BeNil())
			})
		})

		Context("PutObject", func() {
			It("should put Object", func() {
				expectedVersionID := faker.UUIDHyphenated()
				fileBytes := bytes.NewReader([]byte(faker.UUIDHyphenated()))
				s3ClientAPI.EXPECT().PutObject(
					&s3.PutObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
						Body:   fileBytes,
						ACL:    aws.String(acl),
					},
				).Return(&s3.PutObjectOutput{VersionId: aws.String(expectedVersionID)}, nil)
				output, err := client.PutObject(fileBytes, bucket, key, acl)
				Expect(err).To(BeNil())
				Expect(aws.StringValue(output.VersionId)).To(Equal(expectedVersionID))
			})

			It("should return error when put Object error", func() {
				expectedError := errors.New("PutObject error")
				fileBytes := bytes.NewReader([]byte(faker.UUIDHyphenated()))

				s3ClientAPI.EXPECT().PutObject(
					&s3.PutObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
						Body:   fileBytes,
						ACL:    aws.String(acl),
					},
				).Return(nil, expectedError)

				output, err := client.PutObject(fileBytes, bucket, key, acl)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
				Expect(output).To(BeNil())
			})
		})

		Context("GetSignedURL", func() {

			It("should get signed url", func() {
				expectedRequest := &request.Request{RequestID: faker.UUIDHyphenated()}
				expectURL := faker.URL()

				s3ClientAPI.EXPECT().GetObjectRequest(
					&s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
				).Return(expectedRequest, nil)

				requestPreSigner.EXPECT().Presign(expectedRequest, expire).Return(expectURL, nil)

				signedURL, err := client.GetSignedURL(bucket, key, expire)
				Expect(err).To(BeNil())
				Expect(signedURL).To(Equal(expectURL))
			})

			It("should return error when presign error", func() {
				expectedRequest := &request.Request{RequestID: faker.UUIDHyphenated()}
				expectedError := errors.New("presign error")

				s3ClientAPI.EXPECT().GetObjectRequest(
					&s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
				).Return(expectedRequest, nil)

				requestPreSigner.EXPECT().Presign(expectedRequest, expire).Return("", expectedError).Times(1)

				_, err := client.GetSignedURL(bucket, key, expire)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})

			Context("RequestPreSigner", func() {
				It("should s3 presign ", func() {
					expectedURL := faker.URL()
					requestPresigner := &files.RequestPreSigner{}

					s3RequestPresignerAPI.EXPECT().Presign(expire).Return(expectedURL, nil)

					preSignedURL, err := requestPresigner.Presign(s3RequestPresignerAPI, expire)
					Expect(err).ToNot(HaveOccurred())
					Expect(preSignedURL).To(Equal(expectedURL))
				})
				It("should return error when s3 presign error", func() {
					expectedError := errors.New("s3 Presign error")
					requestPresigner := &files.RequestPreSigner{}

					s3RequestPresignerAPI.EXPECT().Presign(expire).Return("", expectedError)

					preSignedURL, err := requestPresigner.Presign(s3RequestPresignerAPI, expire)
					Expect(err).To(HaveOccurred())
					Expect(err).To(Equal(expectedError))
					Expect(preSignedURL).To(BeEmpty())
				})
			})
		})

		Context("Download", func() {
			It("should s3 download", func() {
				expectedNumber := int64(100)
				buf := aws.NewWriteAtBuffer([]byte{})
				s3DownloaderAPI.EXPECT().Download(
					buf, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
				).Return(expectedNumber, nil)
				n, err := client.Download(buf, bucket, key)
				Expect(err).ToNot(HaveOccurred())
				Expect(n).To(Equal(expectedNumber))
			})

			It("should return error when s3 download fail", func() {
				expectedError := errors.New("s3 download error")
				buf := aws.NewWriteAtBuffer([]byte{})
				s3DownloaderAPI.EXPECT().Download(
					buf, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)},
				).Return(int64(0), expectedError)
				_, err := client.Download(buf, bucket, key)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})

		Context("Upload", func() {
			It("should s3 upload", func() {
				expectedLocation := faker.URL()
				buf := bytes.NewReader([]byte(faker.UUIDHyphenated()))
				s3UploaderAPI.EXPECT().Upload(
					&s3manager.UploadInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
						ACL:    aws.String(acl),
						Body:   buf,
					},
				).Return(&s3manager.UploadOutput{Location: expectedLocation}, nil)
				output, err := client.Upload(buf, bucket, key, acl)
				Expect(err).ToNot(HaveOccurred())
				Expect(output).ToNot(BeNil())
				Expect(output.Location).To(Equal(expectedLocation))
			})

			It("should return error when s3 upload fail", func() {
				expectedError := errors.New("s3 upload error")
				buf := bytes.NewReader([]byte(faker.UUIDHyphenated()))
				s3UploaderAPI.EXPECT().Upload(
					&s3manager.UploadInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(key),
						ACL:    aws.String(acl),
						Body:   buf,
					},
				).Return(nil, expectedError)
				_, err := client.Upload(buf, bucket, key, acl)
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(expectedError))
			})
		})
	})
})
