package models

import "gorm.io/gorm"

// Commit represents an OSTree commit from image builder
type Commit struct {
	gorm.Model
	Name                 string    `json:"Name"`
	Account              string    `json:"Account"`
	ImageBuildHash       string    `json:"ImageBuildHash"`
	ImageBuildParentHash string    `json:"ImageBuildParentHash"`
	ImageBuildTarURL     string    `json:"ImageBuildTarURL"`
	OSTreeCommit         string    `json:"OSTreeCommit"`
	OSTreeParentCommit   string    `json:"OSTreeParentCommit"`
	OSTreeRef            string    `json:"OSTreeRef"`
	BuildDate            string    `json:"BuildDate"`
	BuildNumber          uint      `json:"BuildNumber"`
	BlueprintToml        string    `json:"BlueprintToml"`
	Arch                 string    `json:"Arch"`
	Packages             []Package `json:"Packages" gorm:"many2many:commit_packages;"`
}

type Package struct {
	gorm.Model
	Name string `json:"Name"`
}

func (c *Commit) GetPackagesList() *[]string {
	pkgs := make([]string, len(c.Packages))
	for i, p := range c.Packages {
		pkgs[i] = p.Name
	}
	return &pkgs
}
