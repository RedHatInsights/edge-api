package commits

import (
	"io"
	"net/http"
	"os"
)

func CommitDownloader(source_url string, destination_path string) error {

	resp, err := http.Get(source_url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destination_path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
