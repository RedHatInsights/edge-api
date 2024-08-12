package manual

import (
	"github.com/redhatinsights/edge-api/pkg/db"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerMigration("truncate packages if enabled (000)", truncatePackages)
}

func truncatePackages() error {
	if feature.TruncatePackages.IsEnabled() {
		log.Info("Truncating packages table ...")
		if err := db.DB.Exec("TRUNCATE TABLE commit_installed_packages, installed_packages").Error; err != nil {
			return err
		}

		if err := db.DB.Exec("VACUUM FULL").Error; err != nil {
			return err
		}
	}

	return nil
}
