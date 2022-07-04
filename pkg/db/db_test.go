package db_test

import (
	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var _ = Describe("Db", func() {
	Context("get orgID from device table", func() {
		It("should return OrgID", func() {
			device := []models.Device{
				{
					Name:  faker.Name(),
					UUID:  faker.UUIDHyphenated(),
					OrgID: common.DefaultOrgID,
				},
			}
			sql := db.DB.ToSQL(func(tx *gorm.DB) *gorm.DB {
				return db.OrgTx("00000000", tx, "").Find(&device)
			})
			Expect(sql).ToNot(BeNil())
			log.Info(">>>>", sql)
		})
	})
})