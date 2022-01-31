package main

import (
	"github.com/redhatinsights/edge-api/config"
	l "github.com/redhatinsights/edge-api/logger"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	config.Init()
	l.InitLogger()
	cfg := config.Get()
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
		"DefaultOSTreeRef":         cfg.DefaultOSTreeRef,
		"InventoryURL":             cfg.InventoryConfig.URL,
		"PlaybookDispatcherConfig": cfg.PlaybookDispatcherConfig.URL,
		"TemplatesPath":            cfg.TemplatesPath,
		"DatabaseType":             cfg.Database.Type,
		"DatabaseName":             cfg.Database.Name,
	}).Info("Configuration Values:")
	db.InitDB()

	log.Info("Cleaning up relations and old tables...")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DELETE FROM images_packages")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DELETE FROM commit_packages")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DELETE FROM updatetransaction_devices")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DELETE FROM commit_installed_packages")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DELETE FROM devices_groups")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DROP TABLE updaterecord_commits")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DROP TABLE updaterecord_devices")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("DROP TABLE update_records")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Exec("ALTER TABLE commits DROP CONSTRAINT fk_commits_repo")

	log.Info("Starting deleting of models...")
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.Image{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.ImageSet{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.Installer{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.Commit{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.Repo{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.UpdateTransaction{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.Package{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.DispatchRecord{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.ThirdPartyRepo{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.FDODevice{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.OwnershipVoucherData{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.FDOUser{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.SSHKey{})
	db.DB.Session(&gorm.Session{AllowGlobalUpdate: true}).Unscoped().Delete(&models.Group{})
	log.Info("Wipe Completed")
}
