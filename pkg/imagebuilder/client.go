package imagebuilder

import "github.com/redhatinsights/edge-api/pkg/models"

// A lot of this code comes from https://github.com/osbuild/osbuild-composer

type OSTree struct {
	URL string `json:"url"`
	Ref string `json:"ref"`
}

type Customizations struct {
	Packages *[]string `json:"packages,omitempty"`
}

type UploadRequest struct {
	Options interface{} `json:"options"`
	Type    string      `json:"type"`
}

type UploadTypes string
type ImageRequest struct {
	Architecture  string        `json:"architecture"`
	ImageType     string        `json:"image_type"`
	UploadRequest UploadRequest `json:"upload_request"`
}

type ComposeRequest struct {
	Customizations *Customizations `json:"customizations,omitempty"`
	Distribution   string          `json:"distribution"`
	ImageRequests  []ImageRequest  `json:"image_requests"`
	Ostree         *OSTree         `json:"ostree,omitempty"`
}

type ComposeResult struct {
	Id string `json:"id"`
}

type ImageBuilderClientInterface interface {
	Compose(image models.Image) (*ComposeResult, error)
}

type ImageBuilderClient struct{}

func (c *ImageBuilderClient) Compose(image models.Image) (*ComposeResult, error) {
	cr := &ComposeResult{}
	return cr, nil
}
