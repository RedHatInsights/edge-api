package main

import (
	"context"
	"os"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	log "github.com/sirupsen/logrus"
)

func main() {
	config.Init()
	l.InitLogger(os.Stdout)
	log.Info("Starting deletion of orphaned images...")
	db.InitDB()

	imageService := services.NewImageService(
		context.Background(),
		log.WithField("service", "image"),
	)

	// TODO: where to get a list of third party repos?
	var repos []models.ThirdPartyRepo

	imageRepos, imageRepoErr := services.GetImageReposFromDB(common.DefaultOrgID, repos)

	if imageRepoErr != nil {
		log.Error("Could not retrieve image repositories from database")
		cleanupAndExit(imageRepoErr)
	}

	// find orphaned images
	for i := 0; i < len(*imageRepos); i++ {
		repo := (*imageRepos)[i]

		for j := 0; j < len(repo.Images); j++ {
			image := repo.Images[j]

			if image.ImageSetID == nil {
				deleteErr := imageService.DeleteImage(&image)
				if deleteErr != nil {
					log.WithFields(log.Fields{
						"name":    image.Name,
						"account": image.Account,
					}).Error("Attempted to delete image but failed")
				}
			}
		}
	}
}

func cleanupAndExit(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
