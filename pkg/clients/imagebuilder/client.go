// Package imagebuilder provides Image Builder API client functions
// FIXME: golangci-lint
// nolint:errcheck,govet,revive
package imagebuilder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/pkg/clients"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/redhatinsights/edge-api/config"
)

// ClientInterface is an Interface to make request to ImageBuilder
type ClientInterface interface {
	ComposeCommit(image *models.Image) (*models.Image, error)
	ComposeInstaller(image *models.Image) (*models.Image, error)
	GetCommitStatus(image *models.Image) (*models.Image, error)
	GetInstallerStatus(image *models.Image) (*models.Image, error)
	GetMetadata(image *models.Image) (*models.Image, error)
	SearchPackage(packageName string, arch string, dist string) (*models.SearchPackageResult, error)
	ValidatePackages(pkg []string) (map[uint]*models.InstalledPackage, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
}
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var ImageBuilderHTTPClient HTTPClient

func init() {
	ImageBuilderHTTPClient = &http.Client{}
}

// InitClient initializes the client for Image Builder
func InitClient(ctx context.Context, log *log.Entry) *Client {
	return &Client{ctx: ctx, log: log}
}

// A lot of this code comes from https://github.com/osbuild/osbuild-composer

// OSTree gives OSTree information for an image
type OSTree struct {
	URL        string `json:"url,omitempty"`
	ContentURL string `json:"contenturl,omitempty"`
	RHSM       bool   `json:"rhsm,omitempty"`
	Ref        string `json:"ref"`
	ParentRef  string `json:"parent,omitempty"`
}

// Customizations is made of the packages that are baked into an image
type Customizations struct {
	Packages            *[]string     `json:"packages"`
	PayloadRepositories *[]Repository `json:"payload_repositories,omitempty"`
}

// Repository is the record of Third Party Repository
type Repository struct {
	BaseURL    string  `json:"baseurl"`
	CheckGPG   *bool   `json:"check_gpg,omitempty"`
	GPGKey     *string `json:"gpg_key,omitempty"`
	IgnoreSSL  *bool   `json:"ignore_ssl,omitempty"`
	MetaLink   *string `json:"metalink,omitempty"`
	MirrorList *string `json:"mirrorlist,omitempty"`
	RHSM       bool    `json:"rhsm"`
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
	Reason       imageStatusValue `json:"reason"`
}

type imageStatusValue string

const (
	imageStatusBuilding    imageStatusValue = "building"
	imageStatusFailure     imageStatusValue = "failure"
	imageStatusPending     imageStatusValue = "pending"
	imageStatusRegistering imageStatusValue = "registering"
	imageStatusSuccess     imageStatusValue = "success"
	imageStatusUploading   imageStatusValue = "uploading"
	imageReasonFailure     imageStatusValue = "Worker running this job stopped responding"
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

// PackageRequestError indicates request search packages from Image Builder
type PackageRequestError struct{}

func (e *PackageRequestError) Error() string {
	return "image builder search packages request error"
}

// Metadata struct to get the metadata response
type Metadata struct {
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
	if err := json.NewEncoder(payloadBuf).Encode(composeReq); err != nil {
		return nil, err
	}
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/image-builder/v1/compose", cfg.ImageBuilderConfig.URL)
	c.log.WithFields(log.Fields{
		"url":     url,
		"payload": payloadBuf.String(),
	}).Debug("Image Builder Compose Request Started")
	req, _ := http.NewRequest("POST", url, payloadBuf)
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	req.Header.Add("Content-Type", "application/json")

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("Image Builder Compose Request Error")
		return nil, err
	}
	respBody, err := io.ReadAll(res.Body)
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(respBody),
		"error":        err,
	}).Debug("Image Builder Compose Response")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("image is not being created by image builder")
	}

	cr := &ComposeResult{}
	err = json.Unmarshal(respBody, &cr)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("Error unmarshalling response JSON")
		return nil, err
	}

	return cr, nil
}

// ComposeCommit composes a Commit on ImageBuilder
func (c *Client) ComposeCommit(image *models.Image) (*models.Image, error) {
	payloadRepos, err := c.GetImageThirdPartyRepos(image)
	if err != nil {
		return nil, errors.New("error getting information on third Party repository")
	}
	req := &ComposeRequest{
		Customizations: &Customizations{
			Packages:            image.GetALLPackagesList(),
			PayloadRepositories: &payloadRepos,
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
	if image.Commit.OSTreeParentCommit != "" {
		if req.ImageRequests[0].Ostree == nil {
			req.ImageRequests[0].Ostree = &OSTree{}
		}
		req.ImageRequests[0].Ostree.URL = image.Commit.OSTreeParentCommit
		if feature.StorageImagesRepos.IsEnabled() {
			req.ImageRequests[0].Ostree.RHSM = true
			// because of some redirect failures (SSL connect), use the same url as URL
			req.ImageRequests[0].Ostree.ContentURL = req.ImageRequests[0].Ostree.URL
		}

		if image.Commit.OSTreeRef != "" && image.Commit.OSTreeParentRef != "" && image.Commit.OSTreeRef != image.Commit.OSTreeParentRef {
			req.ImageRequests[0].Ostree.ParentRef = image.Commit.OSTreeParentRef
		}
	}

	cr, err := c.compose(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("Error sending request to image builder")
		return nil, err
	}
	image.Commit.ComposeJobID = cr.ID
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	return image, nil
}

// ComposeInstaller composes a Installer on ImageBuilder
func (c *Client) ComposeInstaller(image *models.Image) (*models.Image, error) {
	pkgs := make([]string, 0)
	var repoURL string
	var rhsm bool
	if feature.StorageImagesRepos.IsEnabled() {
		repoURL = fmt.Sprintf("%s/api/edge/v1/storage/images-repos/%d", config.Get().EdgeCertAPIBaseURL, image.ID)
		rhsm = true
	} else {
		repoURL = image.Commit.Repo.URL
		rhsm = false
	}

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
					Ref:        image.Commit.OSTreeRef,
					URL:        repoURL,
					ContentURL: repoURL, // because of some redirect failures (SSL connect), use the same url as URL
					RHSM:       rhsm,
				},
				UploadRequest: &UploadRequest{
					Options: make(map[string]string),
					Type:    "aws.s3",
				},
			}},
	}
	cr, err := c.compose(req)
	if err != nil {
		image.Installer.Status = models.ImageStatusError
		image.Status = models.ImageStatusError
	} else {
		image.Installer.ComposeJobID = cr.ID
		image.Installer.Status = models.ImageStatusBuilding
		image.Status = models.ImageStatusBuilding

	}
	image.Commit.ExternalURL = false
	tx := db.DB.Save(&image)
	if tx.Error != nil {
		c.log.WithField("error", tx.Error.Error()).Error("Error saving image")
	}
	tx = db.DB.Save(&image.Installer)
	if tx.Error != nil {
		c.log.WithField("error", tx.Error.Error()).Error("Error saving installer")
	}
	if err != nil {
		return nil, err
	}
	return image, nil
}

// GetComposeStatus returns a compose job status given a specific ID
func (c *Client) GetComposeStatus(jobID string) (*ComposeStatus, error) {
	cs := &ComposeStatus{}
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/image-builder/v1/composes/%s", cfg.ImageBuilderConfig.URL, jobID)
	c.log.Info("Image Builder ComposeStatus Request Started")
	req, _ := http.NewRequest("GET", url, nil)
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error("Image Builder ComposeStatus Request Error")
		return nil, err
	}
	body, err := io.ReadAll(res.Body)
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(body),
		"error":        err,
	}).Debug("Image Builder ComposeStatus Response")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request for status was not successful")
	}

	err = json.Unmarshal(body, &cs)
	if cs.ImageStatus.Reason == imageReasonFailure && cs.ImageStatus.Status == imageStatusFailure {
		return nil, fmt.Errorf("worker running this job stopped responding")
	}
	if err != nil {
		return nil, err
	}

	return cs, nil
}

// GetCommitStatus gets the Commit status on Image Builder
func (c *Client) GetCommitStatus(image *models.Image) (*models.Image, error) {
	cs, err := c.GetComposeStatus(image.Commit.ComposeJobID)
	if err != nil {
		return nil, err
	}
	c.log.WithField("status", cs.ImageStatus.Status).Info("Got commit response status")
	if cs.ImageStatus.Status == imageStatusSuccess {
		c.log.Info("Set image commit status with success")
		image.Commit.Status = models.ImageStatusSuccess
		image.Commit.ImageBuildTarURL = cs.ImageStatus.UploadStatus.Options.URL
		image.Commit.ExternalURL = true
	} else if cs.ImageStatus.Status == imageStatusFailure {
		c.log.Info("Set image and image commit status with error")
		image.Commit.Status = models.ImageStatusError
		image.Status = models.ImageStatusError
	}
	return image, nil
}

// GetInstallerStatus gets the Installer status on Image Builder
func (c *Client) GetInstallerStatus(image *models.Image) (*models.Image, error) {
	cs, err := c.GetComposeStatus(image.Installer.ComposeJobID)
	if err != nil {
		return nil, err
	}
	c.log.WithField("status", cs.ImageStatus.Status).Info("Got installer response status")
	if cs.ImageStatus.Status == imageStatusSuccess {
		c.log.Info("Set image installer status with success")
		image.Installer.Status = models.ImageStatusSuccess
		image.Installer.ImageBuildISOURL = cs.ImageStatus.UploadStatus.Options.URL
	} else if cs.ImageStatus.Status == imageStatusFailure {
		c.log.Info("Set image and image installer status with error")
		image.Installer.Status = models.ImageStatusError
		image.Status = models.ImageStatusError
	}
	return image, nil
}

// GetMetadata returns the metadata on image builder for a particular image based on the ComposeJobID
func (c *Client) GetMetadata(image *models.Image) (*models.Image, error) {
	c.log.Info("Getting metadata for image")
	composeJobID := image.Commit.ComposeJobID
	cfg := config.Get()
	url := fmt.Sprintf("%s/api/image-builder/v1/composes/%s/metadata", cfg.ImageBuilderConfig.URL, composeJobID)
	c.log.WithFields(log.Fields{
		"url": url,
	}).Info("Image Builder GetMetadata Request Started")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	req.Header.Add("Content-Type", "application/json")

	res, err := ImageBuilderHTTPClient.Do(req)

	if err != nil {
		c.log.WithField("error", err.Error()).Error("Image Builder GetMetadata Request Error")
		return nil, err
	}
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	c.log.WithFields(log.Fields{
		"statusCode":   res.StatusCode,
		"responseBody": string(respBody),
		"error":        err,
	}).Info("Image Builder GetMetadata Response")
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("image metadata not found")
	}

	var metadata Metadata
	if err := json.Unmarshal(respBody, &metadata); err != nil {
		c.log.WithField("response", metadata).Error("Error while trying to unmarshal Image Builder GetMetadata Response")
		return nil, err
	}

	var dupPackages []uint
	var metadataPackages []string
	for n := range metadata.InstalledPackages {
		metadataPackages = append(metadataPackages, fmt.Sprintf("%s-%s-%s", metadata.InstalledPackages[n].Name, metadata.InstalledPackages[n].Release, metadata.InstalledPackages[n].Version))
	}

	var pkg map[uint]*models.InstalledPackage
	var cip []models.CommitInstalledPackages

	packagesExistsMap := make(map[string]bool)

	if feature.DedupPackage.IsEnabled() {
		pkg, err = c.ValidatePackages(metadataPackages)
		if len(pkg) > 0 && err == nil {
			for key := range pkg {
				dupPackages = append(dupPackages, key)
				packagesExistsMap[pkg[key].Name] = true
			}
		}
	}

	for n := range metadata.InstalledPackages {

		if feature.DedupPackage.IsEnabled() {
			if packagesExistsMap[metadata.InstalledPackages[n].Name] == true {
				continue
			} else {
				pkg := models.InstalledPackage{
					Arch: metadata.InstalledPackages[n].Arch, Name: metadata.InstalledPackages[n].Name,
					Release: metadata.InstalledPackages[n].Release, Sigmd5: metadata.InstalledPackages[n].Sigmd5,
					Signature: metadata.InstalledPackages[n].Signature, Type: metadata.InstalledPackages[n].Type,
					Version: metadata.InstalledPackages[n].Version, Epoch: metadata.InstalledPackages[n].Epoch,
				}
				image.Commit.InstalledPackages = append(image.Commit.InstalledPackages, pkg)
			}

		} else {
			pkg := models.InstalledPackage{
				Arch: metadata.InstalledPackages[n].Arch, Name: metadata.InstalledPackages[n].Name,
				Release: metadata.InstalledPackages[n].Release, Sigmd5: metadata.InstalledPackages[n].Sigmd5,
				Signature: metadata.InstalledPackages[n].Signature, Type: metadata.InstalledPackages[n].Type,
				Version: metadata.InstalledPackages[n].Version, Epoch: metadata.InstalledPackages[n].Epoch,
			}
			image.Commit.InstalledPackages = append(image.Commit.InstalledPackages, pkg)
		}
	}

	image.Commit.OSTreeCommit = metadata.OstreeCommit
	db.DB.Omit("Image.InstalledPackages.*").Save(image.Commit)
	if feature.DedupPackage.IsEnabled() &&
		len(dupPackages) > 0 {

		for i := range dupPackages {
			cip = append(cip, models.CommitInstalledPackages{InstalledPackageId: dupPackages[i], CommitId: image.Commit.ID})
		}

		err := db.DB.Create(&cip)
		if err.Error != nil {
			c.log.WithField("error", err.Error.Error()).Error(new(PackageRequestError))
			return nil, err.Error
		}

	}

	c.log.Info("Done with metadata for image")
	return image, nil
}

// GetImageThirdPartyRepos finds the url of Third Party Repository using the name
func (c *Client) GetImageThirdPartyRepos(image *models.Image) ([]Repository, error) {
	if len(image.ThirdPartyRepositories) == 0 {
		return []Repository{}, nil
	}
	if image.OrgID == "" {
		return nil, errors.New("error retrieving orgID  information, image orgID undefined")
	}
	repos := make([]Repository, len(image.ThirdPartyRepositories))
	thirdpartyrepos := make([]models.ThirdPartyRepo, len(image.ThirdPartyRepositories))
	thirdpartyrepoIDS := make([]int, len(image.ThirdPartyRepositories))

	for repo := range image.ThirdPartyRepositories {
		thirdpartyrepoIDS[repo] = int(image.ThirdPartyRepositories[repo].ID)
	}
	var count int64
	result := db.Org(image.OrgID, "").Find(&thirdpartyrepos, thirdpartyrepoIDS).Count(&count)
	if result.Error != nil {
		c.log.WithField("error", result.Error).Error("Error finding custom repositories")
		return nil, result.Error
	}

	if count != int64(len(thirdpartyrepoIDS)) {
		return nil, errors.New("enter valid third party repository id")
	}
	for i := 0; i < len(thirdpartyrepos); i++ {
		repos[i] = Repository{
			BaseURL: thirdpartyrepos[i].URL,
		}
	}

	return repos, nil
}

// SearchPackage validate package name with Image Builder API
func (c *Client) SearchPackage(packageName string, arch string, dist string) (*models.SearchPackageResult, error) {
	c.log.Infof("Searching rhel package")
	cfg := config.Get()
	if packageName == "" || arch == "" || dist == "" {
		return nil, errors.New("mandatory fields should not be empty")
	}

	// build the correct URL using the package name
	url := fmt.Sprintf("%s/api/image-builder/v1/packages?distribution=%s&architecture=%s&search=%s", cfg.ImageBuilderConfig.URL, dist, arch, url.QueryEscape(packageName))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range clients.GetOutgoingHeaders(c.ctx) {
		req.Header.Add(key, value)
	}
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		c.log.WithField("error", err.Error()).Error(new(PackageRequestError))
		return nil, err
	}
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		c.log.WithFields(log.Fields{
			"statusCode": res.StatusCode,
		}).Error(new(PackageRequestError))
		return nil, new(PackageRequestError)
	}
	var searchResult models.SearchPackageResult
	err = json.Unmarshal(respBody, &searchResult)
	if err != nil {
		c.log.WithField("error", err.Error()).Error(new(PackageRequestError))
		return nil, err
	}
	return &searchResult, nil
}

func (c *Client) ValidatePackages(pkgs []string) (map[uint]*models.InstalledPackage, error) {
	var result []models.InstalledPackage
	setOfPackages := make(map[uint]*models.InstalledPackage)

	err := db.DB.Table("Installed_Packages").Select("ID, name,release, arch, version, epoch").
		Where("( (name || '-' || release || '-' ||  version)) in (?)", pkgs).
		Find(&result)
	if err.Error != nil {
		c.log.WithField("error", err.Error.Error()).Error(new(PackageRequestError))
		return nil, err.Error
	} else {
		if len(result) > 0 {
			for n := range result {
				setOfPackages[result[n].ID] = &result[n]
			}
			return setOfPackages, nil
		}
		return nil, nil
	}
}
