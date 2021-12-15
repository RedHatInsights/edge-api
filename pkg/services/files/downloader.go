package files

import (
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
)

// Downloader is the interface that downloads a source into a path
type Downloader interface {
	DownloadToPath(source string, destinationPath string) error
}

// NewDownloader returns the main downloader used by Edge API
func NewDownloader() Downloader {
	return &HTTPDownloader{}
}

// HTTPDownloader implements Downloader and downloads from a URL through HTTP
type HTTPDownloader struct{}

// DownloadToPath download function that puts the source_url into the destination_path on the local filesystem
func (d *HTTPDownloader) DownloadToPath(sourceURL string, destinationPath string) error {

	resp, err := http.Get(sourceURL)
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
			log.Error("Error closing file: ", err)
		}
	}()

	_, err = io.Copy(out, resp.Body)
	return err
}
