package db_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

// This will setup the test database and run the tests for whole package
func TestMain(m *testing.M) {
	setupTestDB()
	retCode := m.Run()
	tearDownTestDB()
	os.Exit(retCode)
}

var dbName string

func setupTestDB() {
	config.Init()
	config.Get().Debug = true
	time := time.Now().UnixNano()
	dbName = fmt.Sprintf("%d-services.db", time)
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		&models.Device{},
	)
	if err != nil {
		panic(err)
	}
}

func tearDownTestDB() {
	os.Remove(dbName)
}
