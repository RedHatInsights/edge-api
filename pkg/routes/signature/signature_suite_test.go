package signature_test

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

func TestClients(t *testing.T) {
	RegisterFailHandler(Fail)
	setUp()
	RunSpecs(t, "Signature Suite")
	tearDown()
}

var dbName string

func setUp() {
	config.Init()
	config.Get().Debug = true
	dbName = fmt.Sprintf("%d-routes.db", time.Now().UnixNano())
	config.Get().Database.Name = dbName
	db.InitDB()
	err := db.DB.AutoMigrate(
		&models.UpdateTransaction{},
	)
	if err != nil {
		panic(err)
	}

}

func tearDown() {
	db.DB.Exec("DELETE FROM update_transactions")
	os.Remove(dbName)
}
