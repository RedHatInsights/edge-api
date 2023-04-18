package repairrepos

import (
	"errors"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	log "github.com/sirupsen/logrus"
)

var ErrMigrationNotAvailable = errors.New("migration is not available")

// RepairUrls add slash to urls to conform to content-sources urls
// any url like "http://example.com" became "http://example.com/"
func RepairUrls() (int, error) {
	if !feature.MigrateCustomRepositories.IsEnabled() {
		log.Info("custom repositories migration feature is disabled, repair urls is not available")
		return 0, ErrMigrationNotAvailable
	}
	log.Info("repair repositories  urls started ...")
	type RepoURL struct {
		URL   string
		Count int
	}

	var repoURLs []RepoURL
	if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).
		Select("url, count(url) as count").
		Where("url NOT LIKE '%/' AND url IS NOT NULL AND url != ''").
		Group("url").
		Scan(&repoURLs).Error; err != nil {

		log.WithField("error", err.Error()).Error("error when retrieving urls without slashes")
		return 0, err
	}
	for _, repoURL := range repoURLs {
		newURL := models.AddSlashToURL(repoURL.URL)
		log.WithFields(log.Fields{"URL": repoURL.URL, "Count": repoURL.Count, "newURL": newURL}).Info("updating url")
		if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).Where("url = ?", repoURL.URL).Update("url", newURL).Error; err != nil {
			log.WithField("error", err.Error()).Error("error while updating with newURL")
			return 0, err
		}
	}

	affectedURLS := len(repoURLs)
	log.WithField("effected", affectedURLS).Info("... repair repositories urls finished")
	return affectedURLS, nil
}

func repairOrgDuplicateRepoURLs(orgID string, url string) error {
	repairLogger := log.WithFields(log.Fields{"org_id": orgID, "url": url})
	if orgID == "" || url == "" {
		repairLogger.Info("no orgID or url supplied to repair repositories duplicate urls")
		// return early if orgID or url are not defined
		return nil
	}

	repairLogger.Info("repair org repositories duplicate urls started ...")

	repairLogger.Debug("collect all required repositories")

	// collect all the requested reposIDS, order by id descending to choose later the latest repo id
	var reposIDS []uint
	if err := db.Org(orgID, "").Debug().Model(&models.ThirdPartyRepo{}).Where("url = ?", url).
		Order("id DESC").Pluck("id", &reposIDS).Error; err != nil {
		repairLogger.WithField("error", err.Error()).Error("error occurred while retrieving the required repos")
		return err
	}
	if len(reposIDS) <= 1 {
		repairLogger.Info("... repair org repositories duplicate urls finished with no duplicates found")
		// return early
		return nil
	}

	// chose the first one from the IDS to be preserved (this is the last one from the query above)
	chosenRepoID := reposIDS[0]
	// define the other repos to be replaced and deleted
	// e.g. the otherRepos with same urls will be replaced by the chosen one
	otherReposIDS := reposIDS[1:]
	repairLogger = repairLogger.WithFields(log.Fields{"chosenRepo": chosenRepoID, "otherRepos": otherReposIDS})
	repairLogger.Info("repositories collected and ready for repair duplicate urls")

	// replace all otherRepos in images repos with the chosenRepo
	if err := db.DB.Debug().Table("images_repos").
		Where("third_party_repo_id IN (?)", otherReposIDS).
		Update("third_party_repo_id", chosenRepoID).Error; err != nil {
		repairLogger.WithField("error", err.Error()).Error("error occurred while replacing other repos by the chosen one")
		return err
	}

	// delete forever all otherRepos
	if err := db.Org(orgID, "").Unscoped().Debug().Delete(&models.ThirdPartyRepo{}, otherReposIDS).Error; err != nil {
		repairLogger.WithField("error", err.Error()).Error("error occurred while deleting other repos")
		return err
	}

	repairLogger.Info("... repair org repositories duplicate urls finished")

	return nil
}

func RepairDuplicates() error {
	if !feature.MigrateCustomRepositories.IsEnabled() {
		log.Info("custom repositories migration feature is disabled, repair duplicates is not available")
		return ErrMigrationNotAvailable
	}

	log.Info("repair repositories duplicate urls started ...")

	type RepoData struct {
		OrgID string
		URL   string
		Count int
	}

	var reposData []RepoData
	if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).
		Select("org_id, url, count(url) as count").
		Where("url IS NOT NULL AND url != ''").
		Group("org_id, url").
		Having("count(url) > 1").
		Scan(&reposData).Error; err != nil {
		log.WithField("error", err.Error()).Error("error when collecting orgs duplicate urls")
	}

	if len(reposData) == 0 {
		log.Info("... repair repositories duplicate urls finished with no organizations repository urls duplicates found")
		return nil
	}

	for _, repoData := range reposData {
		log.WithFields(log.Fields{"URL": repoData.URL, "Count": repoData.Count, "org": repoData.OrgID}).Info("repair duplicates")
		if err := repairOrgDuplicateRepoURLs(repoData.OrgID, repoData.URL); err != nil {
			log.WithField("error", err.Error()).Error("error when repairing org duplicate urls")
			return err
		}
	}

	log.Info("... repair repositories duplicate urls finished")

	return nil
}
