package routes

import (
	"os"
	"testing"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var (
	testImage models.Image
	testRepo  models.Repo

	updateDevices = []models.Device{
		{UUID: "1", DesiredHash: "11"},
		{UUID: "2", DesiredHash: "11"},
		{UUID: "3", DesiredHash: "22"},
		{UUID: "4", DesiredHash: "12"},
	}

	updateTrans = []models.UpdateTransaction{
		{
			Account: "0000000",
			Devices: []models.Device{updateDevices[0], updateDevices[1]},
		},
		{
			Account: "0000001",
			Devices: []models.Device{updateDevices[2], updateDevices[3]},
		},
	}
)

func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	tearDown()
	os.Exit(retCode)
}

func setUp() {
	config.Init()
	config.Get().Debug = true
	db.InitDB()

}

func tearDown() {
}
