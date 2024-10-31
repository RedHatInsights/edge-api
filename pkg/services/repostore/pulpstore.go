package repostore

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/pulp"

	log "github.com/sirupsen/logrus"
)

// PulpRepoStore imports an OSTree repo into a Pulp repository
func PulpRepoStore(ctx context.Context, orgID string, sourceRepoID uint, sourceURL string) (string, error) {
	// create a pulp service with a pre-defined name
	pserv, err := domainService(ctx, orgID)
	if err != nil {
		return "", err
	}
	log.WithContext(ctx).Info("Service for domain created")

	// Import the commit tarfile into an initial Pulp file repo
	fileRepoArtifact, fileRepoVersion, err := fileRepoImport(ctx, pserv, sourceURL)
	if err != nil {
		log.WithContext(ctx).Error("Error importing tarfile to initial pulp file repository")

		return "", err
	}
	log.WithContext(ctx).WithFields(log.Fields{
		"artifact": fileRepoArtifact,
		"version":  fileRepoArtifact,
	}).Info("Pulp artifact uploaded")

	// create an OSTree repository in Pulp
	repoName := fmt.Sprintf("repo-%s-%d", orgID, sourceRepoID)

	distBaseURL, pulpHref, err := createOSTreeRepository(ctx, pserv, orgID, repoName)
	if err != nil {
		log.WithContext(ctx).Error("Error creating pulp ostree repository")

		return "", err
	}
	log.WithContext(ctx).WithFields(log.Fields{
		"dist_base_url": distBaseURL,
		"pulp_href":     pulpHref,
	}).Info("Pulp OSTree Repo created with Content Guard and Distribution")

	repoURL, err := ostreeRepoImport(ctx, pserv, pulpHref, repoName, distBaseURL, fileRepoArtifact)
	if err != nil {
		log.WithContext(ctx).Error("Error importing tarfile into pulp ostree repository")

		return "", err
	}
	parsedURL, _ := url.Parse(repoURL)
	log.WithContext(ctx).WithField("repo_url", parsedURL.Redacted()).Info("Image Builder commit tarfile imported into pulp ostree repo from pulp file repo")

	if err = pserv.FileRepositoriesVersionDelete(ctx, pulp.ScanUUID(&fileRepoVersion), pulp.ScanRepoFileVersion(&fileRepoVersion)); err != nil {
		return "", err
	}
	log.WithContext(ctx).WithField("filerepo_version", fileRepoVersion).Info("Artifact version deleted")

	return repoURL, nil
}

// looks up a domain, creates one if it does not exist, and returns a service using that domain
func domainService(ctx context.Context, orgID string) (*pulp.PulpService, error) {
	name := fmt.Sprintf("em%sd", orgID)

	pulpDefaultService, err := pulp.NewPulpServiceDefaultDomain(ctx)
	if err != nil {
		return nil, err
	}

	domains, err := pulpDefaultService.DomainsList(ctx, name)
	if err != nil {
		log.WithContext(ctx).WithFields(log.Fields{
			"domain_name": name,
			"error":       err.Error(),
		}).Error("Error listing pulp domains")
		return nil, err
	}

	// there can be only one
	if len(domains) > 1 {
		log.WithContext(ctx).WithFields(log.Fields{
			"name":    name,
			"domains": domains,
		}).Error("More than one domain matches name")

		return nil, errors.New("More than one domain matches name")
	}

	// if a domain doesn't already exist, create a new one and use it for the service
	var domainUUID uuid.UUID
	if len(domains) == 0 {
		pulpDomain, err := pulpDefaultService.DomainsCreate(ctx, name)
		if err != nil {
			log.WithContext(ctx).WithField("error", err.Error()).Error("Error creating pulp domain")

			return nil, err
		}

		domainUUID = pulp.ScanUUID(pulpDomain.PulpHref)

		log.WithContext(ctx).WithFields(log.Fields{
			"domain_name": pulpDomain.Name,
			"domain_href": pulpDomain.PulpHref,
			"domain_uuid": domainUUID,
		}).Info("Created a new pulp domain")
	}

	// get a pulp service based on the specific domain
	pserv, err := pulp.NewPulpServiceWithDomain(ctx, name)
	if err != nil {
		return nil, err
	}

	log.WithContext(ctx).WithField("domain_name", name).Info("Service for domain created")

	return pserv, nil
}

// creates a new Pulp OSTree repository for a commit
func createOSTreeRepository(ctx context.Context, pulpService *pulp.PulpService, orgID string, name string) (string, string, error) {
	pulpRepo, err := pulpService.RepositoriesCreate(ctx, name)
	if err != nil {
		return "", "", err
	}
	pulpHref := *pulpRepo.PulpHref
	log.WithContext(ctx).WithField("pulp_href", pulpHref).Info("Pulp Repository created")

	cg, err := pulpService.ContentGuardEnsure(ctx, orgID)
	if err != nil {
		return "", "", err
	}
	cgPulpHref := *cg.PulpHref
	log.WithContext(ctx).WithFields(log.Fields{
		"contentguard_href": cgPulpHref,
		"content_guards":    *cg.Guards,
	}).Info("Pulp Content Guard found or created")

	distribution, err := pulpService.DistributionsCreate(ctx, name, name, *pulpRepo.PulpHref, cgPulpHref)
	if err != nil {
		return "", "", err
	}
	log.WithContext(ctx).WithFields(log.Fields{
		"name":      distribution.Name,
		"base_path": distribution.BasePath,
		"base_url":  *distribution.BaseUrl,
		"pulp_href": distribution.PulpHref,
	}).Info("Pulp Distribution created")

	return *distribution.BaseUrl, *pulpRepo.PulpHref, nil
}

// returns the complete distribution URL
func distributionURL(ctx context.Context, domain string, repoName string) (string, error) {
	cfg := config.Get()
	var distURL string
	distURL = fmt.Sprintf("%s/api/pulp-content/%s/%s", cfg.PulpContentURL, domain, repoName)

	if cfg.Pulp.ContentUsername != "" {
		rbacDistURL, err := url.Parse(distURL)
		if err != nil {
			return "", errors.New("Unable to set user:password for Pulp distribution URL")
		}
		rbacDistURL.User = url.UserPassword(cfg.PulpContentUsername, cfg.PulpContentPassword)
		distURL = rbacDistURL.String()
	}

	parsedURL, _ := url.Parse(distURL)
	log.WithContext(ctx).WithField("distribution_url", parsedURL.Redacted()).Debug("Distribution URL retrieved")

	return distURL, nil
}

// imports an image builder tarfile into a Pulp file repo
func fileRepoImport(ctx context.Context, pulpService *pulp.PulpService, sourceURL string) (string, string, error) {
	var err error
	fileRepo, err := pulpService.FileRepositoriesEnsure(ctx)
	if err != nil {
		return "", "", err
	}

	log.WithContext(ctx).WithField("file_repo", fileRepo).Info("File repo found or created")

	artifact, version, err := pulpService.FileRepositoriesImport(ctx, fileRepo, sourceURL)
	if err != nil {
		return "", "", err
	}

	log.WithContext(ctx).WithFields(log.Fields{"artifact_href": artifact, "version_href": version}).Debug("Artifact and version href")

	return artifact, version, nil
}

// Import imports an artifact into a Pulp repo and deletes the tarfile artifact
func ostreeRepoImport(ctx context.Context, pulpService *pulp.PulpService, pulpHref string, pulpRepoName string,
	distBaseURL string, fileRepoArtifact string) (string, error) {

	log.WithContext(ctx).WithFields(log.Fields{
		"pulp_href":            pulp.ScanUUID(&pulpHref),
		"pulp_reponame":        pulpRepoName,
		"distribution_baseurl": distBaseURL,
		"artifact_href":        fileRepoArtifact,
	}).Debug("Starting tarfile import into Pulp OSTree repository")
	repoImported, err := pulpService.RepositoriesImport(ctx, pulp.ScanUUID(&pulpHref), "repo", fileRepoArtifact)
	if err != nil {
		return "", err
	}
	log.WithContext(ctx).WithField("repo_href", *repoImported.PulpHref).Info("Repository imported")

	repoURL, err := distributionURL(ctx, pulpService.Domain(), pulpRepoName)
	if err != nil {
		log.WithContext(ctx).WithField("error", err.Error()).Error("Error getting distibution URL for Pulp repo")
		return "", err
	}

	parsedURL, _ := url.Parse(repoURL)
	log.WithContext(ctx).WithFields(log.Fields{
		"repo_distribution_url": parsedURL.Redacted(),
	}).Debug("Repo import into Pulp complete")

	return repoURL, nil
}
