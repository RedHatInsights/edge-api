package main

import (
	"os"

	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func handlePanic(errorOccurred *bool) {
	if err := recover(); err != nil {
		log.Error("Database deletion failure")
		os.Exit(1)
	}
}
func main() {
	config.Init()
	l.InitLogger()
	cfg := config.Get()
	log.WithFields(log.Fields{
		"Hostname":                 cfg.Hostname,
		"Auth":                     cfg.Auth,
		"WebPort":                  cfg.WebPort,
		"MetricsPort":              cfg.MetricsPort,
		"LogLevel":                 cfg.LogLevel,
		"Debug":                    cfg.Debug,
		"BucketName":               cfg.BucketName,
		"BucketRegion":             cfg.BucketRegion,
		"RepoTempPath ":            cfg.RepoTempPath,
		"OpenAPIFilePath ":         cfg.OpenAPIFilePath,
		"ImageBuilderURL":          cfg.ImageBuilderConfig.URL,
		"DefaultOSTreeRef":         cfg.DefaultOSTreeRef,
		"InventoryURL":             cfg.InventoryConfig.URL,
		"PlaybookDispatcherConfig": cfg.PlaybookDispatcherConfig.URL,
		"TemplatesPath":            cfg.TemplatesPath,
		"DatabaseType":             cfg.Database.Type,
		"DatabaseName":             cfg.Database.Name,
	}).Info("Configuration Values:")
	db.InitDB()

	log.Info("Cleaning up relations and old tables (which may not exist)...")
	errorOccurred := false
	defer handlePanic(&errorOccurred)

	var sqlStatements = make([]string, 0)
	sqlStatements = append(sqlStatements, "DELETE FROM commit_packages")

	sqlStatements = append(sqlStatements, "DELETE FROM commit_installed_packages")

	sqlStatements = append(sqlStatements, "DELETE FROM device_groups_devices")

	sqlStatements = append(sqlStatements, "DELETE FROM images_packages")

	sqlStatements = append(sqlStatements, "DELETE FROM updatetransaction_devices")

	sqlStatements = append(sqlStatements, "DROP TABLE updaterecord_commits")

	sqlStatements = append(sqlStatements, "DROP TABLE updaterecord_devices")

	sqlStatements = append(sqlStatements, "DROP TABLE update_records")

	sqlStatements = append(sqlStatements, "ALTER TABLE commits DROP CONSTRAINT fk_commits_repo")

	for sqlIndex, sqlStatement := range sqlStatements {
		log.Debugf("Running SQL statement %d: %s", sqlIndex, sqlStatement)
		sqlResult := db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec(sqlStatement)
		if sqlResult.Error != nil {
			log.Warningf("SQL statement failure %s", sqlResult.Error)
			errorOccurred = true
		}
	}

	log.Info("Starting deleting of models...")
	// Order is empircally determine and should be inverse of Automigration cmd/migrate/main.go
	var modelsInterfaces = make([]interface{}, 0)
	var modelsLabels = make([]string, 0)

	modelsInterfaces = append(modelsInterfaces, &models.DeviceGroup{})
	modelsLabels = append(modelsLabels, "DeviceGroup")

	modelsInterfaces = append(modelsInterfaces, &models.DispatchRecord{})
	modelsLabels = append(modelsLabels, "DispatchRecord")

	modelsInterfaces = append(modelsInterfaces, &models.FDODevice{})
	modelsLabels = append(modelsLabels, "FDODevice")

	modelsInterfaces = append(modelsInterfaces, &models.FDOUser{})
	modelsLabels = append(modelsLabels, "FDOUser")

	modelsInterfaces = append(modelsInterfaces, &models.ImageSet{})
	modelsLabels = append(modelsLabels, "ImageSet")

	modelsInterfaces = append(modelsInterfaces, &models.Image{})
	modelsLabels = append(modelsLabels, "Image")

	modelsInterfaces = append(modelsInterfaces, &models.Commit{})
	modelsLabels = append(modelsLabels, "Commit")

	modelsInterfaces = append(modelsInterfaces, &models.Installer{})
	modelsLabels = append(modelsLabels, "Installer")

	modelsInterfaces = append(modelsInterfaces, &models.OwnershipVoucherData{})
	modelsLabels = append(modelsLabels, "OwnershipVoucherData")

	modelsInterfaces = append(modelsInterfaces, &models.Package{})
	modelsLabels = append(modelsLabels, "Package")

	modelsInterfaces = append(modelsInterfaces, &models.Repo{})
	modelsLabels = append(modelsLabels, "Repo")

	modelsInterfaces = append(modelsInterfaces, &models.SSHKey{})
	modelsLabels = append(modelsLabels, "SSHKey")

	modelsInterfaces = append(modelsInterfaces, &models.ThirdPartyRepo{})
	modelsLabels = append(modelsLabels, "ThirdPartyRepo")

	modelsInterfaces = append(modelsInterfaces, &models.UpdateTransaction{})
	modelsLabels = append(modelsLabels, "UpdateTransaction")

	for modelsIndex, modelsInterface := range modelsInterfaces {
		log.Debugf("Removing Model %s", modelsLabels[modelsIndex])
		result := db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(modelsInterface)
		if result.Error != nil {
			log.Warningf("database delete failure %s", result.Error)
			errorOccurred = true
		}
	}

	if !errorOccurred {
		log.Info("Database wipe completed")
	} else {
		log.Error("Database wipe completed with errors")
		os.Exit(2)
	}
}
