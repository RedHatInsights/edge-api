package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
)

// BasicFileService is the base file service struct
// It serves as a base for other file services implementations
type BasicFileService struct {
	extractor  files.Extractor
	uploader   files.Uploader
	downloader files.Downloader
}

// GetExtractor retuns a new extractor for files
func (s *BasicFileService) GetExtractor() files.Extractor {
	return s.extractor
}

// GetUploader retuns a new uploader for files
func (s *BasicFileService) GetUploader() files.Uploader {
	return s.uploader
}

// GetDownloader retuns a new downloads for files
func (s *BasicFileService) GetDownloader() files.Downloader {
	return s.downloader
}

// FilesService is the interface for Files-related service information
type FilesService interface {
	GetFile(path string) (io.ReadCloser, error)
	GetSignedURL(path string) (string, error)
	GetExtractor() files.Extractor
	GetUploader() files.Uploader
	GetDownloader() files.Downloader
}

// S3FilesService contains S3 files-related information
type S3FilesService struct {
	Client *s3.S3
	Bucket string
	BasicFileService
}

// LocalFilesService only handles local uploads
type LocalFilesService struct {
	BasicFileService
}

// GetNewS3Client return anew AWS s3 client
func GetNewS3Client() *s3.S3 {
	sess := files.GetNewS3Session()
	return s3.New(sess)
}

// NewFilesService creates a new service to handle files
func NewFilesService(log *log.Entry) FilesService {
	cfg := config.Get()
	// FIXME: this breaks local dev process with upstream Image Builder
	//			but commenting it out breaks the test. Fix one or the other. :-)
	if cfg.Local {
		return &LocalFilesService{
			BasicFileService{
				extractor:  files.NewExtractor(log),
				uploader:   files.NewUploader(log),
				downloader: files.NewDownloader(log),
			},
		}
	}
	client := GetNewS3Client()
	return &S3FilesService{
		Client: client,
		Bucket: cfg.BucketName,
		BasicFileService: BasicFileService{
			extractor:  files.NewExtractor(log),
			uploader:   files.NewUploader(log),
			downloader: files.NewDownloader(log),
		},
	}
}

// GetFile retuns the file given a path
func (s *LocalFilesService) GetFile(path string) (io.ReadCloser, error) {
	path = "/tmp/" + path
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	return f, nil
}

// GetSignedURL return a signed URL
func (s *LocalFilesService) GetSignedURL(path string) (string, error) {
	// locale File service does not support signed url, and return the same original url
	return path, nil
}

// GetFile returns the file given a path
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

// defaultURLSignatureExpiry The default URL signature expiry time in minutes
const defaultURLSignatureExpiry = 120

// GetSignedURL return and aws s3 bucket signed url
func (s *S3FilesService) GetSignedURL(path string) (string, error) {

	cfg := config.Get()
	client := GetNewS3Client()
	req, _ := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(cfg.BucketName),
		Key:    aws.String(path),
	})

	return req.Presign(defaultURLSignatureExpiry * time.Minute)
}
