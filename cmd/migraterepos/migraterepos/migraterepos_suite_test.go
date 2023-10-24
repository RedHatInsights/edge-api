package migraterepos_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
)

func TestMigrate(t *testing.T) {
	RegisterFailHandler(Fail)
	dbName := setupTestDB()
	defer tearDownTestDB(dbName)
	RunSpecs(t, "Migrate custom repositories Suite")
}

func setupTestDB() string {
	config.Init()
	dbName := fmt.Sprintf("%d-migraterepos.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	if err := db.DB.AutoMigrate(
		&models.Image{},
		&models.ThirdPartyRepo{},
	); err != nil {
		panic(err)
	}
	return dbName
}

func tearDownTestDB(dbName string) {
	_ = os.Remove(dbName)
}
