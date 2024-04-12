// FIXME: golangci-lint
// nolint:revive,typecheck
package update_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/redhatinsights/edge-api/config"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestDevices(t *testing.T) {
	RegisterFailHandler(Fail)
	setUp()
	RunSpecs(t, "Update Suite")
	tearDown()
}

var dbName string

func setUp() {
	config.Init()
	time := time.Now().UnixNano()
	dbName = fmt.Sprintf("/tmp/%d-models.db", time)
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		models.UpdateTransaction{},
	)
	if err != nil {
		panic(err)
	}
}

func tearDown() {
	os.Remove(dbName)
}
