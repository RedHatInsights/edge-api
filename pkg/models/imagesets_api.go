package models

type ImageSetAPI struct {
	Model
	Name    string  `json:"name" example:"my-edge-image"` // the image set name
	Version int     `json:"version" example:"1"`          // the image set version
	Images  []Image `json:"Images"`                       // images of image set

}

// ImageSetImagePackagesAPI return info related to details on images from imageset
type ImageSetImagePackagesAPI struct {
	ImageSetData     ImageSetAPI      `json:"image_set"`                                                   // image set data
	Images           []ImageDetailAPI `json:"images"`                                                      // image detail
	ImageBuildISOURL string           `json:"image_build_iso_url" example:"/api/edge/v1/storage/isos/432"` // The image-set latest available image ISO
}

// ImageSetsViewResponseAPI is the image-set row returned for ui image-sets display
type ImageSetsViewResponseAPI struct {
	Count int            `json:"count" example:"10"` // count of image-sets
	Data  []ImageSetView `json:"data"`               // data of image set view
}

// ImageSetIDViewAPI is the image set details view returned for ui image-set display
type ImageSetIDViewAPI struct {
	ImageBuildIsoURL string         `json:"ImageBuildIsoURL" example:"/api/edge/v1/storage/isos/432"` // The image-set latest available image ISO
	ImageSet         ImageSetAPI    `json:"ImageSet"`                                                 // image set data
	LastImageDetails ImageDetailAPI `json:"LastImageDetails"`                                         // The image-set latest image details
}

// ImageSetImageIDViewAPI is the image set image view returned for ui image-set / version display
type ImageSetImageIDViewAPI struct {
	ImageBuildIsoURL string         `json:"ImageBuildIsoURL" example:"/api/edge/v1/storage/isos/432"` // The image-set latest available image ISO
	ImageSet         ImageSetAPI    `json:"ImageSet"`                                                 // image set data
	ImageDetails     ImageDetailAPI `json:"ImageDetails"`                                             // the requested image details
}

// ImageSetDevicesAPI contains the count and data for a list of Imagesets
type ImageSetDevicesAPI struct {
	Count int      `json:"Count" example:"10"` // count of image-set's devices
	Data  []string `json:"Data"`               // Data of image set's devices
}

// ImageSetInstallerURLAPI returns ImagesetAPI structure with last installer available
type ImageSetInstallerURLAPI struct {
	ImageSetData     ImageSetAPI `json:"image_set"`                                                   // image set data
	ImageBuildISOURL *string     `json:"image_build_iso_url" example:"/api/edge/v1/storage/isos/432"` // The image-set latest available image ISO
}

// ImageSetsResponseAPI is a struct for auto-generation of openapi.json
type ImageSetsResponseAPI struct {
	Count int                       `json:"Count" example:"10"` // count of image-sets
	Data  []ImageSetInstallerURLAPI `json:"Data"`               // all data of image-sets
}

// ImageSetDetailsResponseAPI is a struct for auto-generation of openapi.json
type ImageSetDetailsResponseAPI struct {
	Count int                      `json:"Count" example:"10"` // count of image-sets
	Data  ImageSetImagePackagesAPI `json:"Data"`               // all data of image-sets

}
