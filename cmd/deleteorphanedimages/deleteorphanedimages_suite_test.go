package main_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestMigrate(t *testing.T) {
	RegisterFailHandler(Fail)
	dbName := setupTestDB()
	RunSpecs(t, "Delete Orphaned Images Suite")
	tearDownTestDB(dbName)
}

func setupTestDB() string {
	config.Init()
	config.Get().Debug = true
	dbName := fmt.Sprintf("%d-deleteorphanedimages.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	return dbName
}

func tearDownTestDB(dbName string) {
	_ = os.Remove(dbName)
}
