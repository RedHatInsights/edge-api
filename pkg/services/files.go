package services

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
)

// Files defines methods to handle files
type FilesService struct{}

func NewFilesService() *FilesService {
	return &FilesService{}
}

// Untar file to destination path
func (f *FilesService) Untar(rc io.ReadCloser, dst string) error {
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
