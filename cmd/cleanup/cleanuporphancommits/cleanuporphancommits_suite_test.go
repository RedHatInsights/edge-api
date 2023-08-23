package cleanuporphancommits_test

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

func TestCleanupOrphanCommits(t *testing.T) {
	RegisterFailHandler(Fail)
	dbName := setupTestDB()
	RunSpecs(t, "Cleanup orphan commits Suite")
	tearDownTestDB(dbName)
}

func setupTestDB() string {
	config.Init()
	config.Get().Debug = true
	dbName := fmt.Sprintf("%d-cleanuporphancommits.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		&models.Image{},
		&models.Commit{},
		&models.Repo{},
		&models.UpdateTransaction{},
	)
	if err != nil {
		panic(err)
	}
	return dbName
}

func tearDownTestDB(dbName string) {
	_ = os.Remove(dbName)
}
