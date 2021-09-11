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
	Name                 string
	Account              string             `json:"Account"`
	ImageBuildHash       string             `json:"ImageBuildHash"`
	ImageBuildParentHash string             `json:"ImageBuildParentHash"`
	ImageBuildTarURL     string             `json:"ImageBuildTarURL"`
	OSTreeCommit         string             `json:"OSTreeCommit"`
	OSTreeParentCommit   string             `json:"OSTreeParentCommit"`
	OSTreeRef            string             `json:"OSTreeRef"`
	BuildDate            string             `json:"BuildDate"`
	BuildNumber          uint               `json:"BuildNumber"`
	BlueprintToml        string             `json:"BlueprintToml"`
	Arch                 string             `json:"Arch"`
	Packages             []Package          `json:"Packages" gorm:"many2many:commit_packages;"`
	InstalledPackages    []InstalledPackage `json:"InstalledPackages,omitempty" gorm:"many2many:commit_installed_packages;"`
	ComposeJobID         string             `json:"ComposeJobID"`
	Status               string             `json:"Status"`
	Repos                []Repo
}

// Repo is the delivery mechanism of a Commit over HTTP
type Repo struct {
	gorm.Model
	URL      string `json:"RepoURL"`
	Status   string `json:"RepoStatus"`
	CommitID uint   `json:"CommitID"`
	Commit   Commit `json:"Commit"`
}

// Package represents the packages a Commit can have
type Package struct {
	gorm.Model
	Name string `json:"Name"`
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
