package services

import (
	"io"
	"net/http"
	"os"
)

// Downloader is the interface that downloads a source into a path
type Downloader interface {
	DownloadToPath(source string, destinationPath string) error
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
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
