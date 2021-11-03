package services

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
)

// FilesService is the interface for Files-related service information
type FilesService interface {
	GetFile(path string) (io.ReadCloser, error)
	GetExtractor() files.Extractor
	GetUploader() files.Uploader
	GetDownloader() files.Downloader
}

// S3FilesService contains S3 files-related information
type S3FilesService struct {
	Client     *s3.S3
	Bucket     string
	extractor  files.Extractor
	uploader   files.Uploader
	downloader files.Downloader
}

// NewFilesService creates a new service to handle files
func NewFilesService() FilesService {
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
			Region:      cfg.BucketRegion,
			Credentials: credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		})
		if err != nil {
			panic(err)
		}
	}
	client := s3.New(sess)
	return &S3FilesService{
		Client:     client,
		Bucket:     cfg.BucketName,
		extractor:  files.NewExtractor(),
		uploader:   files.NewUploader(),
		downloader: files.NewDownloader(),
	}
}

// GetExtractor retuns a new extractor for files
func (s *S3FilesService) GetExtractor() files.Extractor {
	return s.extractor
}

// GetUploader retuns a new uploader for files
func (s *S3FilesService) GetUploader() files.Uploader {
	return s.uploader
}

// GetDownloader retuns a new downloads for files
func (s *S3FilesService) GetDownloader() files.Downloader {
	return s.downloader
}

// GetFile retuns the file given a path
func (s *S3FilesService) GetFile(path string) (io.ReadCloser, error) {
	o, err := s.Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(path),
	})

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				return nil, fmt.Errorf("the object %s was not found on the S3 bucket", path)
			case s3.ErrCodeInvalidObjectState:
				return nil, fmt.Errorf("the object %s was not found on the S3 bucket because of an invalid state", path)
			default:
				return nil, fmt.Errorf("something wrong happened while reading from the S3 bucket")
			}
		}
		return nil, err
	}
	return o.Body, nil
}
