// FIXME: golangci-lint
// nolint:revive
package main

import (
	"context"

	log "github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"gorm.io/gorm"

	"github.com/redhatinsights/edge-api/config"
)

func main() {
	ctx := context.Background()
	config.Init()
	cfg := config.Get()
	err := logger.InitializeLogging(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer logger.Flush()

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

	log.Info("Setting default account to existent devices...")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("UPDATE Devices set account = '6089719' WHERE account IS NULL OR account = ''")
	log.Info("Update Completed")
}
