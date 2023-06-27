package files

import (
	"context"
	"io"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3iface"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3ClientAPI base interface for S3ClientAPI
type S3ClientAPI interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
	DeleteObject(input *s3.DeleteObjectInput) (*s3.DeleteObjectOutput, error)
	GetObjectRequest(input *s3.GetObjectInput) (req *request.Request, output *s3.GetObjectOutput)
}

// S3DownloaderAPI base interface for S3DownloaderAPI
type S3DownloaderAPI interface {
	Download(w io.WriterAt, input *s3.GetObjectInput, options ...func(*s3manager.Downloader)) (n int64, err error)
}

// S3UploaderAPI base interface for S3UploaderAPI
type S3UploaderAPI interface {
	Upload(input *s3manager.UploadInput, options ...func(*s3manager.Uploader)) (*s3manager.UploadOutput, error)
}

// S3RequestAPI base interface for S3RequestAPI
type S3RequestAPI interface {
	Presign(expire time.Duration) (string, error)
}

// RequestPreSignerAPI base interface for RequestPreSignerAPI
type RequestPreSignerAPI interface {
	Presign(req S3RequestAPI, expire time.Duration) (string, error)
}

// S3ClientInterface  base interface for S3Client
type S3ClientInterface interface {
	GetObject(bucket string, key string) (output *s3.GetObjectOutput, err error)
	PutObject(file io.ReadSeeker, bucket string, key string, acl string) (*s3.PutObjectOutput, error)
	DeleteObject(bucket string, key string) (*s3.DeleteObjectOutput, error)
	Download(file io.WriterAt, bucket string, key string) (n int64, err error)
	Upload(file io.Reader, bucket string, key string, acl string) (*s3manager.UploadOutput, error)
	GetSignedURL(bucket string, key string, expire time.Duration) (string, error)
}

// S3Client a struct of the S3Client
type S3Client struct {
	Client           S3ClientAPI
	Downloader       S3DownloaderAPI
	RequestPreSigner RequestPreSignerAPI
	Uploader         S3UploaderAPI
	FolderDeleter    BatchFolderDeleterAPI
}

// GetObject API operation on S3 bucket
func (s3Client *S3Client) GetObject(bucket string, key string) (*s3.GetObjectOutput, error) {
	return s3Client.Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// PutObject API operation on S3 bucket
func (s3Client *S3Client) PutObject(file io.ReadSeeker, bucket string, key string, acl string) (*s3.PutObjectOutput, error) {
	return s3Client.Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
		ACL:    aws.String(acl),
	})
}

// DeleteObject API operation on S3 bucket
func (s3Client *S3Client) DeleteObject(bucket string, key string) (*s3.DeleteObjectOutput, error) {
	return s3Client.Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// Download downloads an object in S3 and writes the payload into file
func (s3Client *S3Client) Download(file io.WriterAt, bucket string, key string) (int64, error) {
	return s3Client.Downloader.Download(
		file,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
	)
}

// Upload uploads an object to S3 bucket while reading from file
func (s3Client *S3Client) Upload(file io.Reader, bucket string, key string, acl string) (*s3manager.UploadOutput, error) {
	return s3Client.Uploader.Upload(
		&s3manager.UploadInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			ACL:    aws.String(acl),
			Body:   file,
		},
	)
}

// GetSignedURL return a signed URL of a s3 bucket object location
func (s3Client *S3Client) GetSignedURL(bucket string, key string, expire time.Duration) (string, error) {
	req, _ := s3Client.Client.GetObjectRequest(
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
	)
	return s3Client.RequestPreSigner.Presign(req, expire)
}

// RequestPreSigner define a request pre-signer
type RequestPreSigner struct{}

// Presign return a signed URL of a s3 bucket object using a predefined object request
func (rs *RequestPreSigner) Presign(req S3RequestAPI, expire time.Duration) (string, error) {
	return req.Presign(expire)
}

// GetNewS3Client return a new S3Client
func GetNewS3Client() *S3Client {
	clientSession := GetNewS3Session()
	client := s3.New(clientSession)
	return &S3Client{
		Client:     client,
		Downloader: s3manager.NewDownloaderWithClient(client),
		Uploader: s3manager.NewUploaderWithClient(client, func(u *s3manager.Uploader) {
			u.Concurrency = 1
		}),
		RequestPreSigner: &RequestPreSigner{},
		FolderDeleter:    NewBachFolderDeleter(client),
	}
}

// GetNewS3Session return a new aws s3 session
func GetNewS3Session() *session.Session {
	cfg := config.Get()
	var sess *session.Session
	if cfg.Debug {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			// Force enable Shared Config support
			SharedConfigState: session.SharedConfigEnable,
		}))
	} else {
		var err error
		sess, err = session.NewSession(&aws.Config{
			Region:      &cfg.BucketRegion,
			Credentials: credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		})
		if err != nil {
			logger.LogErrorAndPanic("failure creating new session", err)
		}
	}
	return sess
}

type BatchFolderDeleterAPI interface {
	Delete(bucket string, folderKey string) error
}

type BatchFolderDeleter struct {
	client s3iface.S3API
}

func NewBachFolderDeleter(client s3iface.S3API) BatchFolderDeleterAPI {
	return &BatchFolderDeleter{client: client}
}

func (batchFD *BatchFolderDeleter) Delete(bucket string, folderKey string) error {
	// create a folder objects iterator
	iter := s3manager.NewDeleteListIterator(batchFD.client, &s3.ListObjectsInput{Bucket: aws.String(bucket), Prefix: aws.String(folderKey)})
	// use iterator to delete all objects
	return s3manager.NewBatchDeleteWithClient(batchFD.client).Delete(context.Background(), iter)
}
