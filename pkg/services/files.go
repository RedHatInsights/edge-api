package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
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

// NewFilesService creates a new service to handle files
func NewFilesService(log *log.Entry) FilesService {
	cfg := config.Get()
	if cfg.Local {
		return &LocalFilesService{
			BasicFileService{
				extractor:  files.NewExtractor(log),
				uploader:   files.NewUploader(log),
				downloader: files.NewDownloader(),
			},
		}
	}
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
		Client: client,
		Bucket: cfg.BucketName,
		BasicFileService: BasicFileService{
			extractor:  files.NewExtractor(log),
			uploader:   files.NewUploader(log),
			downloader: files.NewDownloader(),
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
