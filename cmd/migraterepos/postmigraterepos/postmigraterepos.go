package postmigraterepos

import (
	"github.com/redhatinsights/edge-api/cmd/migraterepos/repairrepos"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"
)

// PostMigrateDeleteCustomRepo delete all custom repositories that already migrate to content-sources repositories
// and doesnt connect to image
func PostMigrateDeleteCustomRepo() (int64, error) {

	logger := log.WithField("context", "context-sources-migration")
	if !feature.MigrateCustomRepositories.IsEnabled() {
		logger.Info("post migration delete repository feature is disabled, post migration delete repository is not available")
		return 0, repairrepos.ErrPostMigrationNotAvailable
	}
	var customReposToDelete []models.ThirdPartyRepo
	if err := db.DB.Debug().
		Where("deleted_at IS NULL uuid IS not NULL and uuid =!'' AND id NOT IN (SELECT third_party_repo_id FROM images_repos);").
		Find(&customReposToDelete).Error; err != nil {
		logger.WithField("error", err.Error()).Error("Error checking migrated custom repository existence")
		return 0, err
	}
	if len(customReposToDelete) == 0 {
		logger.Info("there is no repositories to delete ")
		return 0, nil
	}
	logger.WithField("customRepotodelete", len(customReposToDelete)).Info("The custom repositories that will delete after migration")
	var affectedRepos int64 = 0
	if result := db.DB.Debug().Exec("DELETE FROM third_party_repos WHERE deleted_at IS NULL AND uuid IS NOT NULL and uuid !='' AND id NOT IN (SELECT third_party_repo_id FROM images_repos);"); result.Error != nil {
		logger.WithField("error", result.Error).Error("Error when trying to delete ")
		return 0, result.Error

	} else {
		affectedRepos = result.RowsAffected
	}
	return affectedRepos, nil

}
