package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/cmd/migrategroups/migrategroups"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	edgeunleash "github.com/redhatinsights/edge-api/unleash"

	"github.com/Unleash/unleash-client-go/v4"
	log "github.com/osbuild/logging/pkg/logrus"
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

func exitOnError(err error) {
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
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

	config.LogConfigAtStartup(cfg)
	db.InitDB()
	initializeUnleash()

	// wait for 5 seconds, for the unleash client to refresh
	time.Sleep(5 * time.Second)

	var mainErr error

	if err := migrategroups.MigrateAllGroups(db.DB); err != nil && err != migrategroups.ErrMigrationFeatureNotAvailable {
		// consider error only if feature flag is enabled
		mainErr = err
	}

	exitOnError(mainErr)
}
