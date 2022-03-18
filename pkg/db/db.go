package db

import (
	"fmt"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/logger"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB ORM variable
var DB *gorm.DB

// InitDB to configure database connectivity
func InitDB() {
	var err error
	var dia gorm.Dialector
	cfg := config.Get()

	if cfg.Database.Type == "pgsql" {
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d",
			cfg.Database.Hostname,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Name,
			cfg.Database.Port,
		)
		dia = postgres.Open(dsn)
	} else {
		dia = sqlite.Open(cfg.Database.Name)
	}

	DB, err = gorm.Open(dia, &gorm.Config{})
	if err != nil {
		logger.LogErrorAndPanic("failed to connect database", err)
	}

	var minorVersion string
	DB.Raw("SELECT version()").Scan(&minorVersion)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error()}).Error("error selecting version")
	}

	log.Infof("Postgres information: '%s'", minorVersion)
}
