package models

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
	InstalledPackages    []InstalledPackage `json:"installed_packages,omitempty" gorm:"many2many:commit_installed_packages;"`
	ComposeJobID         string             `json:"compose_job_id"`
	Status               string             `json:"status"`
	RepoID               *uint              `json:"repo_id"`
	Repo                 *Repo              `json:"repo"`
}

// Repo is the delivery mechanism of a Commit over HTTP
type Repo struct {
	Model
	URL    string `json:"repo_url"`
	Status string `json:"repo_status"`
}

// Package represents the packages a Commit can have
type Package struct {
	Model
	Name string `json:"name"`
}

// InstalledPackage represents installed packages a image has
type InstalledPackage struct {
	Model
	Name      string `json:"name"`
	Arch      string `json:"arch"`
	Release   string `json:"release"`
	Sigmd5    string `json:"sigmd5"`
	Signature string `json:"signature"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Epoch     string `json:"epoch,omitempty"`
}
