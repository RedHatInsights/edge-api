// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package main

import (
	"context"
	"os"

	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/cmd/migrate/manual"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func handlePanic() {
	if err := recover(); err != nil {
		log.Errorf("Database automigrate failure: %s", err)
		os.Exit(1)
	}
}

func main() {
	ctx := context.Background()
	config.Init()
	cfg := config.Get()
	err := logger.InitializeLogging(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer logger.Flush()

	configValues, err := config.GetConfigValues()
	if err != nil {
		logger.LogErrorAndPanic("error when getting config values", err)
	}
	log.WithFields(configValues).Info("Configuration Values:")
	db.InitDB()
	defer handlePanic()

	var realDbName string
	if result := db.DB.Raw("SELECT current_database()").Scan(&realDbName); result.Error == nil && realDbName == "postgres" {
		log.Warning("Migration attempted on 'postgres' database")
	}

	var errors []error

	// Manual migration
	log.Info("Manual migration started ...")

	// List functions in manual package and execute them
	errors = manual.Execute()

	// Automigration
	log.Info("Auto migration started ...")

	// Order should match model deletions in cmd/db/wipe.go
	// Order is not strictly alphabetical due to dependencies (e.g. Image needs ImageSet)
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
			label:             "Package",
			interfaceInstance: &models.Package{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "Repo",
			interfaceInstance: &models.Repo{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "StaticDeltaState",
			interfaceInstance: &models.StaticDeltaState{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "ThirdPartyRepo",
			interfaceInstance: &models.ThirdPartyRepo{}})

	modelsInterfaces = append(modelsInterfaces,
		ModelInterface{
			label:             "UpdateTransaction",
			interfaceInstance: &models.UpdateTransaction{}})

	for modelsIndex, modelsInterface := range modelsInterfaces {
		log.Debugf("Migrating Model %d: %s", modelsIndex, modelsInterface.label)

		err := db.DB.AutoMigrate(modelsInterface.interfaceInstance)
		if err != nil {
			log.Warningf("database automigrate failure %s", err)
			errors = append(errors, err)
		}
	}

	if len(errors) == 0 {
		log.Info("Migration completed successfully")
	} else {
		log.WithField("errors", errors).Error("Migration completed with errors")
		for _, err := range errors {
			log.Warn(err)
		}
	}

	if len(errors) > 0 {
		os.Exit(2)
	}
}
