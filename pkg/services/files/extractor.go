package files

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Extractor defines methods to extract files to path
type Extractor interface {
	Extract(rc io.ReadCloser, dst string) error
}

// NewExtractor returns the main extractor used by EdgeAPI
func NewExtractor() Extractor {
	return &TARFileExtractor{}
}

// TARFileExtractor implements a method to extract TAR files into a path
type TARFileExtractor struct{}

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

		path := filepath.Join(dst, header.Name)
		// FIX ME!!! - Rollback previous solution due an error on sanitizeExtractPath
		// path, err := sanitizeExtractPath(header.Name, dst)
		// if err != nil {
		// 	return err
		// }
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

func sanitizeExtractPath(filePath string, destination string) (destpath string, err error) {
	destpath = filepath.Join(destination, filePath)
	if !strings.HasPrefix(destpath, filepath.Clean(destination)+string(os.PathSeparator)) {
		err = fmt.Errorf("%s: illegal file path", filePath)
	}
	return
}
