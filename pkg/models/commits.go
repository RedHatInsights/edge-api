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
	Account              string             `json:"Account"`
	OrgID                string             `json:"org_id"`
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
	InstalledPackages    []InstalledPackage `json:"InstalledPackages,omitempty" gorm:"many2many:commit_installed_packages;"`
	ComposeJobID         string             `json:"ComposeJobID"`
	Status               string             `json:"Status"`
	RepoID               *uint              `json:"RepoID"`
	Repo                 *Repo              `json:"Repo"`
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
	Name      string `json:"name"`
	Arch      string `json:"arch"`
	Release   string `json:"release"`
	Sigmd5    string `json:"sigmd5"`
	Signature string `json:"signature"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Epoch     string `json:"epoch,omitempty"`
}
