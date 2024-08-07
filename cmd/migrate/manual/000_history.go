package manual

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerMigration("History", Migrate000History)
}

// Migrate000History is a manual migration that appears to originally have been intended to
// drop indices before auto-migration runs. The database used to be migrated
// by changing ./cmd/migrate/main.go and running the migration. This is the last
// block that was present when we introduced the new migration system.
// For more details see the git history.
func Migrate000History() error {
	// Delete indexes first, before models AutoMigrate
	indexesToDelete := []struct {
		model     interface{}
		label     string
		indexName string
	}{
		{
			model:     &models.ThirdPartyRepo{},
			label:     "ThirdPartyRepo",
			indexName: "idx_third_party_repos_name",
		},
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
