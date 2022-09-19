package routes

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Unleash/unleash-client-go/v3"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	mockUnleash "github.com/redhatinsights/edge-api/unleash"
	feature "github.com/redhatinsights/edge-api/unleash/features"
)

var (
	testImage models.Image
	testRepo  models.Repo
	tprepo    models.ThirdPartyRepo

	testDevices = []models.Device{
		{UUID: "1", AvailableHash: "11"},
		{UUID: "2", AvailableHash: "11"},
		{UUID: "3", AvailableHash: "22"},
		{UUID: "4", AvailableHash: "12"},
	}

	testUpdates = []models.UpdateTransaction{
		{
			Account: "0000000",
			Devices: []models.Device{testDevices[0], testDevices[1]},
		},
		{
			Account: "0000001",
			Devices: []models.Device{testDevices[2], testDevices[3]},
		},
	}
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
	dbName = fmt.Sprintf("%d-routes.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		&models.Commit{},
		&models.UpdateTransaction{},
		&models.Package{},
		&models.Image{},
		&models.ImageSet{},
		&models.Repo{},
		&models.Device{},
		&models.DispatchRecord{},
		&models.ThirdPartyRepo{},
		&models.DeviceGroup{},
	)
	if err != nil {
		panic(err)
	}
	testImage = models.Image{
		Account: "0000000",
		Status:  models.ImageStatusBuilding,
		Commit: &models.Commit{
			Status: models.ImageStatusBuilding,
		},
		Name: "Image Name in DB",
	}
	db.DB.Create(&testImage.Commit)
	db.DB.Create(&testImage)
	testRepo = models.Repo{
		URL: "www.test.com",
	}
	db.DB.Create(&testRepo)
	db.DB.Create(&testUpdates)

	faker := mockUnleash.NewFakeUnleash()

	// FIXME: golangci-lint
	unleash.Initialize(
		unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName("my-application"),
		unleash.WithUrl(faker.URL()),
		unleash.WithRefreshInterval(1*time.Millisecond),
	) // nolint:errcheck,revive
	unleash.WaitForReady()
	faker.Enable(feature.FeatureCustomRepos)

	<-time.After(5 * time.Millisecond) // wait until client refreshes

}

func tearDown() {
	db.DB.Exec("DELETE FROM commits")
	db.DB.Exec("DELETE FROM repos")
	db.DB.Exec("DELETE FROM images")
	db.DB.Exec("DELETE FROM update_transactions")
	os.Remove(dbName)
}
