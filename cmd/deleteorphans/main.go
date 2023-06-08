package main

import (
	"context"
	"errors"
	"os"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"github.com/sirupsen/logrus"
)

func main() {
	if !feature.DeleteOrphanedImages.IsEnabled() {
		message := "delete orphaned images feature is disabled"
		logrus.Error(message)
		cleanupAndExit(errors.New(message))
	}

	config.Init()
	logger.InitLogger(os.Stdout)
	logrus.Info("Starting deletion of orphaned images...")
	db.InitDB()

	imageService := services.NewImageService(
		context.Background(),
		logrus.WithField("service", "image"),
	)

	// TODO:
	//  - where to get a list of third party repos?
	//  - is this the right way to retrieve all images?
	var repos []models.ThirdPartyRepo
	imageRepos, imageRepoErr := services.GetImageReposFromDB(common.DefaultOrgID, repos)

	if imageRepoErr != nil {
		logrus.Error("Could not retrieve image repositories from database. Exiting...")
		cleanupAndExit(imageRepoErr)
	}

	reposLen := len(*imageRepos)
	if reposLen == 0 {
		logrus.Info("No repositories found, no orphaned images to delete. Exiting...")
		cleanupAndExit(nil)
	}

	// find orphaned images
	for i := 0; i < reposLen; i++ {
		repo := (*imageRepos)[i]

		for j := 0; j < len(repo.Images); j++ {
			image := repo.Images[j]

			if image.ImageSetID == nil {
				deleteErr := imageService.DeleteImage(&image)
				if deleteErr != nil {
					logrus.WithFields(logrus.Fields{
						"name":    image.Name,
						"account": image.Account,
					}).Error("Attempted to delete image but failed")
					cleanupAndExit(deleteErr)
				}
			}
		}
	}
}

func cleanupAndExit(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		logrus.Exit(2)
	}
	logrus.Exit(0)
}
