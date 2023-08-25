package cleanuporphancommits

import (
	"errors"
	"strings"

	"github.com/redhatinsights/edge-api/cmd/cleanup/storage"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/files"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ErrCleanupOrphanCommitsNotAvailable error returned when the cleanup orphan commits feature flag is disabled
var ErrCleanupOrphanCommitsNotAvailable = errors.New("cleanup orphan commits is not available")

// ErrCleanUpAllOrphanCommitsInterrupted error returned when an error is returned from cleanupOrphanCommit function
var ErrCleanUpAllOrphanCommitsInterrupted = errors.New("cleanup all orphan commits is interrupted")

// DefaultDataLimit the default data limit to use when collecting data
var DefaultDataLimit = 45

// DefaultMaxDataPageNumber the default data pages to handle as preventive way to enter an indefinite loop
var DefaultMaxDataPageNumber = 1000

type OrphanCommitCandidate struct {
	CommitID   uint    `json:"commit_id"`
	RepoID     *uint   `json:"repo_id"`
	RepoURL    *string `json:"repo_url"`
	RepoStatus *string `json:"repo_status"`
}

func deleteCommit(commitCandidate *OrphanCommitCandidate) error {
	logger := log.WithField("commit_id", commitCandidate.CommitID)
	logger.Info("deleting commit")
	err := db.DB.Transaction(func(tx *gorm.DB) error {
		// delete commit from updatetransaction_commits with commit_id
		if err := tx.Exec("DELETE FROM updatetransaction_commits WHERE commit_id=?", commitCandidate.CommitID).Error; err != nil {
			return err
		}

		// delete commit_installed_packages with commit_id
		if err := tx.Exec("DELETE FROM commit_installed_packages WHERE commit_id=?", commitCandidate.CommitID).Error; err != nil {
			return err
		}

		// delete commit with commit_id
		if err := tx.Exec("DELETE FROM commits WHERE id=?", commitCandidate.CommitID).Error; err != nil {
			return err
		}

		if commitCandidate.RepoID != nil {
			// delete commit repo with repo_id
			if err := tx.Exec("DELETE FROM repos WHERE id=?", *commitCandidate.RepoID).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while deleting commit")
	} else {
		log.Debug("commit deleted successfully")
	}
	return err
}

func cleanupOrphanCommit(s3Client *files.S3Client, commitCandidate *OrphanCommitCandidate) error {

	logger := log.WithField("commit_id", commitCandidate.CommitID)
	logger.Info("clearing commit")

	if commitCandidate.RepoID != nil && commitCandidate.RepoURL != nil && commitCandidate.RepoStatus != nil &&
		*commitCandidate.RepoStatus == models.RepoStatusSuccess && *commitCandidate.RepoURL != "" {
		logger = logger.WithFields(log.Fields{
			"repo_id":     *commitCandidate.RepoID,
			"repo_url":    *commitCandidate.RepoURL,
			"repo_status": *commitCandidate.RepoStatus,
		})

		urlPath, err := storage.GetPathFromURL(*commitCandidate.RepoURL)
		if err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while getting resource path url")
			return err
		}
		logger = logger.WithField("repo_url_path", urlPath)
		logger.Debug("deleting repo directory")
		if err = storage.DeleteAWSFolder(s3Client, urlPath); err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while deleting repo directory")
			return err
		}

		// clean url and update cleaned status
		if err := db.DB.Model(&models.Repo{}).Where("id", *commitCandidate.RepoID).
			Updates(map[string]interface{}{"status": models.UpdateStatusStorageCleaned, "url": ""}).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while updating repo status to cleaned")
			return err
		}
	}

	err := deleteCommit(commitCandidate)
	if err == nil {
		logger.Info("commit cleared successfully")
	}
	return err
}

func getOrphanCommitsCandidates(gormDB *gorm.DB) ([]OrphanCommitCandidate, error) {
	var orphanCommitsCandidates []OrphanCommitCandidate
	mainQuery := gormDB.Debug().Table("commits").
		Select("commits.id as commit_id, commits.repo_id as repo_id, repos.url as repo_url, repos.status as repo_status").
		Joins("LEFT JOIN images ON images.commit_id = commits.id").
		Joins("LEFT JOIN update_transactions ON update_transactions.commit_id = commits.id").
		Joins("LEFT JOIN repos ON repos.id = commits.repo_id").
		Where("images.id IS NULL AND update_transactions.id IS NULL")

	if strings.Contains(config.Get().EdgeAPIBaseURL, "stage") {
		// in old versions repos had commit_id
		// this impossible to test locally as that column has been deleted from repos models
		// this case needs to be handled separately
		// this is only valid for stage env,
		// on prod the column commit_id in repos table does not exit
		log.Info("stage environment detected, add sub-query to ignore commits linked to repos table by repos.commit_id")
		subQuery := db.DB.Table("repos").Select("commit_id").Where("commit_id IS NOT NULL")
		mainQuery = mainQuery.Where("commits.id NOT IN (?)", subQuery)
	}

	err := mainQuery.Order("commits.id ASC").Limit(DefaultDataLimit).Scan(&orphanCommitsCandidates).Error

	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("error occurred when collecting orphan commits candidates")
	}
	return orphanCommitsCandidates, err
}

func CleanupAllOrphanCommits(s3Client *files.S3Client, gormDB *gorm.DB) error {
	if !feature.CleanUPOrphanCommits.IsEnabled() {
		log.Warning("cleanup of orphan commits feature flag is disabled")
		return ErrCleanupOrphanCommitsNotAvailable
	}
	if gormDB == nil {
		gormDB = db.DB
	}

	commitsCount := 0
	page := 0
	for page < DefaultMaxDataPageNumber && feature.CleanUPOrphanCommits.IsEnabled() {
		commitsCandidates, err := getOrphanCommitsCandidates(gormDB)
		if err != nil {
			return err
		}

		if len(commitsCandidates) == 0 {
			break
		}

		// create a new channel for each iteration
		errChan := make(chan error)

		for _, commitCandidate := range commitsCandidates {
			commitCandidate := commitCandidate
			// handle all the page orphan commits candidates at once, by default 45
			go func(resChan chan error) {
				resChan <- cleanupOrphanCommit(s3Client, &commitCandidate)
			}(errChan)
		}

		// wait for all results to be returned
		errCount := 0
		for range commitsCandidates {
			resError := <-errChan
			if resError != nil {
				// errors are logged in the related functions, at this stage need to know if there is an error, to break the loop
				errCount++
			}
		}

		close(errChan)

		commitsCount += len(commitsCandidates)

		// break on any error
		if errCount > 0 {
			log.WithFields(log.Fields{"orphan_commits_count": commitsCount, "errors_count": errCount}).Info("cleanup orphan commits was interrupted because of cleanup errors")
			return ErrCleanUpAllOrphanCommitsInterrupted
		}
		page++
	}

	log.WithField("orphan_commits_count", commitsCount).Info("cleanup orphan commits finished")
	return nil
}
