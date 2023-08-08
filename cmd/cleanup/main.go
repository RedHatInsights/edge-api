package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/cmd/cleanup/cleanupdevices"
	"github.com/redhatinsights/edge-api/cmd/cleanup/cleanupimages"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/services/files"

	edgeunleash "github.com/redhatinsights/edge-api/unleash"

	"github.com/Unleash/unleash-client-go/v3"
	log "github.com/sirupsen/logrus"
)

func initializeUnleash() {
	cfg := config.Get()
	if cfg.FeatureFlagsURL != "" {
		err := unleash.Initialize(
			unleash.WithListener(&edgeunleash.EdgeListener{}),
			unleash.WithAppName("edge-api"),
			unleash.WithUrl(cfg.UnleashURL),
			unleash.WithRefreshInterval(5*time.Second),
			unleash.WithMetricsInterval(5*time.Second),
			unleash.WithCustomHeaders(http.Header{"Authorization": {fmt.Sprintf("Bearer %s", cfg.UnleashSecretName)}}),
		)
		if err != nil {
			log.WithField("Error", err).Error("Unleash client failed to initialize")
		} else {
			log.WithField("FeatureFlagURL", cfg.UnleashURL).Info("Unleash client initialized successfully")
			unleash.WaitForReady()
		}
	} else {
		log.WithField("FeatureFlagURL", cfg.UnleashURL).Warning("FeatureFlag service initialization was skipped.")
	}
}

func initConfiguration() {
	config.Init()
	logger.InitLogger(os.Stdout)
	cfg := config.Get()
	config.LogConfigAtStartup(cfg)
	db.InitDB()
	initializeUnleash()
}

func flushLogAndExit(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func main() {
	initConfiguration()

	client := files.GetNewS3Client()

	var mainErr error

	if err := cleanupimages.CleanUpAllImages(client); err != nil {
		mainErr = err
	}

	if err := cleanupdevices.CleanupAllDevices(client, db.DB); err != nil {
		mainErr = err
	}

	flushLogAndExit(mainErr)
}
