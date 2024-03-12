// FIXME: golangci-lint
// nolint:errcheck,revive,typecheck
package routes

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Unleash/unleash-client-go/v4"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	mockUnleash "github.com/redhatinsights/edge-api/unleash"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
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
	defer func(exitCode int) {
		tearDown()
		os.Exit(exitCode)
	}(retCode)
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
			OrgID:  common.DefaultOrgID,
			Status: models.ImageStatusBuilding,
			InstalledPackages: []models.InstalledPackage{
				{Name: "vim"},
			},
		},
		Name:                  "Image Name in DB",
		TotalDevicesWithImage: 5,
		TotalPackages:         5,
	}
	db.DB.Create(&testImage.Commit)
	db.DB.Create(&testImage)
	testRepo = models.Repo{
		URL: "www.test.com",
	}
	db.DB.Create(&testRepo)
	db.DB.Create(&testUpdates)

	faker := mockUnleash.NewFakeUnleash()

	unleash.Initialize(
		unleash.WithListener(&unleash.DebugListener{}),
		unleash.WithAppName("my-application"),
		unleash.WithUrl(faker.URL()),
		unleash.WithRefreshInterval(1*time.Millisecond),
	)
	unleash.WaitForReady()
	faker.Enable(feature.FeatureCustomRepos)

	<-time.After(5 * time.Millisecond) // wait until client refreshes

}

func tearDown() {
	log.Info("removing routes main test db")
	if err := os.Remove(dbName); err != nil {
		log.Error(err.Error())
		log.Infof("failed to remove rotes db: %s", dbName)
	}
}
