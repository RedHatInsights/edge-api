// FIXME: golangci-lint
// nolint:gofmt,goimports,revive
package db

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
)

// This will set up the test database and run the tests for whole package
func TestMain(m *testing.M) {
	setupTestDB()
	retCode := m.Run()
	tearDownTestDB()
	os.Exit(retCode)
}

var dbName string

func setupTestDB() {
	dbTimeCreation := time.Now().UnixNano()
	dbName = fmt.Sprintf("%d-services.db", dbTimeCreation)
	config.Get().Database.Name = dbName
	InitDB()
}

func tearDownTestDB() {
	sqlDB, err := DB.DB()

	if err != nil {
		log.Info("Failed to open acquire test database", err)
		panic(err)
	}

	err = sqlDB.Close()
	if err != nil {
		log.Info("Failed to close test database", err)
		return
	}

	err = os.Remove(dbName)
	if err != nil {
		log.Info("Failed to remove test database", err)
		return
	}
}
