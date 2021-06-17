package imagebuilder

import (
	"bytes"
	"encoding/json"
	"errors"
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
	Status       imageStatusValue `json:"status"`
	UploadStatus *UploadStatus    `json:"upload_status,omitempty"`
}

type imageStatusValue string

const (
	imageStatusBulding     imageStatusValue = "building"
	imageStatusFailure     imageStatusValue = "failure"
	imageStatusPending     imageStatusValue = "pending"
	imageStatusRegistering imageStatusValue = "registering"
	imageStatusSuccess     imageStatusValue = "success"
	imageStatusUploading   imageStatusValue = "uploading"
)

type UploadStatus struct {
	Options S3UploadStatus `json:"options"`
	Status  string         `json:"status"`
	Type    UploadTypes    `json:"type"`
}
type ComposeResult struct {
	Id string `json:"id"`
}

type S3UploadStatus struct {
	URL string `json:"url"`
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
	url := fmt.Sprintf("%s/compose", cfg.ImageBuilderConfig.URL)
	log.Infof("Requesting url: %s", url)
	req, _ := http.NewRequest("POST", url, payloadBuf)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("error requesting image builder")
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
	url := fmt.Sprintf("%s/composes/%s", cfg.ImageBuilderConfig.URL, image.ComposeJobID)
	req, _ := http.NewRequest("GET", url, nil)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		return nil, errors.New("error requesting image builder")
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
	if cs.ImageStatus.Status == imageStatusSuccess {
		image.Status = models.ImageStatusSuccess
		image.Commit.ImageBuildTarURL = cs.ImageStatus.UploadStatus.Options.URL
		// TODO: What to do if it's an installer?
	} else if cs.ImageStatus.Status == imageStatusFailure {
		image.Status = models.ImageStatusError
	}
	return image, nil
}
