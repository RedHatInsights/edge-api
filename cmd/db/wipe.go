// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package main

import (
	"os"

	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/config"
)

func handlePanic(errorOccurred *bool) {
	if err := recover(); err != nil {
		log.Error("Database deletion failure")
		os.Exit(1)
	}
}
func main() {
	config.Init()
	l.InitLogger(os.Stdout)
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
	type ModelInterface struct {
		label             string
		interfaceInstance interface{}
	}
	var modelsInterfaces = make([]ModelInterface, 0)

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "Commit",
			interfaceInstance: &models.Commit{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "DeviceGroup",
			interfaceInstance: &models.DeviceGroup{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "DispatchRecord",
			interfaceInstance: &models.DispatchRecord{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "FDODevice",
			interfaceInstance: &models.FDODevice{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "FDOUser",
			interfaceInstance: &models.FDOUser{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "ImageSet",
			interfaceInstance: &models.ImageSet{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "Image",
			interfaceInstance: &models.Image{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "Installer",
			interfaceInstance: &models.Installer{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "OwnershipVoucherData",
			interfaceInstance: &models.OwnershipVoucherData{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "Package",
			interfaceInstance: &models.Package{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "Repo",
			interfaceInstance: &models.Repo{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "SSHKey",
			interfaceInstance: &models.SSHKey{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "ThirdPartyRepo",
			interfaceInstance: &models.ThirdPartyRepo{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "UpdateTransaction",
			interfaceInstance: &models.UpdateTransaction{}})

	for modelsIndex, modelsInterface := range modelsInterfaces {
		log.Debugf("Removing Model %d: %s", modelsIndex, modelsInterface.label)

		result := db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(modelsInterface.interfaceInstance)
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
