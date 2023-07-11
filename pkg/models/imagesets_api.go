package models

type ImageSetAPI struct {
	Model
	Name    string  `json:"name" example:"my-edge-image"`
	Version int     `json:"version" example:"1"`
	Images  []Image `json:"Images"`
}

// ImageSetImagePackagesAPI return info related to details on images from imageset
type ImageSetImagePackagesAPI struct {
	ImageSetData     ImageSetAPI      `json:"image_set"`
	Images           []ImageDetailAPI `json:"images"`
	ImageBuildISOURL string           `json:"image_build_iso_url"`
}

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

// ImageSetInstallerURLAPI returns ImagesetAPI structure with last installer available
type ImageSetInstallerURLAPI struct {
	ImageSetData     ImageSetAPI `json:"image_set"`
	ImageBuildISOURL *string     `json:"image_build_iso_url" example:"https://buket.example.com"`
}

// ImageSetsResponseAPI is a struct for auto-generation of openapi.json
type ImageSetsResponseAPI struct {
	Count int                       `json:"Count" example:"100"`
	Data  []ImageSetInstallerURLAPI `json:"Data"`
}

// ImageSetDetailsResponseAPI is a struct for auto-generation of openapi.json
type ImageSetDetailsResponseAPI struct {
	Count int                      `json:"Count" example:"100"`
	Data  ImageSetImagePackagesAPI `json:"Data"`
}
