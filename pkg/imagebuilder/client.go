package imagebuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/models"
)

var Client ImageBuilderClientInterface

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

type ComposeStatus struct {
	ImageStatus ImageStatus `json:"image_status"`
}
type ImageStatus struct {
	Status       ImageStatusValue `json:"status"`
	UploadStatus *UploadStatus    `json:"upload_status,omitempty"`
}

type ImageStatusValue string

// List of ImageStatusValue
const (
	ImageStatusValue_building    ImageStatusValue = "building"
	ImageStatusValue_failure     ImageStatusValue = "failure"
	ImageStatusValue_pending     ImageStatusValue = "pending"
	ImageStatusValue_registering ImageStatusValue = "registering"
	ImageStatusValue_success     ImageStatusValue = "success"
	ImageStatusValue_uploading   ImageStatusValue = "uploading"
)

type UploadStatus struct {
	Options interface{} `json:"options"`
	Status  string      `json:"status"`
	Type    UploadTypes `json:"type"`
}
type ComposeResult struct {
	Id string `json:"id"`
}

type ImageBuilderClientInterface interface {
	Compose(image *models.Image) (*models.Image, error)
	GetStatus(image *models.Image) (*models.Image, error)
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
	reqBody := &ComposeRequest{
		Customizations: &Customizations{
			Packages: image.Commit.GetPackagesList(),
		},
		Ostree: &OSTree{
			Ref: image.Commit.OSTreeRef,
			URL: image.Commit.OSTreeParentCommit,
		},
		Distribution:  image.Distribution,
		ImageRequests: []ImageRequest{imgReq},
	}

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(reqBody)
	cfg := config.Get()
	url := fmt.Sprintf("%s/compose", cfg.ImageBuilderConfig.Url)
	log.Info("Requesting url", url)
	req, _ := http.NewRequest("POST", url, payloadBuf)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	respBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(respBody, &cr)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	image.ComposeJobID = cr.Id
	image.Status = models.ImageStatusBuilding

	return image, nil
}

func (c *ImageBuilderClient) GetStatus(image *models.Image) (*models.Image, error) {
	cs := &ComposeStatus{}
	cfg := config.Get()
	url := fmt.Sprintf("%s/composes/%s", cfg.ImageBuilderConfig.Url, image.ComposeJobID)
	req, _ := http.NewRequest("GET", url, nil)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	respBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(respBody, &cs)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	if cs.ImageStatus.Status == ImageStatusValue_success {
		image.Status = models.ImageStatusSuccess
	} else if cs.ImageStatus.Status == ImageStatusValue_failure {
		image.Status = models.ImageStatusError
	}
	// TODO: We might wanna update db here
	return image, nil
}
