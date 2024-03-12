package main

import (
	"net/http"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/cmd/migraterepos/migraterepos"
	"github.com/redhatinsights/edge-api/cmd/migraterepos/postmigraterepos"
	"github.com/redhatinsights/edge-api/cmd/migraterepos/repairrepos"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	edgeunleash "github.com/redhatinsights/edge-api/unleash"
	feature "github.com/redhatinsights/edge-api/unleash/features"

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
			unleash.WithCustomHeaders(http.Header{"Authorization": {cfg.FeatureFlagsAPIToken}}),
		)
		if err != nil {
			log.WithField("Error", err).Error("Unleash client failed to initialize")
		} else {
			log.WithField("FeatureFlagURL", cfg.UnleashURL).Info("Unleash client initialized successfully")
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

func cleanupAndExit(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func main() {

	initConfiguration()
	// wait for 5 seconds, for the unleash client to refresh
	time.Sleep(5 * time.Second)

	if feature.MigrateCustomRepositories.IsEnabled() {
		log.Info("custom repositories migration started")
		if _, err := repairrepos.RepairUrls(); err != nil {
			cleanupAndExit(err)
			return
		}

		if err := repairrepos.RepairDuplicateImagesReposURLS(); err != nil {
			cleanupAndExit(err)
			return
		}

		if err := repairrepos.RepairDuplicates(); err != nil {
			cleanupAndExit(err)
			return
		}

		if err := migraterepos.MigrateAllCustomRepositories(); err != nil {
			cleanupAndExit(err)
			return
		}
	} else {
		log.Info("custom repositories migration feature is disabled")
	}

	if feature.PostMigrateDeleteCustomRepositories.IsEnabled() {
		log.Info("post migrate delete custom repositories start")
		if _, err := postmigraterepos.PostMigrateDeleteCustomRepo(); err != nil {
			cleanupAndExit(err)
			return
		}
	}

	cleanupAndExit(nil)
}
