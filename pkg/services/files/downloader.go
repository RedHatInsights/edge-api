// FIXME: golangci-lint
// nolint:govet,revive
package files

import (
	"io"
	"net/http"
	url2 "net/url"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// Downloader is the interface that downloads a source into a path
type Downloader interface {
	DownloadToPath(source string, destinationPath string) error
}

// NewDownloader returns the main downloader used by Edge API
func NewDownloader(log *log.Entry) Downloader {
	cfg := config.Get()
	if cfg.Local {
		return &HTTPDownloader{log: log}
	}
	return &S3Downloader{log: log}
}

// HTTPDownloader implements Downloader and downloads from a URL through HTTP
type HTTPDownloader struct {
	log *log.Entry
}

// DownloadToPath download function that puts the source_url into the destination_path on the local filesystem
func (d *HTTPDownloader) DownloadToPath(sourceURL string, destinationPath string) error {

	d.log.WithFields(log.Fields{"sourceURL": sourceURL, "destinationPath": destinationPath}).Info("downloading using url ...")

	resp, err := http.Get(sourceURL) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := out.Close(); err != nil {
			d.log.WithField("error", err.Error()).Error("Error closing file")
		}
	}()

	_, err = io.Copy(out, resp.Body)
	return err
}

// S3Downloader aws s3 files Downloader, download resources at aws s3 bucket via sdk
type S3Downloader struct {
	log *log.Entry
}

// DownloadToPath download function that puts the source_url  at s3 bucket into the destination_path on the local filesystem
func (d *S3Downloader) DownloadToPath(sourceURL string, destinationPath string) error {
	d.log.WithFields(log.Fields{"sourceURL": sourceURL, "destinationPath": destinationPath}).Info("downloading from aws S3 bucket ...")
	cfg := config.Get()
	url, err := url2.Parse(sourceURL)
	if err != nil {
		d.log.WithFields(log.Fields{
			"error": err.Error(),
			"URL":   sourceURL,
		}).Error("error occurred when parsing url")
		return err
	}

	file, err := os.Create(destinationPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			d.log.WithField("error", err.Error()).Error("Error closing file")
		}
	}()

	sess := GetNewS3Session()
	downloader := s3manager.NewDownloader(sess)
	_, err = downloader.Download(file,
		&s3.GetObjectInput{
			Bucket: aws.String(cfg.BucketName),
			Key:    aws.String(url.Path),
		})

	if err != nil {
		d.log.WithField("error", err.Error()).Errorf(`Error downloading file : "%s"`, url.Path)
		return err
	}

	d.log.Infof(`sourceURL: "%s" successfully downloaded to destinationPath: "%s"`, sourceURL, destinationPath)
	return nil
}
