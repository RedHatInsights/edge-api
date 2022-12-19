// FIXME: golangci-lint
// nolint: revive,typecheck
package db_test

import (
	"fmt"
	"os"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gorm.io/gorm"
)

var _ = Describe("Db", func() {
	Context("get orgID from devices table", func() {
		It("should return OrgID", func() {
			device := []models.Device{
				{
					Name:  faker.Name(),
					UUID:  faker.UUIDHyphenated(),
					OrgID: common.DefaultOrgID,
				},
			}
			sql := db.DB.ToSQL(func(gormDB *gorm.DB) *gorm.DB {
				return db.OrgDB("00000000", gormDB, "").Find(&device)
			})
			Expect(sql).To(ContainSubstring("org_id = \"00000000\""))
		})
	})
	Context("get orgID from images table", func() {
		It("should return OrgID", func() {
			var images []models.Image
			sql := db.DB.ToSQL(func(gormDB *gorm.DB) *gorm.DB {
				return db.OrgDB("00000000", gormDB, "images").Find(&images)
			})
			Expect(sql).To(ContainSubstring("images.org_id = \"00000000\""))
		})
	})
	Context("create CreateDB", func() {
		var localDB *gorm.DB
		var err error
		var dbName string
		var originalDBName string

		BeforeEach(func() {
			dbTimeCreation := time.Now().UnixNano()
			dbName = fmt.Sprintf("%d-dbtest.db", dbTimeCreation)
			originalDBName = config.Get().Database.Name
			config.Get().Database.Name = dbName
		})
		AfterEach(func() {
			config.Get().Database.Name = originalDBName

			sqlDB, err := localDB.DB()
			Expect(err).ToNot(HaveOccurred())
			Expect(sqlDB).ToNot(BeNil())

			err = sqlDB.Close()
			Expect(err).ToNot(HaveOccurred())

			err = os.Remove(dbName)
			Expect(err).ToNot(HaveOccurred())
		})
		It("should create new database", func() {
			localDB, err = db.CreateDB()

			Expect(err).ToNot(HaveOccurred())
			Expect(localDB).ToNot(BeNil())
		})
		It("should initialize the application db", func() {
			currentDB := db.DB

			db.InitDB()
			Expect(db.DB).ToNot(BeNil())
			Expect(db.DB).ToNot(Equal(currentDB))
			localDB = db.DB
		})
	})
})
