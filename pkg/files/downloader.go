package files

import (
	"io"
	"net/http"
	"os"
)

// CommitDownloader download function that puts the source_url into the destination_path on the local filesystem
func CommitDownloader(sourceURL string, destinationPath string) error {

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
