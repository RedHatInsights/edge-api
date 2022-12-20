// FIXME: golangci-lint
// nolint:gocritic,govet,revive
package main

import (
	"os"

	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	log "github.com/sirupsen/logrus"
)

func handlePanic(errorOccurred *bool) {
	if err := recover(); err != nil {
		log.Error("Database automigrate failure")
		l.FlushLogger()
		os.Exit(1)
	}
}

func main() {
	config.Init()
	l.InitLogger(os.Stdout)
	configValues, err := config.GetConfigValues()
	if err != nil {
		l.LogErrorAndPanic("error when getting config values", err)
	}
	log.WithFields(configValues).Info("Configuration Values:")
	log.Info("Migration started ...")
	db.InitDB()

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
	// Automigration
	errorOccurred := false
	defer handlePanic(&errorOccurred)

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
				errorOccurred = true
			}
		} else {
			log.Debugf(`Model index %s "%s"  does not exist`, indexToDelete.label, indexToDelete.indexName)
		}
	}

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
		log.Debugf("Migrating Model %d: %s", modelsIndex, modelsInterface.label)

		err := db.DB.AutoMigrate(modelsInterface.interfaceInstance)
		if err != nil {
			log.Warningf("database automigrate failure %s", err)
			errorOccurred = true
		}
	}

	if !errorOccurred {
		log.Info("Migration completed successfully")
	} else {
		log.Error("Migration completed with errors")
	}
	// flush logger before app exit
	l.FlushLogger()
	if errorOccurred {
		os.Exit(2)
	}
}
