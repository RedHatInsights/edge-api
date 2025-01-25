package manual

import (
	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func init() {
	registerMigration("drop installed package timestamps (001)", dropInstalledPackageTimestamps001)
}

func dropInstalledPackageTimestamps001() error {
	if db.DB.Migrator().HasColumn(&models.InstalledPackage{}, "created_at") {
		log.Info("Dropping created_at column from installed_packages")
		err := db.DB.Migrator().DropColumn(&models.InstalledPackage{}, "created_at")
		if err != nil {
			return err
		}
	} else {
		log.Info("Column created_at does not exist in installed_packages")
	}

	if db.DB.Migrator().HasColumn(&models.InstalledPackage{}, "deleted_at") {
		log.Info("Dropping deleted_at column from installed_packages")
		err := db.DB.Migrator().DropColumn(&models.InstalledPackage{}, "deleted_at")
		if err != nil {
			return err
		}
	} else {
		log.Info("Column deleted_at does not exist in installed_packages")
	}

	if db.DB.Migrator().HasColumn(&models.InstalledPackage{}, "updated_at") {
		log.Info("Dropping updated_at column from installed_packages")
		err := db.DB.Migrator().DropColumn(&models.InstalledPackage{}, "updated_at")
		if err != nil {
			return err
		}
	} else {
		log.Info("Column updated_at does not exist in installed_packages")
	}

	return nil
}
