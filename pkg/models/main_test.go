package models

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
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
	dbName = fmt.Sprintf("%d-models.db", time)
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		ImageSet{},
		Commit{},
		UpdateTransaction{},
		Package{},
		Image{},
		Repo{},
		Device{},
		DispatchRecord{},
	)
	if err != nil {
		panic(err)
	}
}

func tearDown() {
	os.Remove(dbName)
}
