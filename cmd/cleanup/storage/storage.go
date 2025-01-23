package storage

import (
	url2 "net/url"
	"strings"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services/files"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/osbuild/logging/pkg/logrus"
)

// DefaultTimeDuration the default time duration to use, this will allow to speedup testing
var DefaultTimeDuration = 1 * time.Second

// GetPathFromURL return the path of an url
func GetPathFromURL(url string) (string, error) {
	RepoURL, err := url2.Parse(url)
	if err != nil {
		return "", err
	}
	return RepoURL.Path, nil
}

// DeleteAWSFolder delete an AWS S3 bucket folder
func DeleteAWSFolder(s3Client *files.S3Client, folder string) error {
	// remove the prefixed url separator if exists
	folder = strings.TrimPrefix(folder, "/")
	logger := logrus.WithField("folder-key", folder)
	configAttempts := config.Get().DeleteFilesAttempts
	configDelay := time.Duration(config.Get().DeleteFilesRetryDelay) * DefaultTimeDuration
	var err error
	for attempt := uint(1); attempt <= configAttempts; attempt++ {
		err = s3Client.FolderDeleter.Delete(config.Get().BucketName, folder)
		if err != nil {
			logger.WithFields(logrus.Fields{"attempt": attempt, "error": err.Error()}).Error("error deleting folder")
			time.Sleep(configDelay)
			continue
		}
		logger.WithField("attempt", attempt).Info("folder deleted successfully")
		break
	}
	// return the latest error
	return err
}

// DeleteAWSFile delete AWS S3 bucket file
func DeleteAWSFile(client *files.S3Client, fileKey string) error {
	logger := logrus.WithField("file-key", fileKey)
	var err error
	configAttempts := config.Get().DeleteFilesAttempts
	configDelay := time.Duration(config.Get().DeleteFilesRetryDelay) * DefaultTimeDuration
	for attempt := uint(1); attempt <= configAttempts; attempt++ {
		_, err = client.DeleteObject(config.Get().BucketName, fileKey)
		if err != nil {
			var contextErr error
			var errCode string
			if awsErr, ok := err.(awserr.Error); ok {
				errCode = awsErr.Code()
				contextErr = awsErr
			} else {
				contextErr = err
			}
			logger.WithFields(logrus.Fields{"attempt": attempt, "error": contextErr.Error(), "error-code": errCode}).Error("error deleting file")
			time.Sleep(configDelay)
			continue
		}
		logger.WithField("attempt", attempt).Info("file deleted successfully")
		break
	}
	// return the latest error
	return err
}
