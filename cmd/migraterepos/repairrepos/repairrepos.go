package repairrepos

import (
	"errors"

	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	log "github.com/sirupsen/logrus"
)

// ErrMigrationNotAvailable error returned when the migration feature flag is disabled
var ErrMigrationNotAvailable = errors.New("migration is not available")

// ErrPostMigrationNotAvailable error returned when the post delete migration feature flag is disabled
var ErrPostMigrationNotAvailable = errors.New("post migration  delete repository is not available")

// DefaultDataLimit the default data limit to use when collecting data
var DefaultDataLimit = 100

// DefaultMaxDataPageNumber the default data pages to handle as preventive way to enter an indefinite loop
var DefaultMaxDataPageNumber = 100

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

	affectedURLS := 0
	page := 0
	for page < DefaultMaxDataPageNumber {
		var repoURLs []RepoURL
		if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).
			Select("url, count(url) as count").
			Where("url NOT LIKE '%/' AND url IS NOT NULL AND url != ''").
			Group("url").
			Limit(DefaultDataLimit).
			Scan(&repoURLs).Error; err != nil {

			log.WithField("error", err.Error()).Error("error when retrieving urls without slashes")
			return 0, err
		}
		if len(repoURLs) == 0 {
			break
		}
		for _, repoURL := range repoURLs {
			newURL := models.AddSlashToURL(repoURL.URL)
			log.WithFields(log.Fields{"URL": repoURL.URL, "Count": repoURL.Count, "newURL": newURL}).Info("updating url")
			if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).Where("url = ?", repoURL.URL).Update("url", newURL).Error; err != nil {
				log.WithField("error", err.Error()).Error("error while updating with newURL")
				return 0, err
			}
		}

		affectedURLS += len(repoURLs)
		page++
	}
	log.WithField("effected", affectedURLS).Info("... repair repositories urls finished")
	return affectedURLS, nil
}

func repairDuplicateImagesReposURLS(imageID uint, url string) error {

	log.WithFields(log.Fields{"URL": url, "image_id": imageID}).Info("repair images_repos url duplicates starting ...")

	// collect all image url repos ids from images_repos
	var reposIDS []uint
	if err := db.DB.Debug().Table("images_repos").
		Select("third_party_repo_id").
		Joins("JOIN third_party_repos on third_party_repos.id = images_repos.third_party_repo_id").
		Where("image_id = ? AND url = ? AND deleted_at IS NULL", imageID, url).
		Order("third_party_repo_id DESC").
		Pluck("third_party_repo_id", &reposIDS).Error; err != nil {
		log.WithFields(log.Fields{"URL": url, "image_id": imageID, "error": err.Error()}).Info("error occurred  while collecting image urls for images_repos url duplicates repair")
		return err
	}

	if len(reposIDS) <= 1 {
		log.WithFields(log.Fields{"URL": url, "image_id": imageID}).Info("... repair images_repos url no duplicates found, finished")
		return nil
	}

	// let only one to exists (the first one from the slice) and delete the others
	otherReposIDS := reposIDS[1:]
	if err := db.DB.Debug().Exec("DELETE FROM images_repos WHERE image_id = ? AND third_party_repo_id IN (?)", imageID, otherReposIDS).Error; err != nil {
		log.WithFields(log.Fields{"URL": url, "image_id": imageID, "to-remove": len(otherReposIDS), "error": err.Error()}).Info("error occurred  while deleting image repos with duplicates urls in image_repos")
		return err
	}

	log.WithFields(log.Fields{"URL": url, "image_id": imageID, "removed": len(otherReposIDS)}).Info("... repair images_repos url duplicates starting finished")

	return nil
}

// RepairDuplicateImagesReposURLS repair images_repos table from images duplicates urls.
// e.g. some images may have may repos with same urls, and this function remove that duplicates.
func RepairDuplicateImagesReposURLS() error {
	if !feature.MigrateCustomRepositories.IsEnabled() {
		log.Info("custom repositories migration feature is disabled, repair images_repos urls duplicates is not available")
		return ErrMigrationNotAvailable
	}

	log.Info("... repair images_repos urls duplicates repair started")

	// collect all images repos with duplicate repos url
	type ImagesReposData struct {
		ImageID uint
		URL     string
		Count   int
	}

	imagesUrls := 0
	page := 0
	for page < DefaultMaxDataPageNumber {
		var imagesReposData []ImagesReposData
		if err := db.DB.Debug().Table("images_repos").
			Select("image_id, url, count(url) as count").
			Joins("JOIN third_party_repos on third_party_repos.id = images_repos.third_party_repo_id").
			Where("deleted_at IS NULL").
			Group("image_id, url, deleted_at").
			Having("count(url) > 1").
			Order("image_id").
			Limit(DefaultDataLimit).
			Scan(&imagesReposData).Error; err != nil {
			log.WithField("error", err.Error()).Info("error occurred while collecting main data for images_repos urls duplicates")
			return err
		}
		if len(imagesReposData) == 0 {
			break
		}
		for _, imageRepoURL := range imagesReposData {
			log.WithFields(log.Fields{"URL": imageRepoURL.URL, "Count": imageRepoURL.Count, "image_id": imageRepoURL.ImageID}).Info("repair images_repos url duplicates")
			if err := repairDuplicateImagesReposURLS(imageRepoURL.ImageID, imageRepoURL.URL); err != nil {
				return err
			}
		}

		imagesUrls += len(imagesReposData)
		page++
	}

	log.WithField("images_repos_urls", imagesUrls).Info("... repair images_repos urls duplicates repair finished")

	return nil
}

func repairOrgDuplicateRepoURL(orgID string, url string) error {
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

// RepairDuplicates repair table third_party_repos from repos with duplicate urls
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

	orgUrls := 0
	page := 0
	for page < DefaultMaxDataPageNumber {
		var reposData []RepoData
		if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).
			Select("org_id, url, count(url) as count").
			Where("url IS NOT NULL AND url != ''").
			Group("org_id, url").
			Having("count(url) > 1").
			Limit(DefaultDataLimit).
			Scan(&reposData).Error; err != nil {
			log.WithField("error", err.Error()).Error("error when collecting orgs duplicate urls")
		}

		if len(reposData) == 0 {
			break
		}

		for _, repoData := range reposData {
			log.WithFields(log.Fields{"URL": repoData.URL, "Count": repoData.Count, "org": repoData.OrgID}).Info("repair duplicates")
			if err := repairOrgDuplicateRepoURL(repoData.OrgID, repoData.URL); err != nil {
				log.WithField("error", err.Error()).Error("error when repairing org duplicate urls")
				return err
			}
		}
		orgUrls += len(reposData)
		page++
	}

	if orgUrls == 0 {
		log.Info("... repair repositories duplicate urls finished with no organizations repository urls duplicates found")
	} else {
		log.WithField("orgUrls", orgUrls).Info("... repair repositories duplicate urls finished all organizations repository urls duplicates has been repaired")
	}

	return nil
}
