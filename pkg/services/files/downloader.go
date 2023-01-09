// FIXME: golangci-lint
// nolint:govet,revive
package files

import (
	"io"
	"net/http"
	url2 "net/url"
	"os"

	"github.com/redhatinsights/edge-api/config"

	log "github.com/sirupsen/logrus"
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
	return NewS3Downloader(log, GetNewS3Client())
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

// NewS3Downloader return a new S3Downloader
func NewS3Downloader(logger *log.Entry, client S3ClientInterface) *S3Downloader {
	return &S3Downloader{log: logger, Client: client}
}

// S3Downloader aws s3 files Downloader, download resources at aws s3 bucket via sdk
type S3Downloader struct {
	log    *log.Entry
	Client S3ClientInterface
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

	_, err = d.Client.Download(file, cfg.BucketName, url.Path)

	if err != nil {
		d.log.WithField("error", err.Error()).Errorf(`Error downloading file : "%s"`, url.Path)
		return err
	}

	d.log.Infof(`sourceURL: "%s" successfully downloaded to destinationPath: "%s"`, sourceURL, destinationPath)
	return nil
}
