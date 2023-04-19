package main

import (
	"os"

	"github.com/redhatinsights/edge-api/cmd/migraterepos/repairrepos"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	log "github.com/sirupsen/logrus"
)

func initConfiguration() {
	config.Init()
	logger.InitLogger(os.Stdout)
	cfg := config.Get()
	config.LogConfigAtStartup(cfg)
	db.InitDB()
}

func handleExist(err error) {
	// flush logger before app exit
	logger.FlushLogger()
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func main() {

	initConfiguration()

	if feature.MigrateCustomRepositories.IsEnabled() {
		log.Info("custom repositories migration started")
		if _, err := repairrepos.RepairUrls(); err != nil {
			handleExist(err)
			return
		}

		if err := repairrepos.RepairDuplicates(); err != nil {
			handleExist(err)
			return
		}
	} else {
		log.Info("custom repositories migration feature is disabled")
	}

	handleExist(nil)
}
