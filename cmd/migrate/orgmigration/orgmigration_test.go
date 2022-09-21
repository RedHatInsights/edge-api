// FIXME: golangci-lint
// nolint:revive
package orgmigration_test

import (
	"reflect"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"
	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/cmd/migrate/orgmigration"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func getModelInterfaceFieldValue(modelInterface interface{}, fieldName string) string {
	val := reflect.ValueOf(modelInterface).Elem()
	fieldValue := val.FieldByName(fieldName)
	if fieldValue.Kind() == 0 {
		return ""
	}
	return fieldValue.String()
}

func getAccount(modelInterface interface{}) string {
	return getModelInterfaceFieldValue(modelInterface, "Account")
}

func getOrgID(modelInterface interface{}) string {
	return getModelInterfaceFieldValue(modelInterface, "OrgID")
}

func getID(modelInterface interface{}) uint {
	val := reflect.ValueOf(modelInterface).Elem()
	fieldValue := val.FieldByName("ID")
	if fieldValue.Kind() == 0 {
		return 0
	}
	return fieldValue.Interface().(uint)
}

var _ = Describe("Main", func() {
	var translator tenantid.Translator

	account := faker.UUIDHyphenated()
	accountOrg := faker.UUIDHyphenated()

	account2 := faker.UUIDHyphenated()
	account2Org := faker.UUIDHyphenated()

	accountNotExists := "DOES_NOT_EXIT"
	accountEmpty := ""

	mappings := map[string]*string{
		accountOrg:  &account,
		account2Org: &account2,
	}
	translator = tenantid.NewTranslatorMockWithMapping(mappings)

	type modelData struct {
		modelValue      interface{}
		expectedOrgID   string
		expectedAccount string
	}

	deviceImage := models.Image{Name: faker.Name()}
	db.DB.Session(&gorm.Session{SkipHooks: true}).Create(&deviceImage)

	modelsDataMap := map[string][]modelData{
		"commits": {
			{modelValue: &models.Commit{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.Commit{Name: faker.Name(), Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.Commit{Name: faker.Name(), Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.Commit{Name: faker.Name(), Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.Commit{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
		"device_groups": {
			{modelValue: &models.DeviceGroup{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.DeviceGroup{Name: faker.Name(), Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.DeviceGroup{Name: faker.Name(), Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.DeviceGroup{Name: faker.Name(), Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.DeviceGroup{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
		"devices": {
			{modelValue: &models.Device{Name: faker.Name(), Account: account, ImageID: deviceImage.ID}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.Device{Name: faker.Name(), Account: account2, ImageID: deviceImage.ID}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.Device{Name: faker.Name(), Account: accountNotExists, ImageID: deviceImage.ID}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.Device{Name: faker.Name(), Account: accountEmpty, ImageID: deviceImage.ID}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.Device{Name: faker.Name(), Account: account, ImageID: deviceImage.ID}, expectedOrgID: accountOrg, expectedAccount: account},
			// devices without images should not be updated
			{modelValue: &models.Device{Name: faker.Name(), Account: account}, expectedOrgID: "", expectedAccount: account},
			{modelValue: &models.Device{Name: faker.Name(), Account: account2}, expectedOrgID: "", expectedAccount: account2},
			{modelValue: &models.Device{Name: faker.Name(), Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.Device{Name: faker.Name(), Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.Device{Name: faker.Name(), Account: account}, expectedOrgID: "", expectedAccount: account},
		},
		"images": {
			{modelValue: &models.Image{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.Image{Name: faker.Name(), Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.Image{Name: faker.Name(), Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.Image{Name: faker.Name(), Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.Image{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
		"image_sets": {
			{modelValue: &models.ImageSet{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.ImageSet{Name: faker.Name(), Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.ImageSet{Name: faker.Name(), Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.ImageSet{Name: faker.Name(), Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.ImageSet{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
		"installers": {
			{modelValue: &models.Installer{Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.Installer{Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.Installer{Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.Installer{Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.Installer{Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
		"third_party_repos": {
			{modelValue: &models.ThirdPartyRepo{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.ThirdPartyRepo{Name: faker.Name(), Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.ThirdPartyRepo{Name: faker.Name(), Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.ThirdPartyRepo{Name: faker.Name(), Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.ThirdPartyRepo{Name: faker.Name(), Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
		"update_transactions": {
			{modelValue: &models.UpdateTransaction{Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
			{modelValue: &models.UpdateTransaction{Account: account2}, expectedOrgID: account2Org, expectedAccount: account2},
			{modelValue: &models.UpdateTransaction{Account: accountNotExists}, expectedOrgID: "", expectedAccount: accountNotExists},
			{modelValue: &models.UpdateTransaction{Account: accountEmpty}, expectedOrgID: "", expectedAccount: accountEmpty},
			{modelValue: &models.UpdateTransaction{Account: account}, expectedOrgID: accountOrg, expectedAccount: account},
		},
	}

	It("All records created successfully", func() {
		for _, modelsData := range modelsDataMap {
			for _, modelRecord := range modelsData {
				result := db.DB.Session(&gorm.Session{SkipHooks: true}).Debug().Create(modelRecord.modelValue)
				Expect(result.Error).ToNot(HaveOccurred())
			}
		}
	})

	It("Migration finished successfully", func() {
		err := orgmigration.MigrateAllModels(config.Get(), &translator)
		Expect(err).ToNot(HaveOccurred())
	})

	It("ensure all tables migrated as expected", func() {
		for tableName, modelsData := range modelsDataMap {
			log.Debugf("Checking table: %s", tableName)
			Expect(len(modelsData) > 0).To(BeTrue())
			for _, modelRecord := range modelsData {
				Expect(modelRecord.modelValue).ToNot(BeNil())
				result := db.DB.Debug().First(modelRecord.modelValue, getID(modelRecord.modelValue))
				Expect(result.Error).ToNot(HaveOccurred())
				Expect(getAccount(modelRecord.modelValue)).To(Equal(modelRecord.expectedAccount))
				Expect(getOrgID(modelRecord.modelValue)).To(Equal(modelRecord.expectedOrgID))
			}
		}
	})
})
