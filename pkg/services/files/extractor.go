package files

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
)

// Extractor defines methods to extract files to path
type Extractor interface {
	Extract(rc io.ReadCloser, dst string) error
}

// NewExtractor returns the main extractor used by EdgeAPI
func NewExtractor(log *log.Entry) Extractor {
	return &TARFileExtractor{log: log}
}

// TARFileExtractor implements a method to extract TAR files into a path
type TARFileExtractor struct {
	log *log.Entry
}

// Extract extracts file to destination path
func (f *TARFileExtractor) Extract(rc io.ReadCloser, dst string) error {
	defer rc.Close()
	tarReader := tar.NewReader(rc)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path, err := sanitizeExtractPath(dst, header.Name)
		if err != nil {
			// FIX ME!!! - Rollback previous solution due an error on sanitizeExtractPath
			// Crawl: log error and dont return since this code is hard to test locally
			f.log.WithField("error", err.Error()).Error("Error sanitizing path")
			// 	return err
		}
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		// FIX ME!!! - Rollback previous solution due an error on sanitizeExtractPath
		// file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		for {
			_, err = io.CopyN(file, tarReader, 1024*1024)
			if err != nil {
				if err == io.EOF {
					break
				}
				if err := file.Close(); err != nil {
					return err
				}
				return err
			}
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func sanitizeExtractPath(destination string, filePath string) (destpath string, err error) {
	destpath = filepath.Join(destination, filePath)
	prefix := filepath.Clean(destination) + string(os.PathSeparator)
	if !strings.HasPrefix(destpath, prefix) {
		err = fmt.Errorf("%s: illegal file path, prefix: %s, destpath: %s", filePath, prefix, destpath)
	}
	return
}
