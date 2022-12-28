// FIXME: golangci-lint
// nolint:gofmt,goimports,revive
package models

import (
	"fmt"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/pkg/db"

	"github.com/redhatinsights/edge-api/config"
)

func TestMain(m *testing.M) {
	setUp()
	retCode := m.Run()
	tearDown()
	os.Exit(retCode)
}

var dbName string

func setUp() {
	dbTimeCreation := time.Now().UnixNano()
	dbName = fmt.Sprintf("%d-models.db", dbTimeCreation)
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
		FDODevice{},
		OwnershipVoucherData{},
		FDOUser{},
		SSHKey{},
		DeviceGroup{},
	)

	if err != nil {
		panic(err)
	}

	var testImage = Image{
		Account:      "0000000",
		Status:       ImageStatusBuilding,
		Distribution: "rhel-8",
		Name:         "image_name_pre_exist",
		Commit:       &Commit{Arch: "x86_64"},
		OutputTypes:  []string{ImageTypeCommit},
	}
	db.DB.Create(&testImage)
}

func tearDown() {
	sqlDB, err := db.DB.DB()

	if err != nil {
		log.Info("Failed to acquire test database", err)
		panic(err)
	}

	err = sqlDB.Close()
	if err != nil {
		log.Info("Failed to close test database", err)
		return
	}

	err = os.Remove(dbName)
	if err != nil {
		log.Info("Failed to remove test database", err)
		return
	}
}
