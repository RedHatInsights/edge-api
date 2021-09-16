package models

import "gorm.io/gorm"

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
	gorm.Model
	Name                 string             `json:"name"`
	Account              string             `json:"account"`
	ImageBuildHash       string             `json:"image_build_hash"`
	ImageBuildParentHash string             `json:"image_build_parent_hash"`
	ImageBuildTarURL     string             `json:"image_build_tar_url"`
	OSTreeCommit         string             `json:"os_tree_commit"`
	OSTreeParentCommit   string             `json:"os_tree_parent_commit"`
	OSTreeRef            string             `json:"os_tree_ref"`
	BuildDate            string             `json:"build_date"`
	BuildNumber          uint               `json:"build_number"`
	BlueprintToml        string             `json:"blueprint_toml"`
	Arch                 string             `json:"arch"`
	Packages             []Package          `json:"packages" gorm:"many2many:commit_packages;"`
	InstalledPackages    []InstalledPackage `json:"installed_packages,omitempty" gorm:"many2many:commit_installed_packages;"`
	ComposeJobID         string             `json:"compose_job_id"`
	Status               string             `json:"status"`
}

// Repo is the delivery mechanism of a Commit over HTTP
type Repo struct {
	gorm.Model
	URL      string  `json:"repo_url"`
	Status   string  `json:"repo_status"`
	CommitID *uint   `json:"commit_id"`
	Commit   *Commit `json:"commit"`
}

// Package represents the packages a Commit can have
type Package struct {
	gorm.Model
	Name string `json:"name"`
}

// InstalledPackage represents installed packages a image has
type InstalledPackage struct {
	gorm.Model
	Name      string `json:"name"`
	Arch      string `json:"arch"`
	Release   string `json:"release"`
	Sigmd5    string `json:"sigmd5"`
	Signature string `json:"signature"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Epoch     string `json:"epoch,omitempty"`
}

var requiredPackages = [6]string{
	"ansible",
	"rhc",
	"rhc-worker-playbook",
	"subscription-manager",
	"subscription-manager-plugin-ostree",
	"insights-client",
}

// GetPackagesList returns the packages in a user-friendly list containing their names
func (c *Commit) GetPackagesList() *[]string {
	l := len(requiredPackages)
	pkgs := make([]string, len(c.Packages)+l)
	for i, p := range requiredPackages {
		pkgs[i] = p
	}
	for i, p := range c.Packages {
		pkgs[i+l] = p.Name
	}
	return &pkgs
}
