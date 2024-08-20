// FIXME: golangci-lint
// nolint:revive
package models

import (
	"errors"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

/*
ThirdPartyRepo is a record of Third Party Repository or we can call it as Custom Repository provided by customers per OrgID.

	Here, URL refers to the url of the third party repository, OrgID refers to the OrgID attached to the third party
	repository.
*/
type ThirdPartyRepo struct {
	Model
	Name                string    `json:"Name" gorm:"index"`
	URL                 string    `json:"URL"`
	Description         string    `json:"Description,omitempty"`
	UUID                string    `json:"uuid,omitempty" gorm:"index"`
	DistributionArch    string    `json:"distribution_arch,omitempty"`
	DistributionVersion *[]string `json:"distribution_version,omitempty" gorm:"-"`
	GpgKey              string    `json:"gpg_key,omitempty"`
	PackageCount        int       `json:"package_count,omitempty"`
	Account             string    `json:"account"`
	OrgID               string    `json:"org_id" gorm:"index;<-:create"`
	Images              []Image   `faker:"-" json:"Images,omitempty" gorm:"many2many:images_repos;"`
}

const (
	// RepoNameCantBeInvalidMessage is the error message when the name is invalid
	RepoNameCantBeInvalidMessage = "name must start with alphanumeric characters and can contain underscore and hyphen characters"
	// RepoURLCantBeNilMessage is the error message when Repository url is nil
	RepoURLCantBeNilMessage = "repository URL can't be empty"
	// RepoNameCantBeNilMessage is the error when Repository name is nil
	RepoNameCantBeNilMessage = "repository name can't be empty"
	// InvalidURL is the error when type invalid URL
	InvalidURL = "invalid URL"
)

var (
	validRepoName = regexp.MustCompile(`^[A-Za-z0-9]+[A-Za-z0-9\s_-]*$`)
	validURL      = regexp.MustCompile(`^(?:http(s)?:\/\/)[\w.-]+(?:\.[\w\.-]+)+[\w\-\._~:/?#[\]@!\$&'\(\)\*\+,;=.]+$`)
)

// ValidateRepoURL validates the repo URL Request
func ValidateRepoURL(url string) bool {
	return validURL.MatchString(url)
}

// ValidateRequest validates the Repository Request
func (t *ThirdPartyRepo) ValidateRequest() error {
	if t.Name == "" {
		return errors.New(RepoNameCantBeNilMessage)
	}
	if t.URL == "" {
		return errors.New(RepoURLCantBeNilMessage)
	}
	if !validRepoName.MatchString(t.Name) {
		return errors.New(RepoNameCantBeInvalidMessage)
	}
	if !ValidateRepoURL(t.URL) {
		return errors.New(InvalidURL)
	}
	return nil
}

// AddSlashToURL cleanup url from leading and trailing white spaces and add slash "/" at the end if missing
// e.g. transform " http://repo.url.com/repo "  to "http://repo.url.com/repo/"
func AddSlashToURL(url string) string {
	url = strings.TrimSpace(url)
	if len(url) > 0 && url[len(url)-1] != '/' {
		url += "/"
	}
	return url
}

// RepoURLCleanUp define the cleanup function, by default equal to AddSlashToURL, now it's used only to allow unit-testing
// of the migration preparation scripts
var RepoURLCleanUp = AddSlashToURL

// BeforeCreate method is called before creating Third Party Repository, it make sure org_id is not empty
func (t *ThirdPartyRepo) BeforeCreate(tx *gorm.DB) error {
	if t.OrgID == "" {
		log.Error("custom-repository do not have an org_id")
		return ErrOrgIDIsMandatory
	}
	// clean up URL and add slash "/"
	t.URL = RepoURLCleanUp(t.URL)

	return nil
}

// BeforeUpdate is called before updating third party repository
func (t *ThirdPartyRepo) BeforeUpdate(tx *gorm.DB) error {
	// clean up URL and add slash "/"
	t.URL = RepoURLCleanUp(t.URL)
	return nil
}
