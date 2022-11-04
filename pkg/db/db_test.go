// FIXME: golangci-lint
// nolint:revive,typecheck
package db_test

import (
	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
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
})
