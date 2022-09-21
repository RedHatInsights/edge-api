// FIXME: golangci-lint
// nolint:gosimple,revive
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

	if cfg.Database.Type == "pgsql" {
		var minorVersion string
		DB.Raw("SELECT version()").Scan(&minorVersion)
		if err != nil {
			log.WithFields(log.Fields{"error": err.Error()}).Error("error selecting version")
		}

		log.Infof("Postgres information: '%s'", minorVersion)
	}
}

// AccountOrOrg returns a gorm db transaction with account or orgID filter
func AccountOrOrg(account string, orgID string, table string) *gorm.DB {
	return AccountOrOrgTx(account, orgID, DB, table)
}

// AccountOrOrgTx returns a gorm db with account or orgID filter from a known gorm db transaction
func AccountOrOrgTx(account string, orgID string, tx *gorm.DB, table string) *gorm.DB {
	if tx == nil {
		return nil
	}
	accountName := "account"
	orgIDName := "org_id"
	if table != "" {
		accountName = fmt.Sprintf("%s.%s", table, accountName)
		orgIDName = fmt.Sprintf("%s.%s", table, orgIDName)
	}
	sqlText := fmt.Sprintf(
		"((%s = ? AND (%s != '' AND %s IS NOT NULL)) OR (%s = ? AND (%s != '' AND %s IS NOT NULL)))",
		accountName, accountName, accountName,
		orgIDName, orgIDName, orgIDName,
	)
	return tx.Where(sqlText, account, orgID)
}

// Org returns a gorm db query with orgID filter
func Org(orgID string, table string) *gorm.DB {
	return OrgDB(orgID, DB, table)
}

// OrgDB returns a gorm db with orgID filter from a known gorm db query
func OrgDB(orgID string, gormDB *gorm.DB, table string) *gorm.DB {
	if gormDB == nil {
		return nil
	}
	orgIDName := "org_id"
	if table != "" {
		orgIDName = fmt.Sprintf("%s.%s", table, orgIDName)
	}
	if orgIDName == "" {
		return nil
	}
	var sqlText string

	sqlText = fmt.Sprintf(
		"(%s = ? )",
		orgIDName,
	)

	return gormDB.Where(sqlText, orgID)
}
