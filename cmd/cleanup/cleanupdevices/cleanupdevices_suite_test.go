package cleanupdevices_test

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
	RunSpecs(t, "Cleanup devices storage Suite")
	tearDownTestDB(dbName)
}

func setupTestDB() string {
	config.Init()
	config.Get().Debug = true
	dbName := fmt.Sprintf("%d-cleanupdevices.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		&models.Image{},
		&models.Commit{},
		&models.Installer{},
		&models.Repo{},
		&models.UpdateTransaction{},
		&models.DispatchRecord{},
		&models.Device{},
	)
	if err != nil {
		panic(err)
	}
	return dbName
}

func tearDownTestDB(dbName string) {
	_ = os.Remove(dbName)
}
