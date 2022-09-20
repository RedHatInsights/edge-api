// FIXME: golangci-lint
// nolint:revive
package orgmigration_test

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

func TestMigrate(t *testing.T) {
	RegisterFailHandler(Fail)
	setupTestDB()
	RunSpecs(t, "Account to OrgID Migration Suite")
	tearDownTestDB()
}

var dbName string

func setupTestDB() {
	config.Init()
	config.Get().Debug = true
	dbName = fmt.Sprintf("%d-client.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()

	err := db.DB.AutoMigrate(
		&models.Commit{},
		&models.DeviceGroup{},
		&models.Device{},
		&models.Image{},
		&models.ImageSet{},
		&models.Installer{},
		&models.ThirdPartyRepo{},
		&models.UpdateTransaction{},
	)
	if err != nil {
		panic(err)
	}
}

func tearDownTestDB() {
	os.Remove(dbName)
}
