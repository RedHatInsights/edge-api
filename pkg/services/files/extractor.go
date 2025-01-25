// FIXME: golangci-lint
// nolint:govet,revive
package files

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"

	log "github.com/osbuild/logging/pkg/logrus"
)

// Extractor defines methods to extract files to path
type Extractor interface {
	Extract(rc io.ReadCloser, dst string) error
}

// NewExtractor returns the main extractor used by EdgeAPI
func NewExtractor(log log.FieldLogger) Extractor {
	return &TARFileExtractor{log: log}
}

// TARFileExtractor implements a method to extract TAR files into a path
type TARFileExtractor struct {
	log log.FieldLogger
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

		path, err := sanitizePath(dst, header.Name)
		if err != nil {
			f.log.WithField("error", err.Error()).Error("Error sanitizing path")
		}
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
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
