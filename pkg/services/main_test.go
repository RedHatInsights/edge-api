// FIXME: golangci-lint
// nolint:revive
package services

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/redhatinsights/edge-api/config"
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
		&models.ImageSet{},
		&models.Commit{},
		&models.UpdateTransaction{},
		&models.Package{},
		&models.Image{},
		&models.Repo{},
		&models.Device{},
		&models.DispatchRecord{},
		&models.FDODevice{},
		&models.OwnershipVoucherData{},
		&models.FDOUser{},
		&models.SSHKey{},
		&models.DeviceGroup{},
	)
	if err != nil {
		panic(err)
	}
}

func tearDownTestDB() {
	os.Remove(dbName)
}
