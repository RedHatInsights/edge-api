package postmigraterepos

import (
	"errors"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"
)

// ErrPostMigrationNotAvailable error returned when the post delete migration feature flag is disabled
var ErrPostMigrationNotAvailable = errors.New("post migration  delete repositories is not available")

// PostMigrateDeleteCustomRepo delete all custom repositories that already migrate to content-sources repositories
// and doesnt connect to image
func PostMigrateDeleteCustomRepo() (int64, error) {

	logger := log.WithField("context", "post-migration-delete-repos")
	if !feature.PostMigrateDeleteCustomRepositories.IsEnabled() {
		logger.Info("post migration delete repository feature is disabled, post migration delete repository is not available")
		return 0, ErrPostMigrationNotAvailable
	}
	var customReposToDelete []models.ThirdPartyRepo
	if err := db.DB.Where("deleted_at IS NULL AND uuid IS NOT NULL AND uuid !='' AND id NOT IN (SELECT third_party_repo_id FROM images_repos)").
		Find(&customReposToDelete).Error; err != nil {
		logger.WithField("error", err.Error()).Error("error occurred when collecting custom repositories candidates to delete")
		return 0, err
	}
	if len(customReposToDelete) == 0 {
		logger.Info("there is no repositories to delete ")
		return 0, nil
	}
	logger.WithField("customRepoToDelete", len(customReposToDelete)).Info("The custom repositories that will delete after migration")

	result := db.DB.Debug().Delete(&customReposToDelete)
	if result.Error != nil {
		logger.WithField("error", result.Error.Error()).Error("error occurred when deleting repositories")
		return 0, result.Error
	}
	return result.RowsAffected, nil

}
