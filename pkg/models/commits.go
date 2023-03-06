// FIXME: golangci-lint
// nolint:govet,revive
package models

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	// ErrOrgIDIsMandatory is for when orgID is not set
	ErrOrgIDIsMandatory = errors.New("org_id is mandatory")
	// ErrDeviceExists is for UUID already registered on our DB
	ErrDeviceExists = errors.New("This UUID already exists")
)

const (
	// RepoStatusBuilding is for when a image is on a error state
	RepoStatusBuilding = "BUILDING"
	// RepoStatusError is for when a Repo is on a error state
	RepoStatusError = "ERROR"
	// RepoStatusSuccess is for when a Repo is available to the user
	RepoStatusSuccess = "SUCCESS"
)

// Commit represents an OSTree commit from image builder
type Commit struct {
	Model
	Name                 string
	Account              string             `json:"Account"`
	OrgID                string             `json:"org_id" gorm:"index;<-:create"`
	ImageBuildHash       string             `json:"ImageBuildHash"`
	ImageBuildParentHash string             `json:"ImageBuildParentHash"`
	ImageBuildTarURL     string             `json:"ImageBuildTarURL"`
	OSTreeCommit         string             `json:"OSTreeCommit"`
	OSTreeParentCommit   string             `json:"OSTreeParentCommit"`
	OSTreeRef            string             `json:"OSTreeRef"`
	OSTreeParentRef      string             `json:"OSTreeParentRef"`
	BuildDate            string             `json:"BuildDate"`
	BuildNumber          uint               `json:"BuildNumber"`
	BlueprintToml        string             `json:"BlueprintToml"`
	Arch                 string             `json:"Arch"`
	InstalledPackages    []InstalledPackage `json:"InstalledPackages,omitempty" gorm:"many2many:commit_installed_packages;"`
	ComposeJobID         string             `json:"ComposeJobID"`
	Status               string             `json:"Status"`
	RepoID               *uint              `json:"RepoID"`
	Repo                 *Repo              `json:"Repo"`
	ChangesRefs          bool               `gorm:"default:false" json:"ChangesRefs"`
	ExternalURL          bool               `json:"external"`
}

// Repo is the delivery mechanism of a Commit over HTTP
type Repo struct {
	Model
	URL    string `json:"RepoURL"`
	Status string `json:"RepoStatus"`
}

// Package represents the packages a Commit can have
type Package struct {
	Model
	Name string `json:"Name"`
}

// InstalledPackage represents installed packages a image has
type InstalledPackage struct {
	Model
	Name      string   `json:"name"`
	Arch      string   `json:"arch"`
	Release   string   `json:"release"`
	Sigmd5    string   `json:"sigmd5"`
	Signature string   `json:"signature"`
	Type      string   `json:"type"`
	Version   string   `json:"version"`
	Epoch     string   `json:"epoch,omitempty"`
	Commits   []Commit `gorm:"many2many:commit_installed_packages;save_association:false"`
}

type CommitInstalledPackages struct {
	InstalledPackageId uint
	CommitId           uint
}

// SearchPackageResult contains Meta of a MetaCount
type SearchPackageResult struct {
	Meta MetaCount       `json:"meta"`
	Data []SearchPackage `json:"data"`
}

// MetaCount contains Count of a SearchPackageResult
type MetaCount struct {
	Count int `json:"count"`
}

// SearchPackage contains Name of package
type SearchPackage struct {
	Name string `json:"name"`
}

// BeforeCreate method is called before creating Commits, it make sure org_id is not empty
func (c *Commit) BeforeCreate(tx *gorm.DB) error {
	if c.OrgID == "" {
		log.Error("commit do not have an org_id")
		return ErrOrgIDIsMandatory
	}

	return nil
}
