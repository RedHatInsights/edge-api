package services

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	tearDown()
	os.Exit(retCode)
}

var dbName string

func setUp() {
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
	)
	if err != nil {
		panic(err)
	}
}

func tearDown() {
	os.Remove(dbName)
}
