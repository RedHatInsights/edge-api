package models

import "database/sql"

type ImageSetAPI struct {
	Name    string  `json:"name" example:"my-edge-image"`
	Version int     `json:"version" example:"1"`
	Account string  `json:"Account" example:"0000"`
	Images  []Image `json:"Images"`
}

// ImageSetInstallerURLAPI returns ImagesetAPI structure with last installer available
type ImageSetInstallerURLAPI struct {
	ImageSetData     ImageSetAPI `json:"image_set"`
	ImageBuildISOURL *string     `json:"image_build_iso_url" example:"https://buket.example.com"`
}

// ImageDetailAPI return the structure to inform package info to images
type ImageDetailAPI struct {
	Image              *Image `json:"image"`
	AdditionalPackages int    `json:"additional_packages" example:"3"`
	Packages           int    `json:"packages" example:"3"`
	UpdateAdded        int    `json:"update_added" example:"3"`
	UpdateRemoved      int    `json:"update_removed" example:"2"`
	UpdateUpdated      int    `json:"update_updated" example:"3"`
}

// ImageSetImagePackagesAPI return info related to details on images from imageset
type ImageSetImagePackagesAPI struct {
	ImageSetData     ImageSetAPI      `json:"image_set"`
	Images           []ImageDetailAPI `json:"images"`
	ImageBuildISOURL string           `json:"image_build_iso_url"`
}

// EdgeAPITimeAPI is a time.Time with a valid flag.
type EdgeAPITimeAPI sql.NullTime

// ImageSetsViewResponseAPI is the image-set row returned for ui image-sets display
type ImageSetsViewResponseAPI struct {
	Count int            `json:"count" example:"100"`
	Data  []ImageSetView `json:"data"`
}

// ImageSetIDViewAPI is the image set details view returned for ui image-set display
type ImageSetIDViewAPI struct {
	ImageBuildIsoURL string         `json:"ImageBuildIsoURL"`
	ImageSet         ImageSetAPI    `json:"ImageSet"`
	LastImageDetails ImageDetailAPI `json:"LastImageDetails"`
}

// ImagesViewDataAPI is the images view data return for images view with filters , limit, offSet
type ImagesViewDataAPI struct {
	Count int64       `json:"count"`
	Data  []ImageView `json:"data"`
}

// ImageSetImageIDViewAPI is the image set image view returned for ui image-set / version display
type ImageSetImageIDViewAPI struct {
	ImageBuildIsoURL string         `json:"ImageBuildIsoURL"`
	ImageSet         ImageSetAPI    `json:"ImageSet"`
	ImageDetails     ImageDetailAPI `json:"ImageDetails"`
}

// ImageSetDevicesAPI contains the count and data for a list of Imagesets
type ImageSetDevicesAPI struct {
	Count int      `json:"Count"`
	Data  []string `json:"Data"`
}
