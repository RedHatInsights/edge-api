package manual

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerMigration("History", Migrate000History)
}

// Migrate000History is a manual migration that does nothing. The database used to be migrated
// pretty much randomly by changing ./cmd/migrate/main.go and running the migration. This is the last
// commented block that was present when we introduced the new migration system. For more details see
// the git history.
func Migrate000History() error {
	/*
		// FIXME: this can create issues when only one out of many replicas evicts
		// If there any image builds in progress, in the current architecture, we need to set them as errors because this is a brand new deployment
		var images []models.Image
		db.DB.Where(&models.Image{Status: models.ImageStatusBuilding}).Find(&images)
		for _, image := range images {
			log.WithField("imageID", image.ID).Debug("Found image with building status")
			image.Status = models.ImageStatusError
			if image.Commit != nil {
				image.Commit.Status = models.ImageStatusError
				if image.Commit.Repo != nil {
					image.Commit.Repo.Status = models.RepoStatusError
					db.DB.Save(image.Commit.Repo)
				}
				db.DB.Save(image.Commit)
			}
			if image.Installer != nil {
				image.Installer.Status = models.ImageStatusError
				db.DB.Save(image.Installer)
			}
			db.DB.Save(image)
		}

		// FIXME: this runs into an issue when only one of many pods is evicted and restarts...
		// If there any updates in progress, in the current architecture, we need to set them as errors because this is a brand new deployment
		var updates []models.UpdateTransaction
		db.DB.Where(&models.UpdateTransaction{Status: models.UpdateStatusBuilding}).Or(&models.UpdateTransaction{Status: models.UpdateStatusCreated}).Find(&updates)
		for _, update := range updates {
			log.WithField("updateID", update.ID).Debug("Found update with building status")
			update.Status = models.UpdateStatusError
			if update.Repo != nil {
				update.Repo.Status = models.RepoStatusError
				db.DB.Save(update.Repo)
			}
			db.DB.Save(update)
		}
	*/

	// Delete indexes first, before models AutoMigrate
	indexesToDelete := []struct {
		model     interface{}
		label     string
		indexName string
	}{
		{model: &models.ThirdPartyRepo{}, label: "ThirdPartyRepo", indexName: "idx_third_party_repos_name"},
	}
	for _, indexToDelete := range indexesToDelete {
		if db.DB.Migrator().HasIndex(indexToDelete.model, indexToDelete.indexName) {
			log.Debugf(`Model index %s "%s" exists deleting ...`, indexToDelete.label, indexToDelete.indexName)
			if err := db.DB.Migrator().DropIndex(indexToDelete.model, indexToDelete.indexName); err != nil {
				log.Warningf(`Model index %s "%s" deletion failure %s`, indexToDelete.label, indexToDelete.indexName, err)
				return err
			}
		} else {
			log.Debugf(`Model index %s "%s"  does not exist`, indexToDelete.label, indexToDelete.indexName)
		}
	}

	return nil
}
