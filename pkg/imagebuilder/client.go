package imagebuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var Client *ImageBuilderClient

func InitClient() {
	Client = new(ImageBuilderClient)
}

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
	Architecture  string         `json:"architecture"`
	ImageType     string         `json:"image_type"`
	UploadRequest *UploadRequest `json:"upload_request"`
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

func (c *ImageBuilderClient) Compose(image *models.Image) (*models.Image, error) {
	cr := &ComposeResult{}
	imgReq := ImageRequest{
		Architecture: image.Commit.Arch,
		ImageType:    "rhel-edge-commit",
		UploadRequest: &UploadRequest{
			Options: nil,
			Type:    "aws.s3",
		},
	}
	body := &ComposeRequest{
		Customizations: &Customizations{
			Packages: image.Packages,
		},
		Ostree: &OSTree{
			Ref: image.Commit.OSTreeRef,
			URL: image.Commit.OSTreeParentCommit,
		},
		Distribution:  image.Distribution,
		ImageRequests: []ImageRequest{imgReq},
	}

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(body)
	cfg := config.Get()
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/compose", cfg.ImageBuilderConfig.Url), payloadBuf)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	image.ComposeJobID = cr.Id

	return image, nil
}
