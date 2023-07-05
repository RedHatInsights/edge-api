package cleanupimages

import (
	"errors"
	url2 "net/url"
	"strings"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/aws/aws-sdk-go/aws/awserr"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ErrImagesCleanUPNotAvailable error returned when the images clean up feature flag is disabled
var ErrImagesCleanUPNotAvailable = errors.New("images cleanup is not available")

// ErrImageNotCleanUPCandidate error returned when the image is not a cleanup candidate
var ErrImageNotCleanUPCandidate = errors.New("image is not a cleanup candidate")

// DefaultDataLimit the default data limit to use when collecting data
var DefaultDataLimit = 30

// DefaultMaxDataPageNumber the default data pages to handle as preventive way to enter an indefinite loop
var DefaultMaxDataPageNumber = 1000

// DefaultDeleteFoldersAttempts the default delete folder attempts
var DefaultDeleteFoldersAttempts = 10

// DefaultDeleteFoldersRetryDelay the default delete folder delay before a retry
var DefaultDeleteFoldersRetryDelay = 5 * time.Second

type CandidateImage struct {
	ImageID         uint           `json:"image_id"`
	ImageStatus     string         `json:"image_status"`
	ImageDeletedAt  gorm.DeletedAt `json:"image_deleted_at"`
	ImageSetID      uint           `json:"image_set_id"`
	CommitID        uint           `json:"commit_id"`
	CommitStatus    string         `json:"commit_status"`
	CommitTarURL    string         `json:"commit_tar_url"`
	RepoID          uint           `json:"repo_id"`
	RepoStatus      string         `json:"repo_status"`
	RepoURL         string         `json:"repo_url"`
	InstallerID     uint           `json:"installer_id"`
	InstallerStatus string         `json:"installer_status"`
	InstallerISOURL string         `json:"installer_iso_url"`
}

func GetPathFromURL(url string) (string, error) {
	RepoURL, err := url2.Parse(url)
	if err != nil {
		return "", err
	}
	return RepoURL.Path, nil
}

func DeleteAWSFolder(s3Client *files.S3Client, folder string) error {
	// remove the prefixed url separator if exists
	folder = strings.TrimPrefix(folder, "/")
	logger := log.WithField("folder-key", folder)
	var err error
	for attempt := 1; attempt <= DefaultDeleteFoldersAttempts; attempt++ {
		err = s3Client.FolderDeleter.Delete(config.Get().BucketName, folder)
		if err != nil {
			logger.WithFields(log.Fields{"attempt": attempt, "error": err.Error()}).Error("error deleting folder")
			time.Sleep(DefaultDeleteFoldersRetryDelay)
			continue
		}
		logger.WithField("attempt", attempt).Info("folder deleted successfully")
		break
	}
	// return the latest error
	return err
}

func DeleteAWSFile(client *files.S3Client, fileKey string) error {
	logger := log.WithField("file-key", fileKey)
	_, err := client.DeleteObject(config.Get().BucketName, fileKey)
	if err != nil {
		var contextErr error
		var errCode string
		if awsErr, ok := err.(awserr.Error); ok {
			errCode = awsErr.Code()
			contextErr = awsErr
		} else {
			contextErr = err
		}

		logger.WithFields(log.Fields{"error": contextErr.Error(), "error-code": errCode}).Error("error when deleting aws s3 file-key")
		return contextErr
	}
	logger.Info("file deleted successfully")
	return nil
}
func cleanUpImageTarFile(s3Client *files.S3Client, candidateImage *CandidateImage) error {
	logger := log.WithFields(log.Fields{
		"image_id":      candidateImage.ImageID,
		"commit_id":     candidateImage.CommitID,
		"tar-file-url":  candidateImage.CommitTarURL,
		"commit-status": candidateImage.InstallerStatus,
	})
	if candidateImage.CommitStatus == models.ImageStatusSuccess {
		if candidateImage.CommitTarURL != "" {
			urlPath, err := GetPathFromURL(candidateImage.CommitTarURL)
			if err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while getting resource path url")
				return err
			}
			logger = logger.WithField("tar-file-path", urlPath)
			logger.Debug("deleting tar file")
			err = DeleteAWSFile(s3Client, urlPath)
			if err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while deleting tar file")
				return err
			}
		}
		// clean url and update cleaned status
		if err := db.DB.Debug().Model(&models.Commit{}).Where("id", candidateImage.CommitID).
			Updates(map[string]interface{}{"status": models.ImageStatusStorageCleaned, "image_build_tar_url": ""}).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while updating commit status to cleaned")
			return err
		}
	}

	return nil
}

func cleanUpImageRepo(s3Client *files.S3Client, candidateImage *CandidateImage) error {
	logger := log.WithFields(log.Fields{
		"image_id":    candidateImage.ImageID,
		"commit_id":   candidateImage.CommitID,
		"repo-url":    candidateImage.RepoURL,
		"repo_status": candidateImage.RepoStatus,
	})

	if candidateImage.RepoStatus == models.ImageStatusSuccess {
		if candidateImage.RepoURL != "" {
			urlPath, err := GetPathFromURL(candidateImage.RepoURL)
			if err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while getting resource path url")
				return err
			}
			logger = logger.WithField("repo-url-path", urlPath)
			logger.Debug("deleting repo directory")
			err = DeleteAWSFolder(s3Client, urlPath)
			if err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while deleting repo directory")
				return err
			}
		}
		// clean url and update cleaned status
		if err := db.DB.Model(&models.Repo{}).Where("id", candidateImage.RepoID).
			Updates(map[string]interface{}{"status": models.ImageStatusStorageCleaned, "url": ""}).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while updating repo status to cleaned")
			return err
		}
	}

	return nil
}

func cleanUpImageISOFile(s3Client *files.S3Client, candidateImage *CandidateImage) error {
	logger := log.WithFields(log.Fields{
		"image_id":         candidateImage.ImageID,
		"installer_id":     candidateImage.InstallerID,
		"iso-file-url":     candidateImage.InstallerISOURL,
		"installer-status": candidateImage.InstallerStatus,
	})
	if candidateImage.InstallerStatus == models.ImageStatusSuccess {
		if candidateImage.InstallerISOURL != "" {
			urlPath, err := GetPathFromURL(candidateImage.InstallerISOURL)
			if err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while getting resource path url")
				return err
			}
			logger = logger.WithField("iso-file-path", urlPath)
			logger.Debug("deleting iso file")
			err = DeleteAWSFile(s3Client, urlPath)
			if err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while deleting iso file")
				return err
			}
		}
		// clean url and update with Cleaned status
		if err := db.DB.Model(&models.Installer{}).Where("id", candidateImage.InstallerID).
			Updates(map[string]interface{}{"status": models.ImageStatusStorageCleaned, "image_build_iso_url": ""}).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while updating installer status to cleaned")
			return err
		}
	}

	return nil
}

func cleanUpImageStorage(s3Client *files.S3Client, candidateImage *CandidateImage) error {
	logger := log.WithField("image_id", candidateImage.ImageID)
	logger.Info("image storage cleaning started")
	// cleanup tar file
	if err := cleanUpImageTarFile(s3Client, candidateImage); err != nil {
		return err
	}
	// cleanup repo
	if err := cleanUpImageRepo(s3Client, candidateImage); err != nil {
		return err
	}
	// cleanup iso file
	if err := cleanUpImageISOFile(s3Client, candidateImage); err != nil {
		return err
	}
	logger.Info("image storage cleaning finished successfully")
	return nil
}

func DeleteImage(candidateImage *CandidateImage) error {
	if imageDeletedAtValue, err := candidateImage.ImageDeletedAt.Value(); err != nil {
		return err
	} else if imageDeletedAtValue == nil {
		// delete only soft deleted images
		return ErrImageNotCleanUPCandidate
	}

	err := db.DB.Transaction(func(tx *gorm.DB) error {

		// delete images_packages with image_id
		if err := tx.Exec("DELETE FROM images_packages WHERE image_id=?", candidateImage.ImageID).Error; err != nil {
			return err
		}

		// delete images_repos with image_id
		if err := tx.Exec("DELETE FROM images_repos WHERE image_id=?", candidateImage.ImageID).Error; err != nil {
			return err
		}

		// delete images_custom_packages with image_id
		if err := tx.Exec("DELETE FROM images_custom_packages WHERE image_id=?", candidateImage.ImageID).Error; err != nil {
			return err
		}

		// delete commit_installed_packages with commit_id
		if err := tx.Exec("DELETE FROM commit_installed_packages WHERE commit_id=?", candidateImage.CommitID).Error; err != nil {
			return err
		}

		// delete image
		if err := tx.Unscoped().Where("id", candidateImage.ImageID).Delete(&models.Image{}).Error; err != nil {
			return err
		}

		// get the updates count with commit_id
		var updatesCount int64
		if err := tx.Unscoped().Debug().Model(&models.UpdateTransaction{}).Where("commit_id", candidateImage.CommitID).Count(&updatesCount).Error; err != nil {
			return err
		}

		// delete commit only if it has no update transactions
		if updatesCount == 0 {
			if err := tx.Unscoped().Where("id", candidateImage.CommitID).Delete(&models.Commit{}).Error; err != nil {
				return err
			}

			// delete repos with commit repo_id
			if err := tx.Unscoped().Where("id", candidateImage.RepoID).Delete(&models.Repo{}).Error; err != nil {
				return err
			}
		}

		// delete installer
		if err := tx.Unscoped().Where("id", candidateImage.InstallerID).Delete(&models.Installer{}).Error; err != nil {
			return err
		}

		// delete image_sets with image_set_id if no images associated
		var imagesCount int64
		if err := tx.Unscoped().Model(&models.Image{}).Where("image_set_id = ?", candidateImage.ImageSetID).Count(&imagesCount).Error; err != nil {
			return nil
		}
		if imagesCount == 0 {
			if err := tx.Unscoped().Where("id", candidateImage.ImageSetID).Delete(&models.ImageSet{}).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.WithFields(log.Fields{"image-id": candidateImage.ImageID, "error": err.Error()}).Error("error occurred while deleting image")
	}

	return err
}

func CleanUpImage(s3Client *files.S3Client, candidateImage *CandidateImage) error {

	imageDeletedAtValue, err := candidateImage.ImageDeletedAt.Value()
	if err != nil {
		return err
	}
	// clean up only deleted images OR images with ERROR status
	if !(imageDeletedAtValue != nil || candidateImage.ImageStatus == models.ImageStatusError) {
		return ErrImageNotCleanUPCandidate
	}

	if err := cleanUpImageStorage(s3Client, candidateImage); err != nil {
		return err
	}

	if imageDeletedAtValue != nil {
		// delete image forever when it's soft deleted
		if err := DeleteImage(candidateImage); err != nil {
			return err
		}
	}

	return nil
}

func GetCandidateImages(gormDB *gorm.DB) ([]CandidateImage, error) {
	var candidateImages []CandidateImage

	if err := gormDB.Debug().Table("images").
		Select(`images.id as image_id, images.deleted_at as image_deleted_at, images.status as image_status, images.image_set_id as image_set_id, 
commits.id as commit_id, commits.status as commit_status, commits.image_build_tar_url as commit_tar_url, 
repos.id as repo_id, repos.status as repo_status, repos.url repo_url, 
installers.id as installer_id, installers.status as installer_status, installers.image_build_iso_url as installer_iso_url`).
		Joins(`JOIN commits ON images.commit_id = commits.id `).
		Joins(`JOIN installers ON images.installer_id = installers.id `).
		Joins(`JOIN repos ON commits.repo_id = repos.id`).
		Where(`images.deleted_at IS NOT NULL`).
		Or(`images.status = 'ERROR' AND (repos.status='SUCCESS' OR commits.status='SUCCESS' OR installers.status='SUCCESS')`).
		Order("images.id").
		Limit(DefaultDataLimit).
		Scan(&candidateImages).Error; err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("error occurred when collecting images candidate")
		return nil, err
	}

	return candidateImages, nil
}

func CleanUpAllImages(s3Client *files.S3Client) error {
	if !feature.CleanUPImages.IsEnabled() {
		log.Warning("flag is disabled for cleanup of images feature")
		return ErrImagesCleanUPNotAvailable
	}

	imagesCount := 0
	page := 0
	for page < DefaultMaxDataPageNumber {
		candidateImages, err := GetCandidateImages(db.DB)
		if err != nil {
			return err
		}

		if len(candidateImages) == 0 {
			break
		}

		for _, image := range candidateImages {
			image := image
			if err := CleanUpImage(s3Client, &image); err != nil {
				return err
			}
		}

		imagesCount += len(candidateImages)
		page++
	}

	log.WithField("images_count", imagesCount).Info("cleanup images finished")
	return nil
}
