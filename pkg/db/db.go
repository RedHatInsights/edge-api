// FIXME: golangci-lint
// nolint:gosimple,revive
package db

import (
	"context"
	"fmt"

	edgelogger "github.com/redhatinsights/edge-api/logger"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/redhatinsights/edge-api/config"
)

// DB ORM variable
var DB *gorm.DB

// CreateDB create a new application DB
func CreateDB() (*gorm.DB, error) {
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

	var newDB *gorm.DB
	var err error
	if feature.SilentGormLogging.IsEnabled() {
		newDB, err = gorm.Open(dia, &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
	} else {
		newDB, err = gorm.Open(dia, &gorm.Config{
			Logger: edgelogger.NewGormLogger(log.StandardLogger()),
		})
	}

	if err != nil {
		edgelogger.LogErrorAndPanic("failed to connect database", err)
		return nil, err
	}

	if cfg.Database.Type == "pgsql" {
		var minorVersion string
		if result := newDB.Raw("SELECT version()").Scan(&minorVersion); result.Error != nil {
			log.WithFields(log.Fields{"error": result.Error.Error()}).Error("error selecting version")
			return nil, result.Error
		}

		log.Infof("Postgres information: '%s'", minorVersion)
	}

	return newDB, nil
}

// InitDB to configure database connectivity
func InitDB() {
	newDB, err := CreateDB()
	if err != nil {
		return
	}
	DB = newDB
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

// DBX returns a gorm db query with context
func DBx(ctx context.Context) *gorm.DB {
	return DB.WithContext(ctx)
}

// Org returns a gorm db query with orgID filter
func Org(orgID string, table string) *gorm.DB {
	return OrgDB(orgID, DB, table)
}

// Orgx returns a gorm db query with orgID filter with context
func Orgx(ctx context.Context, orgID string, table string) *gorm.DB {
	return Org(orgID, table).WithContext(ctx)
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

// OrgDBx returns a gorm db with orgID filter from a known gorm db query with context
func OrgDBx(ctx context.Context, orgID string, gormDB *gorm.DB, table string) *gorm.DB {
	return OrgDB(orgID, gormDB, table).WithContext(ctx)
}
