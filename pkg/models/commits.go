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
}
