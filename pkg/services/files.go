package services

import "github.com/redhatinsights/edge-api/pkg/services/files"

type FilesService struct {
	Extractor  files.Extractor
	Uploader   files.Uploader
	Downloader files.Downloader
}

func NewFilesService() *FilesService {
	return &FilesService{
		Extractor:  files.NewExtractor(),
		Uploader:   files.NewUploader(),
		Downloader: files.NewDownloader(),
	}
}
