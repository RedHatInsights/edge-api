package models

import (
	"errors"
	"fmt"
	"regexp"

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
	Name        string `json:"Name"`
	URL         string `json:"URL"`
	Description string `json:"Description,omitempty"`
	Account     string
	OrgID       string  `json:"org_id" gorm:"index"`
	Images      []Image `faker:"-" json:"Images,omitempty" gorm:"many2many:images_repos;"`
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
		return fmt.Errorf(InvalidURL)
	}
	return nil
}

// BeforeCreate method is called before creating Third Party Repository, it make sure org_id is not empty
func (t *ThirdPartyRepo) BeforeCreate(tx *gorm.DB) error {
	if t.OrgID == "" {
		log.Error("custom-repository do not have an org_id")
		return ErrOrgIDIsMandatory
	}

	return nil
}
