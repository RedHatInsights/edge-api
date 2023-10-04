package migrategroups_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMigrate(t *testing.T) {
	RegisterFailHandler(Fail)
	dbName := setupTestDB()
	defer tearDownTestDB(dbName)
	RunSpecs(t, "Migrate device groups Suite")
}

func setupTestDB() string {
	config.Init()
	dbName := fmt.Sprintf("%d-migrategroups.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	if err := db.DB.AutoMigrate(
		&models.DeviceGroup{},
		&models.Device{},
	); err != nil {
		panic(err)
	}
	return dbName
}

func tearDownTestDB(dbName string) {
	_ = os.Remove(dbName)
}
