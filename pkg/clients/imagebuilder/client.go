package imagebuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/models"
)

// ClientInterface is an Interface to make request to ImageBuilder
type ClientInterface interface {
	ComposeCommit(image *models.Image) (*models.Image, error)
	ComposeInstaller(repo *models.Repo, image *models.Image) (*models.Image, error)
	GetCommitStatus(image *models.Image) (*models.Image, error)
	GetInstallerStatus(image *models.Image) (*models.Image, error)
	GetMetadata(image *models.Image) (*models.Image, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
}

// InitClient initializes the client for Image Builder
func InitClient(ctx context.Context) *Client {
	return &Client{ctx: ctx}
}

// A lot of this code comes from https://github.com/osbuild/osbuild-composer

// OSTree gives OSTree information for an image
type OSTree struct {
	URL string `json:"url"`
	Ref string `json:"ref"`
}

// Customizations is made of the packages that are baked into an image
type Customizations struct {
	Packages *[]string `json:"packages"`
}

// UploadRequest is the upload options accepted by Image Builder API
type UploadRequest struct {
	Options interface{} `json:"options"`
	Type    string      `json:"type"`
}

// UploadTypes is the type that represents the types of uploads accepted by Image Builder
type UploadTypes string

// ImageRequest is image-related part of a ComposeRequest
type ImageRequest struct {
	Architecture  string         `json:"architecture"`
	ImageType     string         `json:"image_type"`
	Ostree        *OSTree        `json:"ostree,omitempty"`
	UploadRequest *UploadRequest `json:"upload_request"`
}

// ComposeRequest is the request to Compose one or more Images
type ComposeRequest struct {
	Customizations *Customizations `json:"customizations"`
	Distribution   string          `json:"distribution"`
	ImageRequests  []ImageRequest  `json:"image_requests"`
}

// ComposeStatus is the status of a ComposeRequest
type ComposeStatus struct {
	ImageStatus ImageStatus `json:"image_status"`
}

// ImageStatus is the status of the upload of an Image
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

// UploadStatus is the status and metadata of an Image upload
type UploadStatus struct {
	Options S3UploadStatus `json:"options"`
	Status  string         `json:"status"`
	Type    UploadTypes    `json:"type"`
}

// ComposeResult has the ID of a ComposeRequest
type ComposeResult struct {
	ID string `json:"id"`
}

// S3UploadStatus contains the URL to the S3 Bucket
type S3UploadStatus struct {
	URL string `json:"url"`
}

// OsTree struct to get the metadata response
type OsTree struct {
	OstreeCommit      string             `json:"ostree_commit"`
	InstalledPackages []InstalledPackage `json:"packages"`
}

// InstalledPackage contains the metadata of the packages installed on a image
type InstalledPackage struct {
	Arch      string `json:"arch"`
	Name      string `json:"name"`
	Release   string `json:"release"`
	Sigmd5    string `json:"sigmd5"`
	Signature string `json:"signature"`
	Type      string `json:"type"`
	Version   string `json:"version"`
	Epoch     string `json:"epoch,omitempty"`
}

func (c *Client) compose(composeReq *ComposeRequest) (*ComposeResult, error) {
	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(composeReq)
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/image-builder/v1/compose", cfg.ImageBuilderConfig.URL)
	log.Infof("Requesting url: %s with payloadBuf %s", url, payloadBuf.String())
	req, _ := http.NewRequest("POST", url, payloadBuf)
	for key, value := range common.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusCreated {
		body, _ := ioutil.ReadAll(res.Body)
		return nil, fmt.Errorf("error requesting image builder, got status code %d and body %s", res.StatusCode, body)
	}
	respBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	cr := &ComposeResult{}
	err = json.Unmarshal(respBody, &cr)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	return cr, nil
}

// ComposeCommit composes a Commit on ImageBuilder
func (c *Client) ComposeCommit(image *models.Image) (*models.Image, error) {
	req := &ComposeRequest{
		Customizations: &Customizations{
			Packages: image.Commit.GetPackagesList(),
		},

		Distribution: image.Distribution,
		ImageRequests: []ImageRequest{
			{
				Architecture: image.Commit.Arch,
				ImageType:    models.ImageTypeCommit,
				UploadRequest: &UploadRequest{
					Options: make(map[string]string),
					Type:    "aws.s3",
				},
			}},
	}
	if image.Commit.OSTreeRef != "" {
		if req.ImageRequests[0].Ostree == nil {
			req.ImageRequests[0].Ostree = &OSTree{}
		}
		req.ImageRequests[0].Ostree.Ref = image.Commit.OSTreeRef
	}
	if image.Commit.OSTreeRef != "" {
		if req.ImageRequests[0].Ostree == nil {
			req.ImageRequests[0].Ostree = &OSTree{}
		}
		req.ImageRequests[0].Ostree.URL = image.Commit.OSTreeParentCommit
	}

	cr, err := c.compose(req)
	if err != nil {
		return nil, err
	}
	image.Commit.ComposeJobID = cr.ID
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	return image, nil
}

// ComposeInstaller composes a Installer on ImageBuilder
func (c *Client) ComposeInstaller(repo *models.Repo, image *models.Image) (*models.Image, error) {
	pkgs := make([]string, 0)
	req := &ComposeRequest{
		Customizations: &Customizations{
			Packages: &pkgs,
		},

		Distribution: image.Distribution,
		ImageRequests: []ImageRequest{
			{
				Architecture: image.Commit.Arch,
				ImageType:    models.ImageTypeInstaller,
				Ostree: &OSTree{
					Ref: "rhel/8/x86_64/edge", //image.Commit.OSTreeRef,
					URL: repo.URL,
				},
				UploadRequest: &UploadRequest{
					Options: make(map[string]string),
					Type:    "aws.s3",
				},
			}},
	}
	cr, err := c.compose(req)
	if err != nil {
		return nil, err
	}
	image.Installer.ComposeJobID = cr.ID
	image.Installer.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	return image, nil
}

func (c *Client) getComposeStatus(jobID string) (*ComposeStatus, error) {
	cs := &ComposeStatus{}
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/image-builder/v1/composes/%s", cfg.ImageBuilderConfig.URL, jobID)
	req, _ := http.NewRequest("GET", url, nil)
	for key, value := range common.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	req.Header.Add("Content-Type", "application/json")
	log.Infof("Requesting url: %s", url)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		return nil, fmt.Errorf("error requesting image builder, got status code %d and body %s", res.StatusCode, body)
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
	return cs, nil
}

// GetCommitStatus gets the Commit status on Image Builder
func (c *Client) GetCommitStatus(image *models.Image) (*models.Image, error) {
	cs, err := c.getComposeStatus(image.Commit.ComposeJobID)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Got UpdateCommitID status %s", cs.ImageStatus.Status))
	if cs.ImageStatus.Status == imageStatusSuccess {
		image.Commit.Status = models.ImageStatusSuccess
		image.Commit.ImageBuildTarURL = cs.ImageStatus.UploadStatus.Options.URL
	} else if cs.ImageStatus.Status == imageStatusFailure {
		image.Commit.Status = models.ImageStatusError
		image.Status = models.ImageStatusError
	}
	log.Info(fmt.Sprintf("Set image status %s", image.Status))
	return image, nil
}

// GetInstallerStatus gets the Installer status on Image Builder
func (c *Client) GetInstallerStatus(image *models.Image) (*models.Image, error) {
	cs, err := c.getComposeStatus(image.Installer.ComposeJobID)
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Got installer status %s", cs.ImageStatus.Status))
	if cs.ImageStatus.Status == imageStatusSuccess {
		image.Installer.Status = models.ImageStatusSuccess
		image.Installer.ImageBuildISOURL = cs.ImageStatus.UploadStatus.Options.URL
	} else if cs.ImageStatus.Status == imageStatusFailure {
		image.Installer.Status = models.ImageStatusError
		image.Status = models.ImageStatusError
	}
	return image, nil
}

func (c *Client) GetMetadata(image *models.Image) (*models.Image, error) {
	log.Infof("Getting metadata for image ID %d", image.ID)
	composeJobId := image.Commit.ComposeJobID
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/image-builder/v1/composes/%s/metadata", cfg.ImageBuilderConfig.URL, composeJobId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range common.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	req.Header.Add("Content-Type", "application/json")
	log.Infof("Requesting url: %s", url)
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var ostree_struct OsTree
	json.Unmarshal(data, &ostree_struct)
	for n := range ostree_struct.InstalledPackages {
		pkg := models.InstalledPackage{

			Arch: ostree_struct.InstalledPackages[n].Arch, Name: ostree_struct.InstalledPackages[n].Name,
			Release: ostree_struct.InstalledPackages[n].Release, Sigmd5: ostree_struct.InstalledPackages[n].Sigmd5,
			Signature: ostree_struct.InstalledPackages[n].Signature, Type: ostree_struct.InstalledPackages[n].Type,
			Version: ostree_struct.InstalledPackages[n].Version, Epoch: ostree_struct.InstalledPackages[n].Epoch,
		}
		image.Commit.InstalledPackages = append(image.Commit.InstalledPackages, pkg)
	}
	image.Commit.OSTreeCommit = ostree_struct.OstreeCommit
	defer res.Body.Close()
	log.Infof("Done with metadata for image ID %d", image.ID)
	return image, nil
}
