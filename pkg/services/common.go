package services

import (
	"archive/tar"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

const (
	// TrailingSlashIndex is the index used to remove trailing slashs from path prefixes
	TrailingSlashIndex int = 1
)

func getNameAndPrefix(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "name")
	log.Debugf("getNameAndPrefix::name: %#v", name)
	if name == "" {
		return "", "", fmt.Errorf("repo name not provided")
	}
	pathPrefix := getPathPrefix(r.URL.Path, name)
	return name, pathPrefix, nil
}

func getPathPrefix(path string, name string) string {
	_r := strings.Index(path, "/"+name+"/")
	log.Debugf("getNameAndPrefix::_r: %#v", _r)
	pathPrefix := string(path[:_r+TrailingSlashIndex])
	log.Debugf("getNameAndPrefix::pathPrefix: %#v", pathPrefix)
	return pathPrefix
}

// Untar file to destination path
func Untar(rc io.ReadCloser, dst string) error {
	defer rc.Close()
	tarReader := tar.NewReader(rc)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(dst, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tarReader)
		if err != nil {
			file.Close()
			return err
		}
		file.Close()
	}
	return nil
}
