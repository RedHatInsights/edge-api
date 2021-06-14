package models

import "gorm.io/gorm"

// Commit represents an OSTree commit from image builder
type Commit struct {
	gorm.Model
	Name                 string
	Account              string
	ImageBuildHash       string
	ImageBuildParentHash string
	ImageBuildTarURL     string
	OSTreeCommit         string
	OSTreeParentCommit   string
	OSTreeRef            string
	BuildDate            string
	BuildNumber          uint
	BlueprintToml        string
	NEVRAManifest        string
	Arch                 string
	Packages             []Package `gorm:"many2many:image_packages;"`
}

type Package struct {
	gorm.Model
	Name string
}

func (c *Commit) GetPackagesList() *[]string {
	pkgs := make([]string, len(c.Packages))
	for i, p := range c.Packages {
		pkgs[i] = p.Name
	}
	return &pkgs
}
