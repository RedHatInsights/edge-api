package repairrepos_test

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

	RunSpecs(t, "Repair custom repositories Suite")
}

func setupTestDB() string {
	config.Init()
	config.Get().Debug = true
	dbName := fmt.Sprintf("%d-repairrepos.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		&models.Image{},
		&models.ThirdPartyRepo{},
	)
	if err != nil {
		panic(err)
	}
	return dbName
}

func tearDownTestDB(dbName string) {
	_ = os.Remove(dbName)
}
