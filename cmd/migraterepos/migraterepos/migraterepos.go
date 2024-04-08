package migraterepos

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/redhatinsights/edge-api/cmd/migraterepos/repairrepos"
	"github.com/redhatinsights/edge-api/pkg/clients/repositories"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/redhatinsights/platform-go-middlewares/v2/identity"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

func newOrgContentSourcesClient(orgID string) (repositories.ClientInterface, error) {
	// create a new content-sources client and set organization identity in the initialization context
	ident := identity.XRHID{Identity: identity.Identity{
		OrgID:    orgID,
		Type:     "User",
		Internal: identity.Internal{OrgID: orgID},
		User:     &identity.User{OrgAdmin: true, Username: "edge-repo-migrator"},
	}}
	jsonIdent, err := json.Marshal(&ident)
	if err != nil {
		return nil, err
	}
	base64Identity := base64.StdEncoding.EncodeToString(jsonIdent)
	ctx := context.Background()
	ctx = common.SetOriginalIdentity(ctx, base64Identity)

	client := repositories.InitClient(ctx, log.NewEntry(log.StandardLogger()))
	return client, nil
}

func createContentSourcesRepository(client repositories.ClientInterface, emRepo models.ThirdPartyRepo) (*repositories.Repository, error) {
	logger := log.WithFields(log.Fields{"content": "content-sources-repo-migration-create", "org_id": emRepo.OrgID, "url": emRepo.URL})
	repoName := emRepo.Name

	logger.Info("getting content-source repository by name")
	if _, err := client.GetRepositoryByName(repoName); err != nil {
		if err != repositories.ErrRepositoryNotFound {
			logger.WithField("error", err.Error()).Error("error occurred while getting content-sources repository by name")
			return nil, err
		}
		// content-sources repository with repoName does not exist, will keep using the repoName
	} else {
		// content-sources repository with repoName does exist, generate a new repoName using an uuid
		repoName = fmt.Sprintf("%s_migrated_%s", repoName, uuid.NewString())
	}

	logger.WithField("name", repoName).Info("creating content-source repository")
	return client.CreateRepository(repositories.Repository{
		Name: repoName,
		URL:  emRepo.URL,
	})
}

func migrateCustomRepository(client repositories.ClientInterface, emRepo models.ThirdPartyRepo) error {
	logger := log.WithFields(log.Fields{"context": "content-sources-repo-migration", "org_id": emRepo.OrgID, "name": emRepo.Name, "url": emRepo.URL})
	logger.Info("custom repository migration started")
	// check if content-sources repo already exist with repo url
	var csRepo *repositories.Repository
	repo, err := client.GetRepositoryByURL(emRepo.URL)
	if err != nil {
		if err != repositories.ErrRepositoryNotFound {
			logger.WithField("error", err.Error()).Error("error occurred while getting repository by url")
			return err
		}
	} else {
		csRepo = repo
	}
	if csRepo == nil {
		// The content-sources with url does not exist, create a new content-sources repository
		repo, err := createContentSourcesRepository(client, emRepo)
		if err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while creating content-sources repository")
			return err
		}
		csRepo = repo
	}

	// update local repo with main content-sources repo data
	emRepo.UUID = csRepo.UUID.String()
	emRepo.GpgKey = csRepo.GpgKey
	emRepo.DistributionArch = csRepo.DistributionArch
	emRepo.PackageCount = csRepo.PackageCount
	if err := db.DB.Save(&emRepo).Error; err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while saving custom repository with updated data from content-sources")
		return err
	}
	logger.WithField("name", emRepo.Name).Info("custom repository migration successfully")
	return nil
}

func migrateOrgCustomRepositories(orgID string) error {
	if orgID == "" {
		return nil
	}
	logger := log.WithFields(log.Fields{"context": "content-sources-org-migration", "org_id": orgID})

	logger.Info("organization content-source repositories migration started")

	client, err := newOrgContentSourcesClient(orgID)
	if err != nil {
		logger.WithField("error", err.Error()).Error("error occurred while getting organization content-sources client")
		return err
	}

	reposCount := 0
	page := 0
	for page < repairrepos.DefaultMaxDataPageNumber {
		var repos []models.ThirdPartyRepo
		if err := db.Org(orgID, "").Debug().Where("uuid IS NULL OR UUID = ''").
			Order("id ASC").
			Limit(repairrepos.DefaultDataLimit).
			Find(&repos).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while collecting organization custom repositories to migrate")
			return err
		}

		if len(repos) == 0 {
			break
		}
		for _, repo := range repos {
			if err := migrateCustomRepository(client, repo); err != nil {
				logger.WithField("error", err.Error()).Error("error occurred while migrating organization custom repository")
				return err
			}
		}

		reposCount += len(repos)
		page++
	}

	logger.WithField("repos_count", reposCount).Info("organization content-source repositories migration finished")

	return nil
}

// MigrateAllCustomRepositories migrate all custom repositories to content-sources repositories
func MigrateAllCustomRepositories() error {

	logger := log.WithField("context", "context-sources-migration")
	if !feature.MigrateCustomRepositories.IsEnabled() {
		logger.Info("custom repositories migration feature is disabled, migration is not available")
		return repairrepos.ErrMigrationNotAvailable
	}

	// collect all org ids with custom repositories not yet migrated
	// a custom repository is considered migrated if it's uuid is not null and not empty
	type OrgMigrationRepos struct {
		OrgID string
		Count int
	}

	page := 0
	orgsRepos := 0
	reposCount := 0
	for page < repairrepos.DefaultMaxDataPageNumber {
		var orgMigrationRepos []OrgMigrationRepos
		if err := db.DB.Debug().Model(&models.ThirdPartyRepo{}).
			Select("distinct(org_id), count(*) as count").
			Where("uuid IS NULL OR UUID = ''").
			Group("org_id").
			Order("org_id ASC").
			Limit(repairrepos.DefaultDataLimit).
			Scan(&orgMigrationRepos).Error; err != nil {
			logger.WithField("error", err.Error()).Error("error occurred while collecting organizations for custom repositories migration")
			return err
		}
		if len(orgMigrationRepos) == 0 {
			break
		}
		for _, orgToMigrate := range orgMigrationRepos {
			logger.WithFields(log.Fields{"org_id": orgToMigrate.OrgID, "count": orgToMigrate.Count}).Info("starting migration of organization custom repositories")
			err := migrateOrgCustomRepositories(orgToMigrate.OrgID)
			if err != nil {
				logger.WithFields(log.Fields{"org_id": orgToMigrate.OrgID, "count": orgToMigrate.Count, "error": err.Error()}).Error("error occurred while migrating organization custom repositories")
				return err
			}
			reposCount += orgToMigrate.Count
		}
		orgsRepos += len(orgMigrationRepos)
		page++
	}

	logger.WithFields(log.Fields{"orgs_count": orgsRepos, "repos_count": reposCount}).Info("migration of organizations custom repositories finished")
	return nil
}
