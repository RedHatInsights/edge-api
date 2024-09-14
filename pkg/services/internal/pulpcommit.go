package repostore

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/pulp"
	"github.com/redhatinsights/edge-api/pkg/models"

	log "github.com/sirupsen/logrus"
)

// PulpCommit relates to the storage of a commit tarfile in Pulp
type PulpCommit struct {
	OrgID      string
	SourceURL  string
	Domain     *pulpDomain
	FileRepo   *pulpFileRepository
	OSTreeRepo *pulpOSTreeRepository
}

// Store imports an OSTree repo into Pulp
func (pc *PulpCommit) Store(ctx context.Context, commit models.Commit, edgeRepo *models.Repo) error {

	// create a domain with a pre-defined name
	pc.Domain = &pulpDomain{
		Name: fmt.Sprintf("em%sd", commit.OrgID),
	}
	derr := pc.Domain.Create(ctx)
	if derr != nil {
		return derr
	}

	// get a pulp service based on the specific domain
	pserv, err := pulp.NewPulpServiceWithDomain(ctx, pc.Domain.Name)
	if err != nil {
		return err
	}

	// Import the commit tarfile into an initial Pulp file repo
	pc.FileRepo = &pulpFileRepository{}
	if err = pc.FileRepo.Import(ctx, pserv, pc.SourceURL); err != nil {
		log.WithContext(ctx).Error("Error importing tarfile to initial pulp file repository")
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{
		"artifact": pc.FileRepo.artifact,
		"version":  pc.FileRepo.version,
	}).Info("Pulp artifact uploaded")

	// create an OSTree repository in Pulp
	// TODO: check for an existing OSTree repo for image commit versions > 1 and add the commit
	pc.OSTreeRepo = &pulpOSTreeRepository{
		Name: fmt.Sprintf("repo-%s-%d", commit.OrgID, edgeRepo.ID),
	}
	if err = pc.OSTreeRepo.Create(ctx, pserv, pc.OrgID); err != nil {
		log.WithContext(ctx).Error("Error creating pulp ostree repository")
		return err
	}
	log.WithContext(ctx).Info("Pulp OSTree Repo created with Content Guard and Distribution")

	if err = pc.OSTreeRepo.Import(ctx, pserv, pc.OSTreeRepo.Name, edgeRepo, *pc.FileRepo); err != nil {
		log.WithContext(ctx).Error("Error importing tarfile into pulp ostree repository")

		return err
	}
	log.WithContext(ctx).Info("Image Builder commit tarfile imported into pulp ostree repo from pulp file repo")

	return nil
}

type pulpDomain struct {
	Name string
	UUID uuid.UUID
}

// Create creates a domain for a specific org if one does not already exist
func (pd *pulpDomain) Create(ctx context.Context) error {
	pulpDefaultService, err := pulp.NewPulpServiceDefaultDomain(ctx)
	if err != nil {
		return err
	}

	domains, err := pulpDefaultService.DomainsList(ctx, pd.Name)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{
			"domain_name": pd.Name,
			"error":       err.Error(),
		}).Error("Error listing pulp domains")
		return err
	}

	if len(domains) > 1 {
		log.WithContext(ctx).WithFields(log.Fields{
			"name":    pd.Name,
			"domains": domains,
		}).Error("More than one domain matches name")
		return errors.New("More than one domain matches name")
	}

	if len(domains) == 0 {
		createdDomain, err := pulpDefaultService.DomainsCreate(ctx, pd.Name)
		if err != nil {
			log.WithContext(ctx).WithField("error", err.Error()).Error("Error creating pulp domain")
			return err
		}

		pd.UUID = pulp.ScanUUID(createdDomain.PulpHref)

		log.WithContext(ctx).WithFields(log.Fields{
			"domain_name": createdDomain.Name,
			"domain_href": createdDomain.PulpHref,
			"domain_uuid": pd.UUID,
		}).Info("Created new pulp domain")
	}

	return nil
}

type pulpFileRepository struct {
	repo     string
	artifact string
	version  string
}

type pulpFileRepoImporter interface {
	FileRepositoriesEnsure(context.Context) (string, error)
	FileRepositoriesImport(context.Context, string, string) (string, string, error)
}

func (pfr *pulpFileRepository) Import(ctx context.Context, pulpService pulpFileRepoImporter, sourceURL string) error {

	// get the file repo to initially push the tar artifact
	var err error
	pfr.repo, err = pulpService.FileRepositoriesEnsure(ctx)
	if err != nil {
		return err
	}

	log.WithContext(ctx).Info("File repo found or created: ", pfr.repo)

	pfr.artifact, pfr.version, err = pulpService.FileRepositoriesImport(ctx, pfr.repo, sourceURL)
	if err != nil {
		return err
	}

	return nil
}

type pulpOSTreeRepository struct {
	ID           uint
	Name         string
	CGPulpHref   string
	PulpHref     string
	Distribution *pulp.OstreeOstreeDistributionResponse
}

type pulpOSTreeRepositoryCreator interface {
	RepositoriesCreate(context.Context, string) (*pulp.OstreeOstreeRepositoryResponse, error)
	ContentGuardEnsure(context.Context, string) (*pulp.CompositeContentGuardResponse, error)
	DistributionsCreate(context.Context, string, string, string, string) (*pulp.OstreeOstreeDistributionResponse, error)
}

// Create creates a new Pulp OSTree repository for a commit
func (pr *pulpOSTreeRepository) Create(ctx context.Context, pulpService pulpOSTreeRepositoryCreator, orgID string) error {

	pulpRepo, err := pulpService.RepositoriesCreate(ctx, pr.Name)
	if err != nil {
		return err
	}
	pr.PulpHref = *pulpRepo.PulpHref
	log.WithContext(ctx).WithField("pulp_href", pr.PulpHref).Info("Pulp Repository created")

	cg, err := pulpService.ContentGuardEnsure(ctx, orgID)
	if err != nil {
		return err
	}
	pr.CGPulpHref = *cg.PulpHref
	log.WithContext(ctx).WithFields(log.Fields{
		"contentguard_href": pr.CGPulpHref,
		"contentguard_0":    (*cg.Guards)[0],
		"contentguard_1":    (*cg.Guards)[1],
	}).Info("Pulp Content Guard found or created")

	pr.Distribution, err = pulpService.DistributionsCreate(ctx, pr.Name, pr.Name, *pulpRepo.PulpHref, *cg.PulpHref)
	if err != nil {
		return err
	}
	log.WithContext(ctx).WithFields(log.Fields{
		"name":      pr.Distribution.Name,
		"base_path": pr.Distribution.BasePath,
		"base_url":  pr.Distribution.BaseUrl,
		"pulp_href": pr.Distribution.PulpHref,
	}).Info("Pulp Distribution created")

	return nil
}

// DistributionURL returns the password embedded distribution URL for a specific repo
func (pr *pulpOSTreeRepository) DistributionURL(ctx context.Context, domain string, repoName string) (string, error) {
	cfg := config.Get()

	distBaseURL := *pr.Distribution.BaseUrl
	prodDistURL, err := url.Parse(distBaseURL)
	if err != nil {
		return "", errors.New("Unable to set user:password for Pulp distribution URL")
	}
	prodDistURL.User = url.UserPassword(cfg.PulpContentUsername, cfg.PulpContentPassword)
	distURL := prodDistURL.String()

	// temporarily handle stage URLs so Image Builder worker can get to stage Pulp
	if strings.Contains(distBaseURL, "stage") {
		stagePulpURL := fmt.Sprintf("%s/api/pulp-content/%s/%s", cfg.PulpURL, domain, repoName)
		stageDistURL, err := url.Parse(stagePulpURL)
		if err != nil {
			return "", errors.New("Unable to set user:password for Pulp distribution URL")
		}
		stageDistURL.User = url.UserPassword(cfg.PulpContentUsername, cfg.PulpContentPassword)

		distURL = stageDistURL.String()
	}

	parsedDistURL, _ := url.Parse(distURL)
	log.WithContext(ctx).WithField("distribution_url", parsedDistURL.Redacted()).Debug("Distribution URL retrieved")

	return distURL, err
}

type pulpOSTreeRepositoryImporter interface {
	RepositoriesImport(context.Context, uuid.UUID, string, string) (*pulp.OstreeOstreeRepositoryResponse, error)
	FileRepositoriesVersionDelete(context.Context, uuid.UUID, int64) error
	Domain() string
}

// Import imports an artifact into the repo and deletes the tarfile artifact
func (pr *pulpOSTreeRepository) Import(ctx context.Context, pulpService pulpOSTreeRepositoryImporter, pulpRepoName string, edgeRepo *models.Repo, fileRepo pulpFileRepository) error {
	log.WithContext(ctx).Debug("Starting tarfile import into Pulp OSTree repository")
	repoImported, err := pulpService.RepositoriesImport(ctx, pulp.ScanUUID(&pr.PulpHref), "repo", fileRepo.artifact)
	if err != nil {
		return err
	}
	log.WithContext(ctx).Info("Repository imported", *repoImported.PulpHref)

	edgeRepo.URL, err = pr.DistributionURL(ctx, pulpService.Domain(), pulpRepoName)
	if err != nil {
		log.WithContext(ctx).WithField("error", err.Error()).Error("Error getting distibution URL for Pulp repo")
	}

	// TODO: move this to parallel or end of entire process (post Installer if requested)
	err = pulpService.FileRepositoriesVersionDelete(ctx, pulp.ScanUUID(&fileRepo.version), pulp.ScanRepoFileVersion(&fileRepo.version))
	if err != nil {
		return err
	}
	log.WithContext(ctx).Info("Artifact version deleted", fileRepo.version)

	edgeRepo.Status = models.RepoStatusSuccess

	edgeRepoURL, _ := url.Parse(edgeRepo.URL)
	log.WithContext(ctx).WithFields(log.Fields{
		"status":                edgeRepo.Status,
		"repo_distribution_url": edgeRepoURL.Redacted(),
	}).Debug("Repo import into Pulp complete")

	return nil
}
