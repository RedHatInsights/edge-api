// FIXME: golangci-lint
// nolint:gosec,govet,revive
package orgmigration

import (
	"context"
	"errors"
	"time"

	"github.com/RedHatInsights/tenant-utils/pkg/tenantid"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	logger "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var log = logger.WithField("migration", "accountToOrgID")

type modelInterface struct {
	Label    string
	Model    interface{}
	Table    string
	whereSQL string
}

func getModelInterfaces() []modelInterface {
	return []modelInterface{
		{Label: "Commit", Model: models.Commit{}, Table: "commits"},
		{Label: "DeviceGroup", Model: models.DeviceGroup{}, Table: "device_groups"},
		{Label: "Devices", Model: models.Device{}, Table: "devices", whereSQL: "(image_id != 0 AND image_id IS NOT NULL)"},
		{Label: "Images", Model: models.Image{}, Table: "images"},
		{Label: "ImageSet", Model: models.ImageSet{}, Table: "image_sets"},
		{Label: "Installer", Model: models.Installer{}, Table: "installers"},
		{Label: "ThirdPartyRepo", Model: models.ThirdPartyRepo{}, Table: "third_party_repos"},
		{Label: "UpdateTransaction", Model: models.UpdateTransaction{}, Table: "update_transactions"},
	}
}

// DefaultLimit the limit used in sql queries, this the number of accounts we process at once
const DefaultLimit = 1000

// DefaultTranslatorTimeout the timeout passed to translator
const DefaultTranslatorTimeout = 120 * time.Second

// DefaultMaxBadAccounts do not tolerate more than this amount of bad account
// bad accounts are those accounts for which we cannot find an org_id
const DefaultMaxBadAccounts = 10

func dbAccountOrgScope(tx *gorm.DB) *gorm.DB {
	// run Unscoped to migrate all accounts in db even those with deletedAt not null
	return tx.Unscoped().Where("(account != '' AND account IS NOT NULL AND (org_id = '' OR org_id IS NULL))").Group("account")
}

func getModelTotalAccounts(modelInterface *modelInterface) (int64, error) {

	var accountsCount int64

	// run Unscoped to migrate all accounts in db even those with deletedAt
	tx := db.DB.Debug().Scopes(dbAccountOrgScope).Model(modelInterface.Model)
	if modelInterface.whereSQL != "" {
		tx = tx.Where(modelInterface.whereSQL)
	}

	if result := tx.Count(&accountsCount); result.Error != nil {
		log.Errorf(`model: %s, error getting accounts number : %s `, modelInterface.Label, result.Error)
		return 0, result.Error
	}
	return accountsCount, nil
}

func getModelAccounts(modelInterface *modelInterface, limit int, excludeAccounts []string) ([]string, error) {
	var accounts []string
	if limit == 0 {
		limit = DefaultLimit
	}

	log := log.WithFields(logger.Fields{
		"model": modelInterface.Label,
		"table": modelInterface.Table,
	})

	tx := db.DB.Debug().Scopes(dbAccountOrgScope).Model(modelInterface.Model)
	if modelInterface.whereSQL != "" {
		tx = tx.Where(modelInterface.whereSQL)
	}
	if len(excludeAccounts) > 0 {
		tx = tx.Where("account NOT IN (?)", excludeAccounts)
	}

	if result := tx.Order("account ASC").Limit(limit).Pluck("account", &accounts); result.Error != nil {
		log.WithField("error", result.Error.Error()).Error(`error getting model accounts`)
		return nil, result.Error
	}

	return accounts, nil
}

func migrateModelAccounts(translator tenantid.Translator, modelInterface modelInterface, accounts []string) ([]string, int64, error) {

	var badAccounts []string
	var affectedRecords int64

	if len(accounts) == 0 {
		return badAccounts, affectedRecords, nil
	}
	modelLog := log.WithFields(logger.Fields{
		"model": modelInterface.Label,
		"table": modelInterface.Table,
	})

	results, err := translator.EANsToOrgIDs(context.Background(), accounts)
	if err != nil {
		modelLog.WithField("error", err.Error()).Error(`error getting accounts org_id`)
		return badAccounts, affectedRecords, err
	}

	for _, result := range results {
		if result.EAN == nil {
			// this case may never happen as we have a defined account
			// we need to have this case, for the clarity of the code, and to not unwrap a nil pointer
			// in any other case we need to bind the error with the account
			if err != nil {
				modelLog.WithField("error", result.Err.Error()).Error(
					`could not translate to "org_id", one of the accounts got a fatal error`)
			} else {
				modelLog.Error(`could not translate to "org_id", one of the accounts got a silent fatal error`)
			}
			continue
		}
		account := *result.EAN
		modelLog = modelLog.WithField("account", account)
		if result.Err != nil {
			modelLog.WithField("error", result.Err.Error()).Error(`could not translate to "org_id"`)
			badAccounts = append(badAccounts, account)
			continue
		}

		orgID := result.OrgID
		modelLog = modelLog.WithField("org_id", orgID)

		// run Unscoped to migrate all accounts in db even those with deletedAt
		tx := db.DB.Debug().Unscoped().Model(&modelInterface.Model).Where("account = ?", account)
		if modelInterface.whereSQL != "" {
			tx = tx.Where(modelInterface.whereSQL)
		}

		updateResult := tx.Updates(map[string]interface{}{"org_id": result.OrgID})

		if updateResult.Error != nil {
			modelLog.WithField("error", updateResult.Error.Error()).Error(`error when updating org_id rows`)
			continue
		}

		if updateResult.RowsAffected == 0 {
			modelLog.Error(`could not translate to "org_id", account not found`)
			continue
		}
		affectedRecords += updateResult.RowsAffected
		modelLog.WithField("records_affected", updateResult.RowsAffected).Info(`successfully translated`)
	}

	return badAccounts, affectedRecords, nil
}

// removeBadAccounts filter account slice from bad accounts
func removeBadAccounts(badAccountsMap map[string]bool, accounts []string) []string {
	if len(accounts) == 0 {
		return accounts
	}
	newAccounts := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if _, ok := badAccountsMap[account]; !ok {
			newAccounts = append(newAccounts, account)
		}
	}
	return newAccounts
}

// MigrateAllModels fill org_id field of models by the corresponding account org_id value using translator
func MigrateAllModels(conf *config.EdgeConfig, translator *tenantid.Translator) error {
	if translator == nil {
		if conf.TenantTranslatorURL == "" {
			log.Info("translator and config TenantTranslatorURL not defined, nothing todo")
			return nil
		}

		log.WithField("tenant_translator_url", conf.TenantTranslatorURL).Info("Migrating models: account -> org_id")

		translatorOptions := []tenantid.TranslatorOption{
			tenantid.WithTimeout(DefaultTranslatorTimeout),
		}
		newTranslator := tenantid.NewTranslator(conf.TenantTranslatorURL, translatorOptions...)
		translator = &newTranslator
	}

	globalBadAccountsMap := make(map[string]bool)
	var globalBadAccounts []string
	for _, modelInterface := range getModelInterfaces() {
		modelTotalAccounts, err := getModelTotalAccounts(&modelInterface)
		modelLog := log.WithFields(logger.Fields{
			"model": modelInterface.Label,
			"table": modelInterface.Table,
		})
		if err != nil {
			modelLog.WithField("error", err.Error()).Errorf(`error getting total accounts mumber`)
			continue
		}
		modelLog = modelLog.WithFields(logger.Fields{
			"total_accounts": modelTotalAccounts,
		})
		if modelTotalAccounts == 0 {
			modelLog.Info("no account found to translate skiping")
			continue
		}

		modelLog.Info("starting translation")

		var modelTotalBadAccounts int
		var modelTotalProcessedAccounts int64
		var modelAffectedRecords int64
		for modelTotalProcessedAccounts < modelTotalAccounts {
			accounts, err := getModelAccounts(&modelInterface, DefaultLimit, globalBadAccounts)
			if err != nil {
				break
			}
			modelTotalProcessedAccounts += int64(len(accounts))

			accounts = removeBadAccounts(globalBadAccountsMap, accounts)
			if len(accounts) == 0 {
				break
			}

			badAccounts, affectedRecords, err := migrateModelAccounts(*translator, modelInterface, accounts)
			if err != nil {
				break
			}
			modelTotalBadAccounts += len(badAccounts)
			modelAffectedRecords += affectedRecords
			for _, badAccount := range badAccounts {
				globalBadAccountsMap[badAccount] = true
			}
			globalBadAccounts = append(globalBadAccounts, badAccounts...)
			if len(globalBadAccounts) >= DefaultMaxBadAccounts {
				// we do not tolerate that much bad accounts
				err := errors.New("max bad accounts reached")
				modelLog.WithFields(logger.Fields{"bad_accounts_count": len(globalBadAccounts), "bad_accounts": globalBadAccounts}).Infof(err.Error())
				return err
			}
		}

		modelLog.WithFields(logger.Fields{
			"total_accounts_processed": modelTotalProcessedAccounts,
			"total_bad_account":        modelTotalBadAccounts,
			"total_affected_records":   modelAffectedRecords,
		}).Infof("translation proccess finished")
	}

	if len(globalBadAccounts) > 0 {
		log.WithFields(logger.Fields{"bad_accounts": globalBadAccounts, "bad_accounts_count": len(globalBadAccounts)}).
			Warning("migration finished with some accounts not translated")
	} else {
		log.Info("migration finished successfully")
	}
	return nil
}
