// FIXME: golangci-lint
// nolint:govet,revive
package services

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

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

// GetExtractor returns a new extractor for files
func (s *BasicFileService) GetExtractor() files.Extractor {
	return s.extractor
}

// GetUploader returns a new uploader for files
func (s *BasicFileService) GetUploader() files.Uploader {
	return s.uploader
}

// GetDownloader returns a new downloads for files
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
	Client files.S3ClientInterface
	Bucket string
	BasicFileService
}

// LocalFilesService only handles local uploads
type LocalFilesService struct {
	BasicFileService
}

// NewFilesService creates a new service to handle files
func NewFilesService(log log.FieldLogger) FilesService {
	cfg := config.Get()
	basicFileService := BasicFileService{
		extractor:  files.NewExtractor(log),
		uploader:   files.NewUploader(log),
		downloader: files.NewDownloader(log),
	}
	if cfg.Local {
		return &LocalFilesService{basicFileService}
	}

	return NewS3FilesServices(files.GetNewS3Client(), basicFileService)
}

// GetFile returns the file given a path
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

// NewS3FilesServices return a new FilesService with s3 client
func NewS3FilesServices(client files.S3ClientInterface, basicFileService BasicFileService) FilesService {
	return &S3FilesService{
		Client:           client,
		Bucket:           config.Get().BucketName,
		BasicFileService: basicFileService,
	}
}

// GetFile returns the file given a path
func (s *S3FilesService) GetFile(path string) (io.ReadCloser, error) {
	o, err := s.Client.GetObject(s.Bucket, path)

	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchKey:
				// temporarily returning zero byte file for missing ostree optionals as 200
				// FIXME: this is a workaround being tested. don't let it become forgotten technical debt!
				if feature.Return200for404.IsEnabled() {
					basepath := filepath.Base(path)
					switch basepath {
					case ".commitmeta", "summary", "summary.sig", "superblock":
						o.Body.Close()
						zeroByteCloser := io.NopCloser(bytes.NewBufferString(""))

						return zeroByteCloser, nil
					}
				}

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
	return s.Client.GetSignedURL(cfg.BucketName, path, defaultURLSignatureExpiry*time.Minute)
}
