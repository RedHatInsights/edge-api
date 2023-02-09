// Package services handles all service-related features
// FIXME: golangci-lint
// nolint:errcheck,gocritic,govet,revive
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/redhatinsights/edge-api/pkg/clients/imagebuilder"
	kafkacommon "github.com/redhatinsights/edge-api/pkg/common/kafka"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"

	"gorm.io/gorm"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

// ImageServiceInterface defines the interface that helps handle
// the business logic of creating RHEL For Edge Images.
// This interface is used for mock generation.
type ImageServiceInterface interface {
	CreateImage(image *models.Image) error
	ProcessImage(context.Context, *models.Image) error
	UpdateImage(image *models.Image, previousImage *models.Image) error
	AddUserInfo(image *models.Image) error
	UpdateImageStatus(image *models.Image) (*models.Image, error)
	SetErrorStatusOnImage(err error, i *models.Image)
	CreateRepoForImage(context.Context, *models.Image) (*models.Repo, error)
	CreateInstallerForImage(context.Context, *models.Image) (*models.Image, chan error, error)
	GetImageByID(id string) (*models.Image, error)
	GetImageDevicesCount(imageId uint) (int64, error)
	GetUpdateInfo(image models.Image) (*models.ImageUpdateAvailable, error)
	AddPackageInfo(image *models.Image) (ImageDetail, error)
	GetImageByOSTreeCommitHash(commitHash string) (*models.Image, error)
	CheckImageName(name, orgID string) (bool, error)
	RetryCreateImage(context.Context, *models.Image) error
	ResumeCreateImage(context.Context, *models.Image) error
	GetMetadata(image *models.Image) (*models.Image, error)
	SetFinalImageStatus(i *models.Image)
	CheckIfIsLatestVersion(previousImage *models.Image) error
	SetBuildingStatusOnImageToRetryBuild(image *models.Image) error
	GetRollbackImage(image *models.Image) (*models.Image, error)
	SendImageNotification(image *models.Image) (ImageNotification, error)
	SetDevicesUpdateAvailabilityFromImageSet(orgID string, ImageSetID uint) error
	ValidateImagePackage(pack string, image *models.Image) error
	GetImagesViewCount(tx *gorm.DB) (int64, error)
	GetImagesView(limit int, offset int, tx *gorm.DB) (*[]models.ImageView, error)
	SetLog(*log.Entry)
	DeleteImage(image *models.Image) error
}

// NewImageService gives a instance of the main implementation of a ImageServiceInterface
func NewImageService(ctx context.Context, log *log.Entry) ImageServiceInterface {
	return &ImageService{
		Service:      Service{ctx: ctx, log: log.WithField("service", "image")},
		ImageBuilder: imagebuilder.InitClient(ctx, log),
		RepoBuilder:  NewRepoBuilder(ctx, log),
		RepoService:  NewRepoService(ctx, log),
	}
}

// ImageService is the main implementation of a ImageServiceInterface
type ImageService struct {
	Service

	ImageBuilder imagebuilder.ClientInterface
	RepoBuilder  RepoBuilderInterface
	RepoService  RepoServiceInterface
}

// GetImageReposFromDB return ThirdParty repo of image by OrgID
func GetImageReposFromDB(orgID string, repos []models.ThirdPartyRepo) (*[]models.ThirdPartyRepo, error) {

	if orgID == "" {
		return nil, new(OrgIDNotSet)
	}
	var imagesRepos []models.ThirdPartyRepo
	for _, custRepo := range repos {
		var repo models.ThirdPartyRepo
		if custRepo.ID == 0 {
			return nil, new(ThirdPartyRepositoryNotFound)
		}
		if result := db.Org(orgID, "").First(&repo, custRepo.ID); result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				return nil, new(ThirdPartyRepositoryNotFound)
			}
			return nil, result.Error
		}
		imagesRepos = append(imagesRepos, repo)
	}
	return &imagesRepos, nil
}

func (s *ImageService) getImageSetForNewImage(orgID string, image *models.Image) (*models.ImageSet, error) {
	// Check for ImageSet existence, if imageSet does not exist create one,
	// if it exists and is not linked to any images reuse it,
	// if it exists and linked to any images return error
	var imageSet models.ImageSet
	if result := db.Org(orgID, "").Preload("Images").Where("(name = ?)", image.Name).First(&imageSet); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// Create a new imageSet
			imageSet = models.ImageSet{OrgID: orgID, Name: image.Name, Version: image.Version}
			if result := db.DB.Create(&imageSet); result.Error != nil {
				s.log.WithFields(log.Fields{
					"imageSetName": image.Name,
					"error":        result.Error.Error(),
				}).Error("error when creating a new imageSet")
				return nil, result.Error
			}
			s.log.WithField("imageSetName", image.Name).Debug("imageSet created")
			// return immediately
			return &imageSet, nil
		}
		s.log.WithFields(log.Fields{
			"imageSetName": image.Name,
			"error":        result.Error.Error(),
		}).Error("error when checking for previous imageSet existence")
		return nil, result.Error
	}
	// imageSet exists, check images existence
	if len(imageSet.Images) != 0 {
		s.log.WithField("imageSetName", image.Name).Error("imageSet already exists and linked to existing images")
		return nil, new(ImageSetAlreadyExists)
	}
	s.log.WithField("imageSetName", image.Name).Debug("imageSet already exists, with no images linked, imageSet will be reused")
	return &imageSet, nil
}

// CreateImage sets up the image for the EDA-based CreateImage
func (s *ImageService) CreateImage(image *models.Image) error {
	if image.OrgID == "" {
		return new(OrgIDNotSet)
	}
	if image.Name == "" {
		return new(ImageNameUndefined)
	}
	imageNameExists, err := s.CheckImageName(image.Name, image.OrgID)
	if err != nil {
		return err
	}
	if imageNameExists {
		return new(ImageNameAlreadyExists)
	}
	if image.Version == 0 {
		image.Version = 1
	}
	packages := image.Packages
	// we now need to loop this request for each package
	for _, p := range packages {
		er := s.ValidateImagePackage(p.Name, image)
		if er != nil {
			return er
		}
	}
	imagesrepos, err := GetImageReposFromDB(image.OrgID, image.ThirdPartyRepositories)
	if err != nil {
		return err
	}
	image.ThirdPartyRepositories = *imagesrepos
	// Send Image creation to notification
	notify, errNotify := s.SendImageNotification(image)
	if errNotify != nil {
		s.log.WithField("message", errNotify.Error()).Error("Error sending notification")
		s.log.WithField("message", notify).Error("Notify Error")
	}

	// TODO: REFACTOR... ImageSet should be created first and an image created from it
	imageSet, err := s.getImageSetForNewImage(image.OrgID, image)
	if err != nil {
		return err
	}

	// create an image under the new imageSet
	image.ImageSetID = &imageSet.ID
	// make the initial call to Image Builder
	// FIXME: for EDA this should happen on the consumer side
	image, err = s.ImageBuilder.ComposeCommit(image)
	if err != nil {
		return err
	}
	image.Commit.OrgID = image.OrgID
	// FIXME: Status below is already set in the call to ComposeCommit()
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding

	// TODO: Remove code when frontend is not using ImageType on the table
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.ImageType = models.ImageTypeInstaller
	} else {
		image.ImageType = models.ImageTypeCommit
	}

	if image.Installer != nil {
		image.Installer.OrgID = image.OrgID
	}
	// TODO: End of remove block

	if image.HasOutputType(models.ImageTypeInstaller) {
		image.Installer.Status = models.ImageStatusPending
		image.Installer.OrgID = image.OrgID
	}

	if result := db.DB.Create(&image); result.Error != nil {
		return result.Error
	}

	return nil
}

// ProcessImage creates an Image for an OrgID on Image Builder and on our database
func (s *ImageService) ProcessImage(ctx context.Context, img *models.Image) error {

	go s.processImage(ctx, img.ID)

	return nil
}

// ValidateImagePackage validate package name on Image Builder
func (s *ImageService) ValidateImagePackage(packageName string, image *models.Image) error {
	arch := image.Commit.Arch
	dist := image.Distribution
	if arch == "" || dist == "" {
		return errors.NewBadRequest("value is not one of the allowed values")
	}
	res, err := s.ImageBuilder.SearchPackage(packageName, arch, dist)
	if err != nil {
		return err
	}
	if res.Meta.Count == 0 {
		return new(PackageNameDoesNotExist)
	}
	for _, pkg := range res.Data {
		if pkg.Name == packageName {
			return nil
		}
	}
	return new(PackageNameDoesNotExist)
}

// getLatestPreviousSuccessfulImage return the latest previously successfully built image from the given image
// if no image found return nil
func (s *ImageService) getLatestPreviousSuccessfulImage(image *models.Image) (*models.Image, error) {
	var previousSuccessfulImage models.Image
	if result := db.Org(image.OrgID, "images").
		Where(models.Image{ImageSetID: image.ImageSetID, Status: models.ImageStatusSuccess}).
		Preload("Commit.Repo").Joins("Commit").
		Where("images.created_at < ?", image.CreatedAt).
		Order("images.created_at DESC").
		First(&previousSuccessfulImage); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// no previous image was successfully built
			s.log.WithField("image_id", image.ID).Info("latest successful previous image build was not found")
			return nil, nil
		}
		return nil, result.Error
	}
	return &previousSuccessfulImage, nil
}

// UpdateImage updates an image, adding a new version of this image to an imageset
func (s *ImageService) UpdateImage(image *models.Image, previousImage *models.Image) error {
	s.log.Info("Updating image...")
	if previousImage == nil {
		return new(ImageNotFoundError)
	}
	err := s.CheckIfIsLatestVersion(previousImage)
	if err != nil {
		return errors.NewBadRequest("only the latest updated image can be modified")
	}
	packages := image.Packages
	for _, p := range packages {
		er := s.ValidateImagePackage(p.Name, image)
		if er != nil {
			return er
		}
	}
	imagesrepos, err := GetImageReposFromDB(previousImage.OrgID, image.ThirdPartyRepositories)
	if err != nil {
		return err
	}
	image.ThirdPartyRepositories = *imagesrepos

	// important: update the image imageSet for any previous image build status,
	// otherwise image will be orphaned from its imageSet if previous build failed
	image.ImageSetID = previousImage.ImageSetID
	image.OrgID = previousImage.OrgID
	if image.Name == "" {
		// set the name to previous image name, as it must be obvious that it should be the same.
		image.Name = previousImage.Name
	} else if image.Name != previousImage.Name {
		// as we are updating image version, do not allow image to change its name, as the name may be used as image ref.
		return new(ImageNameChangeIsProhibited)
	}

	var currentImageSet models.ImageSet
	result := db.DB.Where("Id = ?", previousImage.ImageSetID).First(&currentImageSet)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error retrieving the image set from parent image")
		return result.Error
	}
	currentImageSet.Version = previousImage.Version + 1
	if err := db.DB.Save(currentImageSet).Error; err != nil {
		return err
	}

	var refs string
	if image.Distribution == "" {
		refs = config.DistributionsRefs[config.DefaultDistribution]
	} else {
		refs = config.DistributionsRefs[image.Distribution]
	}

	if image.Commit.OSTreeRef == "" {
		image.Commit.OSTreeRef = refs
	}

	var previousSuccessfulImage *models.Image
	if previousImage.Status == models.ImageStatusSuccess {
		previousSuccessfulImage = previousImage
	} else {
		// we need to get the latest previously successfully built image
		previousSuccessfulImage, err = s.getLatestPreviousSuccessfulImage(previousImage)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error getting the latest successful built image from previous image")
			return err
		}
	}
	if previousSuccessfulImage != nil {
		if feature.StorageImagesRepos.IsEnabled() {
			image.Commit.OSTreeParentCommit = fmt.Sprintf(
				"%s/api/edge/v1/storage/images-repos/%d",
				config.Get().EdgeCertAPIBaseURL,
				previousSuccessfulImage.ID,
			)
		} else {
			// Always get the repo URL from the previous successful Image's commit
			repo, err := s.RepoService.GetRepoByID(previousSuccessfulImage.Commit.RepoID)
			if err != nil {
				s.log.WithField("error", err.Error()).Error("Commit repo wasn't found on the database")
				err := errors.NewBadRequest(fmt.Sprintf("Commit repo wasn't found in the database: #%v", image.Commit.ID))
				return err
			}
			image.Commit.OSTreeParentCommit = repo.URL
		}

		if config.DistributionsRefs[previousSuccessfulImage.Distribution] != config.DistributionsRefs[image.Distribution] {
			image.Commit.ChangesRefs = true
		}

		if image.Commit.OSTreeParentRef == "" {
			image.Commit.OSTreeParentRef = config.DistributionsRefs[previousSuccessfulImage.Distribution]
		}
	}
	image, err = s.ImageBuilder.ComposeCommit(image)
	if err != nil {
		return err
	}
	image.Commit.OrgID = previousImage.OrgID
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	// TODO: Remove code when frontend is not using ImageType on the table
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.ImageType = models.ImageTypeInstaller
	} else {
		image.ImageType = models.ImageTypeCommit
	}

	if image.Installer != nil {
		image.Installer.OrgID = previousImage.OrgID
	}

	// TODO: End of remove block
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.Installer.Status = models.ImageStatusPending
		image.Installer.OrgID = image.OrgID
	}

	if result := db.DB.Create(&image); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error creating image")
		return result.Error
	}

	s.log = s.log.WithFields(log.Fields{"updatedImageID": image.ID, "updatedCommitID": image.Commit.ID})

	s.log.Info("Image Updated successfully - starting building process")

	return nil
}

func (s *ImageService) processInstaller(ctx context.Context, image *models.Image) error {
	s.log.Debug("Post processing the installer for the image")
	for {
		i, err := s.UpdateImageStatus(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Update image status error")
			return err
		}

		// the Image Builder status has changed from BUILDING
		if i.Installer.Status != models.ImageStatusBuilding {
			// TODO: re-add an event producer to notify consumers of installer completion
			break
		}
		time.Sleep(1 * time.Minute)
	}

	if image.Installer.Status == models.ImageStatusSuccess {
		// Post process the installer ISO
		//	User, kickstart, checksum, etc.
		if feature.ImageCreateISOEDA.IsEnabled() {
			s.log.Debug("Creating image iso with EDA")

			ctx := context.Background()
			ident, errident := common.GetIdentityFromContext(ctx)
			if errident != nil {
				s.log.WithField("error", errident.Error()).Error("Failed retrieving identity from context")
				return errident
			}
			// create payload for ImageISORequested event
			edgePayload := &models.EdgeImageISORequestedEventPayload{
				EdgeBasePayload: models.EdgeBasePayload{
					Identity:       ident,
					LastHandleTime: time.Now().Format(time.RFC3339),
					RequestID:      image.RequestID,
				},
				NewImage: *image,
			}

			// create the edge event
			edgeEvent := kafkacommon.CreateEdgeEvent(ident.Identity.OrgID, models.SourceEdgeEventAPI, image.RequestID,
				models.EventTypeEdgeImageISORequested, image.Name, edgePayload)

			// put the event on the bus
			if err := kafkacommon.NewProducerService().ProduceEvent(kafkacommon.TopicFleetmgmtImageBuild, models.EventTypeEdgeImageISORequested, edgeEvent); err != nil {
				log.WithField("request_id", edgeEvent.ID).Error("Producing the event failed")
				return err
			}
		} else {
			err := s.AddUserInfo(image)
			if err != nil {
				s.log.WithField("error", err.Error()).Error("Kickstart file injection failed")
				return err
			}
		}
	}
	// Regardless of the status, call this method to make sure the status will be updated
	// It updates the status across the image and not just the installer
	s.log.Debug("Setting final image status")
	s.SetFinalImageStatus(image)

	// send an installer completed event
	if feature.ImageCompletionEventsEDA.IsEnabled() {
		s.log.Debug("Installer completed (EDA)")

		// get the identity from the context
		ident, err := common.GetIdentityInstanceFromContext(ctx)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error getting identity from context")
		}

		// create payload for event
		edgePayload := &models.EdgeInstallerCompletedEventPayload{
			EdgeBasePayload: models.EdgeBasePayload{
				Identity:       ident,
				LastHandleTime: time.Now().Format(time.RFC3339),
				RequestID:      image.RequestID,
			},
			NewImage: *image,
		}

		// create the edge event
		edgeEvent := kafkacommon.CreateEdgeEvent(ident.Identity.OrgID, models.SourceEdgeEventAPI, image.RequestID,
			models.EventTypeEdgeInstallerCompleted, image.Name, edgePayload)

		// put the event on the bus
		if err := kafkacommon.NewProducerService().ProduceEvent(kafkacommon.TopicFleetmgmtImageBuild, models.EventTypeEdgeInstallerCompleted, edgeEvent); err != nil {
			log.WithField("request_id", edgeEvent.ID).Error("Producing the event failed")

			return err
		}
	}

	s.log.WithField("status", image.Status).Debug("Processing image installer is done")
	return nil
}

func (s *ImageService) processCommit(ctx context.Context, image *models.Image) error {
	s.log.Debug("Processing image build commit")
	for {
		i, err := s.UpdateImageStatus(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Update image status error")
			return err
		}
		if i.Commit.Status != models.ImageStatusBuilding {
			// TODO: re-add an event producer to notify consumers of installer completion
			break
		}
		time.Sleep(1 * time.Minute)
	}

	if image.Commit.Status == models.ImageStatusSuccess {
		imageWithMetaData, err := s.ImageBuilder.GetMetadata(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Failed getting metadata from image builder")
			s.SetErrorStatusOnImage(err, imageWithMetaData)
			return err
		}

		// send a commit completed event
		if feature.ImageCompletionEventsEDA.IsEnabled() {

			// get the identity from the context
			ident, err := common.GetIdentityInstanceFromContext(ctx)
			if err != nil {
				s.log.WithField("error", err.Error()).Error("Error getting identity from context")

				return err
			}

			// create payload for event
			edgePayload := &models.EdgeCommitCompletedEventPayload{
				EdgeBasePayload: models.EdgeBasePayload{
					Identity:       ident,
					LastHandleTime: time.Now().Format(time.RFC3339),
					RequestID:      image.RequestID,
				},
				NewImage: *image,
			}

			// create the edge event
			edgeEvent := kafkacommon.CreateEdgeEvent(ident.Identity.OrgID, models.SourceEdgeEventAPI, image.RequestID,
				models.EventTypeEdgeCommitCompleted, image.Name, edgePayload)

			// put the event on the bus
			if err := kafkacommon.NewProducerService().ProduceEvent(kafkacommon.TopicFleetmgmtImageBuild, models.EventTypeEdgeCommitCompleted, edgeEvent); err != nil {
				s.log.WithField("request_id", edgeEvent.ID).Error("Producing the event failed")

				return err
			}
		}

		// Create the repo for the image
		_, err = s.CreateRepoForImage(ctx, image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Failed creating repo for image")
			return err
		}
	}
	if !image.HasOutputType(models.ImageTypeInstaller) {
		image.Installer = nil
		s.log.Debug("Setting final image status - no installer to create")
		s.SetFinalImageStatus(image)
		s.log.Debug("Processing image is done - no installer to create")
	}

	s.log.Debug("Processing commit is done")
	return nil
}

// SetFinalImageStatus sets the final image status
func (s *ImageService) SetFinalImageStatus(i *models.Image) {
	// image status can be success if all output types are successful
	// if any status are not final (success/error) then sets to error
	// image status is error if any output status is error
	success := true
	for _, out := range i.OutputTypes {
		if out == models.ImageTypeCommit {
			if i.Commit == nil || i.Commit.Status != models.ImageStatusSuccess {
				success = false
			}
			if i.Commit.Status == models.ImageStatusBuilding {
				success = false
				i.Commit.Status = models.ImageStatusError
				db.DB.Save(i.Commit)
			}
		}
		if out == models.ImageTypeInstaller {
			if i.Installer == nil || i.Installer.Status != models.ImageStatusSuccess {
				success = false
			}
			if i.Installer.Status == models.ImageStatusBuilding {
				success = false
				i.Installer.Status = models.ImageStatusError
				db.DB.Save(i.Installer)
			}
		}
	}

	if success {
		i.Status = models.ImageStatusSuccess
	} else {
		i.Status = models.ImageStatusError
	}

	tx := db.DB.Save(i)
	if tx.Error != nil {
		s.log.WithField("error", tx.Error.Error()).Error("Couldn't set final image status")
	}
	s.log.WithField("status", i.Status).Debug("Setting final image status")

	if i.ImageSetID != nil && i.Status == models.ImageStatusSuccess {
		if err := s.SetDevicesUpdateAvailabilityFromImageSet(i.OrgID, *i.ImageSetID); err != nil {
			s.log.WithField("error", err.Error()).Error("Error while setting devices update availability flag")
		}
	}
}

func (s *ImageService) processImage(ctx context.Context, id uint) {
	s.log.Debug("Processing image build")
	var image *models.Image

	// setup a context and signal for SIGTERM
	intctx, intcancel := context.WithCancel(context.Background())
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)

	// this will run at the end of postProcessImage to tidy up signal and context
	defer func() {
		s.log.WithField("imageID", id).Debug("Stopping the interrupt context and sigint signal")
		signal.Stop(sigint)
		intcancel()
	}()
	// This runs alongside and blocks on either a signal or normal completion from defer above
	// 	if an interrupt, set image to INTERRUPTED in database
	go func() {
		s.log.WithField("imageID", id).Debug("Running the select go routine to handle completion and interrupts")

		select {
		case <-sigint:
			// we caught an interrupt. Mark the image as interrupted.
			s.log.WithField("imageID", id).Debug("Select case SIGINT interrupt has been triggered")

			tx := db.DB.Model(&models.Image{}).Where("ID = ?", id).Update("Status", models.ImageStatusInterrupted)
			s.log.WithField("imageID", id).Debug("Image updated with interrupted status")
			if tx.Error != nil {
				s.log.WithField("error", tx.Error.Error()).Error("Error updating image")
			}

			// cancel the context
			intcancel()
			return
		case <-intctx.Done():
			// Things finished normally and reached the defer defined above.
			s.log.WithField("imageID", id).Info("Select case context intctx done has been triggered")
		}
	}()

	// business as usual from here to end of block
	db.DB.Joins("Commit").Joins("Installer").First(&image, id)

	// Monitor the commit for completion
	s.log.WithField("imageID", image.ID).Debug("Monitoring commit status for this image")
	err := s.processCommit(ctx, image)
	if err != nil {
		if image.Status == models.ImageStatusInterrupted {
			return
		}
		s.SetErrorStatusOnImage(err, image)
		s.log.WithField("error", err.Error()).Error("Failed creating commit for image")
	}

	if image.Commit.Status == models.ImageStatusSuccess {
		s.log.Debug("Commit is successful")

		// Request an installer ISO from Image Builder for the image
		if image.HasOutputType(models.ImageTypeInstaller) {
			s.log.WithField("imageID", image.ID).Debug("Creating an installer for this image")
			image, c, err := s.CreateInstallerForImage(ctx, image)
			/* CreateInstallerForImage is also called directly from an endpoint.
			If called from the endpoint it will not block
				the caller returns the channel output to _
			Here, we catch the channel with c and use it in the next if--so it blocks.
			*/
			if c != nil {
				err = <-c
			}
			if image.Status == models.ImageStatusInterrupted {
				return
			}
			if err != nil {
				s.SetErrorStatusOnImage(err, image)
				s.log.WithField("error", err.Error()).Error("Failed creating installer for image")
			}
		}
	}
	s.log.WithField("status", image.Status).Debug("Processing image build is done")
}

// CreateRepoForImage creates the OSTree repo to host that image
func (s *ImageService) CreateRepoForImage(ctx context.Context, img *models.Image) (*models.Repo, error) {
	s.log.Info("Creating OSTree repo for image")
	repo := &models.Repo{
		Status: models.RepoStatusBuilding,
	}
	tx := db.DB.Create(repo)
	if tx.Error != nil {
		return nil, tx.Error
	}
	s.log = s.log.WithField("repoID", repo.ID)
	s.log.Debug("OSTree repo is created on the database")

	img.Commit.Repo = repo
	img.Commit.RepoID = &repo.ID

	tx = db.DB.Save(img.Commit)
	if tx.Error != nil {
		return nil, tx.Error
	}
	s.log.Debug("OSTree repo was saved to commit")

	repo, err := s.RepoBuilder.ImportRepo(repo)
	if err != nil {
		return nil, err
	}

	// send a commit completed event
	if feature.ImageCompletionEventsEDA.IsEnabled() {
		s.log.Debug("Repo completed (EDA)")

		// get the identity from the context
		ident, err := common.GetIdentityInstanceFromContext(ctx)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error getting identity from context")

			return nil, err
		}

		// create payload for event
		edgePayload := &models.EdgeOstreeRepoCompletedEventPayload{
			EdgeBasePayload: models.EdgeBasePayload{
				Identity:       ident,
				LastHandleTime: time.Now().Format(time.RFC3339),
				RequestID:      img.RequestID,
			},
			NewImage: *img,
		}

		// create the edge event
		edgeEvent := kafkacommon.CreateEdgeEvent(ident.Identity.OrgID, models.SourceEdgeEventAPI,
			img.RequestID, models.EventTypeEdgeOstreeRepoCompleted, img.Name, edgePayload)

		// put the event on the bus
		if err := kafkacommon.NewProducerService().ProduceEvent(kafkacommon.TopicFleetmgmtImageBuild, models.EventTypeEdgeOstreeRepoCompleted, edgeEvent); err != nil {
			log.WithField("request_id", edgeEvent.ID).Error("Producing the event failed")

			return nil, err
		}
	}

	s.log.Info("OSTree repo is ready")

	return repo, nil
}

// SetErrorStatusOnImage is a helper function that sets the error status on images
func (s *ImageService) SetErrorStatusOnImage(err error, image *models.Image) {
	if image.Status != models.ImageStatusError {
		image.Status = models.ImageStatusError
		s.setImageStatus(image, models.ImageStatusError)

		if image.Commit != nil {
			image.Commit.Status = models.ImageStatusError
			s.setCommitStatus(image, models.ImageStatusError)
		}

		if image.Installer != nil {
			image.Installer.Status = models.ImageStatusError
			s.setInstallerStatus(image, models.ImageStatusError)
		}
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Error setting image final status")
		}
	}
}

// AddUserInfo downloads the ISO
// injects the kickstart with username and ssh key
// and then re-uploads the ISO into our bucket
func (s *ImageService) AddUserInfo(image *models.Image) error {
	// Absolute path for manipulating ISO's
	destPath := "/var/tmp/"

	downloadURL := image.Installer.ImageBuildISOURL
	sshKey := image.Installer.SSHKey
	username := image.Installer.Username
	// Files that will be used to modify the ISO and will be cleaned
	imageName := destPath + image.Name
	kickstart := fmt.Sprintf("%sfinalKickstart-%s_%d.ks", destPath, image.OrgID, image.ID)
	// TODO: in the future, org_id will be used as seen below:
	// kickstart := fmt.Sprintf("%sfinalKickstart-%s_%d.ks", destPath, image.OrgID, image.ID)

	err := s.downloadISO(imageName, downloadURL)
	if err != nil {
		return fmt.Errorf("error downloading ISO file :: %s", err.Error())
	}

	s.log.Debug("Adding SSH Key to kickstart file...")
	err = s.addSSHKeyToKickstart(sshKey, username, kickstart)
	if err != nil {
		return fmt.Errorf("error adding ssh key to kickstart file :: %s", err.Error())
	}

	s.log.Debug("Injecting the kickstart into image...")
	err = s.exeInjectionScript(kickstart, imageName, image.ID)
	if err != nil {
		return fmt.Errorf("error executing fleetkick script :: %s", err.Error())
	}

	s.log.Debug("Calculating the checksum for the ISO image...")
	err = s.calculateChecksum(imageName, image)
	if err != nil {
		return fmt.Errorf("error calculating checksum for ISO :: %s", err.Error())
	}

	err = s.uploadISO(image, imageName)
	if err != nil {
		return fmt.Errorf("error uploading ISO :: %s", err.Error())
	}

	s.log.Debug("Cleaning up temporary files...")
	err = s.cleanFiles(kickstart, imageName, image.ID)
	if err != nil {
		return fmt.Errorf("error cleaning files :: %s", err.Error())
	}

	s.log.Debug("Post installer ISO processing complete")
	return nil
}

// UnameSSH is the template struct for username and ssh key
type UnameSSH struct {
	Sshkey   string
	Username string
}

// Adds user provided ssh key to the kickstart file.
func (s *ImageService) addSSHKeyToKickstart(sshKey string, username string, kickstart string) error {
	cfg := config.Get()

	td := UnameSSH{sshKey, username}

	s.log.WithField("templatesPath", cfg.TemplatesPath).Debug("Opening file")
	t, err := template.ParseFiles(cfg.TemplatesPath + "templateKickstart.ks")
	if err != nil {
		return err
	}

	s.log.WithField("kickstart", kickstart).Debug("Creating file")
	file, err := os.Create(kickstart)
	if err != nil {
		return err
	}

	s.log.WithFields(log.Fields{
		"username": username,
		"sshKey":   sshKey,
	}).Debug("Injecting username and key into template")
	err = t.Execute(file, td)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Failed adding username and sshkey on image")
		return err
	}
	if err := file.Close(); err != nil {
		s.log.WithField("error", err.Error()).Error("Failed closing file")
		return err
	}

	return nil
}

// Download created ISO into the file system.
func (s *ImageService) downloadISO(isoName string, url string) error {

	s.log.WithField("isoName", isoName).Debug("Creating ISO file")
	iso, err := os.Create(isoName)
	if err != nil {
		return err
	}
	defer func() {
		if err := iso.Close(); err != nil {
			s.log.WithField("error", err.Error()).Error("Error closing file")
		}
	}()

	s.log.WithField("url", url).Debug("Downloading ISO...")
	res, err := http.Get(url) // #nosec G107
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = io.Copy(iso, res.Body)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Failed downloading ISO")
		return err
	}

	return nil
}

// Upload finished ISO to S3
func (s *ImageService) uploadISO(image *models.Image, imageName string) error {

	uploadPath := fmt.Sprintf("%s/isos/%s.iso", image.OrgID, image.Name)
	s.log.WithField("path", uploadPath).Debug("Uploading ISO...")
	filesService := NewFilesService(s.log)
	url, err := filesService.GetUploader().UploadFile(imageName, uploadPath)

	if err != nil {
		return fmt.Errorf("error uploading the ISO :: %s :: %s", uploadPath, err.Error())
	}

	image.Installer.ImageBuildISOURL = url
	tx := db.DB.Save(&image.Installer)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

// Remove edited kickstart after use.
func (s *ImageService) cleanFiles(kickstart string, isoName string, imageID uint) error {
	err := os.Remove(kickstart)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error removing kickstart file")
		return err
	}
	s.log.WithField("kickstart", kickstart).Debug("Kickstart file removed")

	err = os.Remove(isoName)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error removing tmp iso")
		return err
	}
	s.log.WithField("isoName", isoName).Debug("ISO file removed")

	workDir := fmt.Sprintf("/var/tmp/workdir%d", imageID)
	err = os.RemoveAll(workDir)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error removing work dir path")
		return err
	}
	s.log.WithField("workDir", workDir).Debug("Work dir path removed")

	return nil
}

// UpdateImageStatus updates the status of an commit and/or installer based on Image Builder's status
func (s *ImageService) UpdateImageStatus(image *models.Image) (*models.Image, error) {
	if image.Commit.Status == models.ImageStatusBuilding {
		image, err := s.ImageBuilder.GetCommitStatus(image)
		if err != nil {
			// check that if error contain timeout and job stop responding and image's time creation is less than 3 hours
			if strings.Contains(err.Error(), "running this job stopped responding") {
				image.Status = models.ImageStatusInterrupted
				tx := db.DB.Model(&models.Image{}).Where("ID = ?", image.ID).Update("Status", models.ImageStatusInterrupted)
				if tx.Error != nil {
					return image, err
				}
				return image, err
			}
			return image, err
		}
		if image.Commit.Status != models.ImageStatusBuilding {
			tx := db.DB.Save(&image.Commit)
			if tx.Error != nil {
				return image, tx.Error
			}
		}
	}
	if image.Installer != nil && image.Installer.Status == models.ImageStatusBuilding {
		image, err := s.ImageBuilder.GetInstallerStatus(image)
		if err != nil {
			return image, err
		}
		if image.Installer.Status != models.ImageStatusBuilding {
			tx := db.DB.Save(&image.Installer)
			if tx.Error != nil {
				return image, tx.Error
			}
		}
	}
	if image.Status != models.ImageStatusBuilding {
		tx := db.DB.Save(&image)
		if tx.Error != nil {
			return image, tx.Error
		}
	}
	return image, nil
}

// CheckImageName returns false if the image does not exist and true if the image exists
func (s *ImageService) CheckImageName(name, orgID string) (bool, error) {
	var imageFindByName *models.Image
	// Search for an organization with an image with specific name
	result := db.Org(orgID, "").Where("(name = ?)", name).First(&imageFindByName)
	// If we get an error from the query
	if result.Error != nil {
		// If no records were found
		if result.Error == gorm.ErrRecordNotFound {
			// Return false but don't return the error
			return false, nil
		}
		// If a different type of error happens, return it
		return false, result.Error
	}
	return imageFindByName != nil, nil
}

// Inject the custom kickstart into the iso via script.
func (s *ImageService) exeInjectionScript(kickstart string, image string, imageID uint) error {
	fleetBashScript := "/usr/local/bin/fleetkick.sh"
	workDir := fmt.Sprintf("/var/tmp/workdir%d", imageID)
	err := os.Mkdir(workDir, 0750)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error giving permissions to execute fleetkick")
		return err
	}

	cmd := &exec.Cmd{
		Path: fleetBashScript,
		Args: []string{
			fleetBashScript, kickstart, image, image, workDir,
		},
	}
	output, err := cmd.Output()
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error executing fleetkick")
		return err
	}
	s.log.WithField("output", output).Info("Fleetkick Output")
	return nil
}

// Calculate the checksum of the final ISO.
func (s *ImageService) calculateChecksum(isoPath string, image *models.Image) error {
	s.log.WithField("isoPath", isoPath).Info("Calculating sha256 checksum for ISO")

	fh, err := os.Open(filepath.Clean(isoPath))
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error opening ISO file")
		return err
	}
	defer func() {
		if err := fh.Close(); err != nil {
			s.log.WithField("error", err.Error()).Error("Error closing file")
		}
	}()

	sumCalculator := sha256.New()
	_, err = io.Copy(sumCalculator, fh)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error calculating sha256 checksum for ISO")
		return err
	}

	image.Installer.Checksum = hex.EncodeToString(sumCalculator.Sum(nil))
	s.log.WithField("checksum", image.Installer.Checksum).Info("Checksum calculated")
	tx := db.DB.Save(&image.Installer)
	if tx.Error != nil {
		s.log.WithField("error", tx.Error.Error()).Error("Error saving installer")
		return tx.Error
	}

	return nil
}

// ImageDetail return the structure to inform package info to images
// nolint: unused // golangci-lint does not parse well this block, it says Image not used, but it's used
type ImageDetail struct {
	Image              *models.Image `json:"image"`
	AdditionalPackages int           `json:"additional_packages"`
	Packages           int           `json:"packages"`
	UpdateAdded        int           `json:"update_added"`
	UpdateRemoved      int           `json:"update_removed"`
	UpdateUpdated      int           `json:"update_updated"`
}

// AddPackageInfo return info related to packages on image
func (s *ImageService) AddPackageInfo(image *models.Image) (ImageDetail, error) {
	var imgDetail ImageDetail
	imgDetail.Image = image
	imgDetail.Packages = len(image.Commit.InstalledPackages)
	imgDetail.AdditionalPackages = len(image.Packages)

	upd, err := s.GetUpdateInfo(*image)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting update info")
		return imgDetail, err
	}
	if upd != nil {
		imgDetail.UpdateAdded = len(upd.PackageDiff.Removed)
		imgDetail.UpdateRemoved = len(upd.PackageDiff.Added)
		imgDetail.UpdateUpdated = len(upd.PackageDiff.Upgraded)
	} else {
		imgDetail.UpdateAdded = 0
		imgDetail.UpdateRemoved = 0
		imgDetail.UpdateUpdated = 0
	}
	return imgDetail, nil
}

func (s *ImageService) addImageExtraData(image *models.Image) (*models.Image, error) {
	if image.InstallerID != nil {
		result := db.DB.First(&image.Installer, image.InstallerID)
		if result.Error != nil {
			s.log.WithField("error", result.Error).Error("Error retrieving installer for image")
			return nil, result.Error
		}
	}
	err := db.DB.Model(image).Association("Packages").Find(&image.Packages)
	if err != nil {
		s.log.WithField("error", err).Error("Error packages from image")
		return nil, err
	}
	return image, nil
}

// GetImageByID retrieves an image by its identifier
func (s *ImageService) GetImageByID(imageID string) (*models.Image, error) {
	var image models.Image
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err).Error("Error retrieving org_id")
		return nil, new(OrgIDNotSet)
	}
	id, err := strconv.Atoi(imageID)
	if err != nil {
		s.log.WithField("error", err).Debug("Request related error - ID is not integer")
		return nil, new(IDMustBeInteger)
	}
	result := db.Org(orgID, "images").Preload("Commit.Repo").Preload("Commit.InstalledPackages").Preload("CustomPackages").Preload("ThirdPartyRepositories").Joins("Commit").First(&image, id)
	if result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Debug("Request related error - image is not found")
		return nil, new(ImageNotFoundError)
	}
	return s.addImageExtraData(&image)
}

// GetImageByOSTreeCommitHash retrieves an image by its ostree commit hash
func (s *ImageService) GetImageByOSTreeCommitHash(commitHash string) (*models.Image, error) {
	s.log.WithField("ostreeHash", commitHash).Info("Getting image by OSTreeHash")
	var image models.Image
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.Error("Error retrieving org_id")
		return nil, new(OrgIDNotSet)
	}
	result := db.Org(orgID, "images").Joins("JOIN commits ON commits.id = images.commit_id AND commits.os_tree_commit = ?", commitHash).Joins("Installer").Preload("Packages").Preload("Commit.InstalledPackages").Preload("Commit.Repo").First(&image)
	if result.Error != nil {
		s.log.WithField("error", result.Error).Error("Error retrieving image by OSTreeHash")
		return nil, new(ImageNotFoundError)
	}
	s.log = s.log.WithField("imageID", image.ID)
	s.log.Info("Image successfully retrieved by its OSTreeHash")
	return &image, nil
}

// RetryCreateImage retries the whole post process of the image creation
func (s *ImageService) RetryCreateImage(ctx context.Context, image *models.Image) error {
	s.log = s.log.WithFields(log.Fields{"imageID": image.ID, "commitID": image.Commit.ID})
	// recompose commit
	image, err := s.ImageBuilder.ComposeCommit(image)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Failed recomposing commit")
		return err
	}
	err = s.SetBuildingStatusOnImageToRetryBuild(image)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Failed setting image status")
		return nil
	}
	go s.processImage(ctx, image.ID)
	return nil
}

func (s *ImageService) setImageStatus(image *models.Image, status string) error {
	image.Status = status
	tx := db.DB.Save(image)
	if tx.Error != nil {
		s.log.WithFields(log.Fields{"imageID": image.ID, "status": status, "error": tx.Error.Error()}).Error("Failed to update image status")
		return tx.Error
	}
	s.log.WithFields(log.Fields{"imageID": image.ID, "status": status}).Info("Updated image status")

	return nil
}

func (s *ImageService) setCommitStatus(image *models.Image, status string) error {
	image.Commit.Status = status
	tx := db.DB.Save(image.Commit)
	if tx.Error != nil {
		s.log.WithFields(log.Fields{"imageID": image.ID, "commitID": image.Commit.ID, "status": status, "error": tx.Error.Error()}).Error("Failed to update commit status")
		return tx.Error
	}
	s.log.WithFields(log.Fields{"imageID": image.ID, "commitID": image.Commit.ID, "status": status}).Info("Updated commit status")

	return nil
}

func (s *ImageService) setInstallerStatus(image *models.Image, status string) error {
	image.Installer.Status = status
	tx := db.DB.Save(image.Installer)
	if tx.Error != nil {
		s.log.WithFields(log.Fields{"imageID": image.ID, "installerID": image.Installer.ID, "status": status, "error": tx.Error.Error()}).Error("Failed to update installer status")
		return tx.Error
	}
	s.log.WithFields(log.Fields{"imageID": image.ID, "installerID": image.Installer.ID, "status": status}).Info("Updated installer status")

	return nil
}

// ResumeCreateImage retries the whole post process of the image creation
func (s *ImageService) ResumeCreateImage(ctx context.Context, image *models.Image) error {
	// add additional information to all log entries in this pipeline
	s.log = s.log.WithField("originalRequestId", image.RequestID)
	s.log = s.log.WithField("imageID", image.ID)

	// changing status from INTERRUPTED to BUILDING here so image is updated in return to API call
	err := s.setImageStatus(image, models.ImageStatusBuilding)
	if err != nil {
		return err
	}

	// go routine so we can return to the API caller ASAP
	go s.resumeProcessImage(ctx, image)

	return nil
}

func (s *ImageService) resumeProcessImage(ctx context.Context, image *models.Image) {
	// NOTE: Every log message in this method already has commit id and image id injected
	s.log.Debug("Processing image build from where it was interrupted")

	id := image.ID

	// setup a context and signal for SIGTERM
	intctx, intcancel := context.WithCancel(context.Background())
	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)

	// this will run at the end of postProcessImage to tidy up signal and context
	defer func() {
		s.log.WithField("imageID", id).Debug("Stopping the interrupt context and sigint signal")
		signal.Stop(sigint)
		intcancel()
	}()
	// This runs alongside and blocks on either a signal or normal completion from defer above
	// 	if an interrupt, set image to INTERRUPTED in database
	go func() {
		s.log.WithField("imageID", id).Debug("Running the select go routine to handle completion and interrupts")

		select {
		case <-sigint:
			// we caught an interrupt. Mark the image as interrupted.
			s.log.WithField("imageID", id).Debug("Select case SIGINT interrupt has been triggered")

			tx := db.DB.Model(&models.Image{}).Where("ID = ?", id).Update("Status", models.ImageStatusInterrupted)
			s.log.WithField("imageID", id).Debug("Image updated with interrupted status")
			if tx.Error != nil {
				s.log.WithField("error", tx.Error.Error()).Error("Error updating image")
			}

			// cancel the context
			intcancel()
			return
		case <-intctx.Done():
			// Things finished normally and reached the defer defined above.
			s.log.WithField("imageID", id).Info("Select case context intctx done has been triggered")
		}
	}()

	/* business as usual from here to end of block */

	// skip the commit if status is already set to success
	if image.Commit.Status != models.ImageStatusSuccess {
		// Request a commit from Image Builder for the image
		s.log.Debug("Creating a commit for this image")
		err := s.processCommit(ctx, image)
		if err != nil {
			if image.Status == models.ImageStatusInterrupted {
				return
			}
			s.SetErrorStatusOnImage(err, image)
			s.log.WithField("error", err.Error()).Error("Failed creating commit for image")
		}
	}

	// REFACTOR: break postProcessImage, postProcessCommit, and postProcessInstaller into functional units

	// if we get this far, request an installer
	if image.Commit.Status == models.ImageStatusSuccess {
		/*image, err := s.ImageBuilder.GetMetadata(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Failed getting metadata from image builder")
			s.SetErrorStatusOnImage(err, image)
			return
		} */

		// Create the repo for the image
		if image.Commit.Repo.Status != models.RepoStatusSuccess {
			_, err := s.CreateRepoForImage(ctx, image)
			if err != nil {
				s.log.WithField("error", err.Error()).Error("Failed creating repo for image")
				return
			}
		}

		if !image.HasOutputType(models.ImageTypeInstaller) {
			image.Installer = nil
			s.log.Debug("Setting final image status - no installer to create")
			s.SetFinalImageStatus(image)
			s.log.Debug("Processing image is done - no installer to create")

			return
		}
		// FIXME: revisit this logging
		s.log.Debug("Processing commit is done")
		s.log.Debug("Commit is successful")

		// Request an installer ISO from Image Builder for the image
		// skip if already set to success
		if image.HasOutputType(models.ImageTypeInstaller) && image.Installer.Status != models.ImageStatusSuccess {
			s.log.WithField("imageID", image.ID).Debug("Creating an installer for this image")
			image2, c, err := s.CreateInstallerForImage(ctx, image)
			/* CreateInstallerForImage is also called directly from an endpoint.
			If called from the endpoint it will not block
				the caller returns the channel output to _
			Here, we catch the channel with c and use it in the next if--so it blocks.
			*/
			if c != nil {
				err = <-c
			}
			if err != nil {
				if image.Status == models.ImageStatusInterrupted {
					return
				}
				s.SetErrorStatusOnImage(err, image2)
				s.log.WithField("error", err.Error()).Error("Failed creating installer for image")
			}
		}
	}
	s.log.WithField("status", image.Status).Debug("Processing resumed image build is done")
}

// SetBuildingStatusOnImageToRetryBuild set building status on image so we can try the build
func (s *ImageService) SetBuildingStatusOnImageToRetryBuild(image *models.Image) error {
	s.log.Debug("Setting image status")
	image.Status = models.ImageStatusBuilding
	if image.Commit != nil {
		s.log.Debug("Setting commit status")
		image.Commit.Status = models.ImageStatusBuilding
		// Repo will be recreated from scratch, its safer and simpler as this stage
		if image.Commit.Repo != nil {
			s.log.Debug("Reset repo")
			image.Commit.Repo = nil
		}
		err := s.setCommitStatus(image, models.ImageStatusBuilding)
		if err != nil {
			return err
		}
	}

	if image.Installer != nil {
		s.log.Debug("Setting installer status")
		image.Installer.Status = models.ImageStatusCreated
		err := s.setInstallerStatus(image, models.ImageStatusCreated)
		if err != nil {
			return err
		}
	}

	err := s.setImageStatus(image, models.ImageStatusBuilding)
	if err != nil {
		return err
	}

	return nil
}

// CheckIfIsLatestVersion make sure that there is no same image version present
func (s *ImageService) CheckIfIsLatestVersion(previousImage *models.Image) error {
	/*
		previousImage is the latest version image,
		if when searching the latest version image in the context of the same org_id and imageSet,
		we found an image that is equal to previousImage
	*/
	if previousImage.OrgID == "" {
		return new(OrgIDNotSet)
	}

	if previousImage.ID == 0 {
		return new(ImageUnDefined)
	}

	if previousImage.ImageSetID == nil {
		return new(ImageSetUnDefined)
	}

	var latestImageVersion models.Image
	if result := db.Org(previousImage.OrgID, "").Where(models.Image{ImageSetID: previousImage.ImageSetID}).Order("version DESC").First(&latestImageVersion); result.Error != nil {
		return result.Error
	}

	if latestImageVersion.ID != previousImage.ID {
		return new(ImageVersionAlreadyExists)
	}

	return nil
}

// GetUpdateInfo return package info when has an update to the image
func (s *ImageService) GetUpdateInfo(image models.Image) (*models.ImageUpdateAvailable, error) {
	if image.Status != models.ImageStatusSuccess {
		// we get update info only for a successfully built image as this information make no sense for other statuses
		return nil, nil
	}
	var updateFromImage models.Image
	if result := db.DB.Where("Image_set_id = ? and Images.Status = ? and Images.Id < ?",
		image.ImageSetID, models.ImageStatusSuccess, image.ID).Joins("Commit").
		Order("Images.created_at DESC").First(&updateFromImage); result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// image is not updating from any image
			s.log.WithField("error", result.Error.Error()).Error("Error retrieving update")
			return nil, nil
		}
		return nil, result.Error
	}

	if result := db.DB.First(&updateFromImage.Commit, updateFromImage.CommitID); result.Error != nil {
		s.log.WithField("error", result.Error.Error()).Error("Error when retrieving updateFromImage commit")
		if result.Error == gorm.ErrRecordNotFound {
			return nil, new(ImageCommitNotFound)
		}
		return nil, result.Error
	}
	if err := db.DB.Model(&updateFromImage.Commit).Association("InstalledPackages").Find(&updateFromImage.Commit.InstalledPackages); err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving installed packages")
		return nil, err
	}
	if err := db.DB.Model(&updateFromImage).Association("Packages").Find(&updateFromImage.Packages); err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving updated packages")
		return nil, err
	}

	if err := db.DB.Model(&updateFromImage).Association("CustomPackages").Find(&updateFromImage.CustomPackages); err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving updated CustomPackages")
		return nil, err
	}
	var imageUpdateAvailable models.ImageUpdateAvailable
	imageDevicesCount, err := s.GetImageDevicesCount(image.ID)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Could not find device image info")
		return nil, err
	}
	diff := GetDiffOnUpdate(image, updateFromImage)
	updateFromImage.Commit.InstalledPackages = nil // otherwise the frontend will get the whole list of installed packages
	imageUpdateAvailable.Image = updateFromImage
	imageUpdateAvailable.PackageDiff = diff
	totalPackages := len(image.Commit.InstalledPackages)
	imageUpdateAvailable.Image.TotalPackages = totalPackages
	imageUpdateAvailable.Image.TotalDevicesWithImage = imageDevicesCount

	return &imageUpdateAvailable, nil
}

// GetMetadata return package info when has an update to the image
func (s *ImageService) GetMetadata(image *models.Image) (*models.Image, error) {
	s.log.Debug("Retrieving metadata")
	image, err := s.ImageBuilder.GetMetadata(image)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error retrieving metadata")
		return nil, err
	}
	s.log.Debug("Metadata retrieved successfully")
	return image, nil
}

// CreateInstallerForImage creates a installer given an existing image
func (s *ImageService) CreateInstallerForImage(ctx context.Context, image *models.Image) (*models.Image, chan error, error) {
	s.log.Debug("Creating installer for image")
	c := make(chan error)

	image.ImageType = models.ImageTypeInstaller
	image.Installer.Status = models.ImageStatusBuilding
	tx := db.DB.Save(&image)
	if tx.Error != nil {
		s.log.WithField("error", tx.Error.Error()).Error("Error saving image")
		return nil, c, tx.Error
	}
	tx = db.DB.Save(&image.Installer)
	if tx.Error != nil {
		s.log.WithField("error", tx.Error.Error()).Error("Error saving installer")
		return nil, c, tx.Error
	}
	image, err := s.ImageBuilder.ComposeInstaller(image)
	if err != nil {
		return nil, c, err
	}
	go func(chan error) {
		err := s.processInstaller(ctx, image)
		c <- err
	}(c)
	return image, c, nil
}

// GetRollbackImage returns the previous image from the image set in case of a rollback
func (s *ImageService) GetRollbackImage(image *models.Image) (*models.Image, error) {
	s.log.Info("Getting rollback image")
	var rollback models.Image
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.Error("Error retrieving org_id")
		return nil, new(OrgIDNotSet)
	}
	result := db.Org(orgID, "images").Debug().Joins("Commit").Joins("Installer").Preload("Packages").Preload("CustomPackages").Preload("ThirdPartyRepositories").Preload("Commit.InstalledPackages").Preload("Commit.Repo").Where(&models.Image{ImageSetID: image.ImageSetID, Status: models.ImageStatusSuccess}).Last(&rollback, "images.id < ?", image.ID)
	if result.Error != nil {
		s.log.WithField("error", result.Error).Error("Error retrieving rollback image")
		return nil, new(ImageNotFoundError)
	}
	rollback.TotalPackages = len(rollback.Commit.InstalledPackages)
	s.log = s.log.WithField("imageID", image.ID)
	s.log.Info("Rollback image successfully retrieved")
	return &rollback, nil
}

// SendImageNotification connects to platform.notifications.ingress on image topic
func (s *ImageService) SendImageNotification(i *models.Image) (ImageNotification, error) {
	s.log.WithField("message", i).Info("SendImageNotification::Starts")
	var notify ImageNotification
	notify.Version = NotificationConfigVersion
	notify.Bundle = NotificationConfigBundle
	notify.Application = NotificationConfigApplication
	notify.EventType = NotificationConfigEventTypeImage
	notify.Timestamp = time.Now().Format(time.RFC3339)

	if clowder.IsClowderEnabled() {
		var users []string
		var events []EventNotification
		var event EventNotification
		var recipients []RecipientNotification
		var recipient RecipientNotification
		brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))

		for i, b := range clowder.LoadedConfig.Kafka.Brokers {
			brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
			fmt.Println(brokers[i])
		}

		topic := NotificationTopic

		// Create Producer instance
		p, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers": brokers[0]})
		if err != nil {
			s.log.WithField("message", err.Error()).Error("producer")
			os.Exit(1)
		}

		type metadata struct {
			metaMap map[string]string
		}
		emptyJSON := metadata{
			metaMap: make(map[string]string),
		}

		event.Metadata = emptyJSON.metaMap

		event.Payload = fmt.Sprintf(`{"ImageId":"%v","ImageSetID":"%v"}`, i.ID, i.ImageSetID)
		events = append(events, event)

		recipient.IgnoreUserPreferences = false
		recipient.OnlyAdmins = false
		users = append(users, NotificationConfigUser)
		recipient.Users = users
		recipients = append(recipients, recipient)

		notify.OrgID = i.OrgID
		notify.Context = fmt.Sprintf(`{"ImageName":"%v"}`, i.Name)
		notify.Events = events
		notify.Recipients = recipients

		// assemble the message to be sent
		recordKey := "ImageCreationStarts"
		recordValue, _ := json.Marshal(notify)

		s.log.WithField("message", recordValue).Info("Preparing record for producer")

		// create a delivery channel to catch delivery errors
		deliveryChan := make(chan kafka.Event)
		defer close(deliveryChan)

		// send the message
		perr := p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Key:            []byte(recordKey),
			Value:          []byte(recordValue),
		}, deliveryChan)

		if perr != nil {
			s.log.WithFields(log.Fields{"message": perr.Error(), "topic": topic}).Error("Error on produce")
			return notify, perr
		}

		// wait for notification delivery
		err = WaitNotificationDelivery(deliveryChan, kafkacommon.DefaultDeliveryTimeout)
		if err != nil {
			s.log.WithFields(log.Fields{"message": err.Error(), "topic": topic}).Error("SendNotification message delivery failed")
			return notify, err
		}

		p.Close()
		s.log.WithField("message", topic).Info("SendNotification message was produced to topic")
		fmt.Printf("SendNotification message was produced to topic %s!\n", topic)
		return notify, nil
	}
	return notify, nil
}

// SetDevicesUpdateAvailabilityFromImageSet set whether updates available or not for all devices that use images of imageSet.
func (s *ImageService) SetDevicesUpdateAvailabilityFromImageSet(orgID string, ImageSetID uint) error {
	logger := s.log.WithFields(log.Fields{"org_id": orgID, "image_set": ImageSetID, "context": "SetDevicesUpdateAvailabilityFromImageSet"})

	// get the last image with success status
	var lastImage models.Image
	if result := db.Org(orgID, "").Where("(image_set_id = ? AND status = ?)", ImageSetID, models.ImageStatusSuccess).Order("created_at DESC").First(&lastImage); result.Error != nil {
		return result.Error
	}

	// update all devices with last image that has update_available=true to update_available=false
	if result := db.Org(orgID, "").Model(&models.Device{}).
		Where("(update_available = ? AND image_id = ?)", true, lastImage.ID).
		UpdateColumn("update_available", false); result.Error != nil {
		logger.WithField("error", result.Error).Error("Error occurred while updating device update_available")
		return result.Error
	}

	// Create priorImagesSubQuery query for all successfully created images prior to lastImage
	priorImagesSubQuery := db.Org(orgID, "").Model(&models.Image{}).Select("id").Where("image_set_id = ? AND status = ? AND created_at < ?",
		ImageSetID, models.ImageStatusSuccess, lastImage.CreatedAt)

	// Update all devices with prior images that has update_available=false to update_available=true
	if result := db.Org(orgID, "").Model(&models.Device{}).
		Where("(update_available = ? AND image_id IN (?))", false, priorImagesSubQuery).
		UpdateColumn("update_available", true); result.Error != nil {
		logger.WithField("error", result.Error).Error("Error occurred when updating org_id devices update_available")
		return result.Error
	}

	return nil
}

// GetImagesViewCount get the Images view records count
func (s *ImageService) GetImagesViewCount(tx *gorm.DB) (int64, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return 0, err
	}

	if tx == nil {
		tx = db.DB
	}

	var count int64
	result := db.OrgDB(orgID, tx, "").Model(&models.Image{}).Count(&count)

	if result.Error != nil {
		s.log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error("Error getting images count")
		return 0, result.Error
	}

	return count, nil
}

// GetImageDevicesCount returns a list of devices running an image in an org.
func (s *ImageService) GetImageDevicesCount(imageId uint) (int64, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Error getting orgID from context")
		return 0, err
	}

	var count int64
	res := db.Org(orgID, "").Model(&models.Device{}).Where("image_id =? ", imageId).Count(&count)
	if res.Error != nil {
		s.log.WithField("error", res.Error.Error()).Error("Error getting device count")
		return 0, res.Error
	}
	return count, nil
}

// GetImagesView returns a list of Images view.
func (s *ImageService) GetImagesView(limit int, offset int, tx *gorm.DB) (*[]models.ImageView, error) {
	orgID, err := common.GetOrgIDFromContext(s.ctx)
	if err != nil {
		return nil, err
	}

	if tx == nil {
		tx = db.DB
	}

	var images []models.Image

	if result := db.OrgDB(orgID, tx, "").Debug().Limit(limit).Offset(offset).
		Preload("Installer").
		Preload("Commit").
		Find(&images); result.Error != nil {
		log.WithFields(log.Fields{"error": result.Error.Error(), "OrgID": orgID}).Error(
			"error when getting images",
		)
		return nil, result.Error
	}
	if len(images) == 0 {
		return &[]models.ImageView{}, nil
	}

	imagesView := make([]models.ImageView, 0, len(images))
	for _, image := range images {
		imageView := models.ImageView{
			ID:          image.ID,
			Name:        image.Name,
			Version:     image.Version,
			ImageType:   image.ImageType,
			Status:      image.Status,
			OutputTypes: image.OutputTypes,
			CreatedAt:   image.CreatedAt,
		}
		if image.Installer != nil && image.Installer.ImageBuildISOURL != "" && image.Installer.Status == models.ImageStatusSuccess {
			imageView.ImageBuildIsoURL = GetStorageInstallerIsoURL(image.Installer.ID)
		}
		if image.Commit != nil && image.Commit.Status == models.ImageStatusSuccess {
			imageView.CommitCheckSum = image.Commit.OSTreeCommit
		}
		imagesView = append(imagesView, imageView)
	}
	return &imagesView, nil
}

func (s *ImageService) DeleteImage(i *models.Image) error {
	if i.Status != models.ImageStatusError {
		s.log.WithFields(
			log.Fields{"Image_id": i.ID},
		).Error("Error when deleting image, only errored images can be deleted")
		return new(ImageNotInErrorState)
	}

	// if this is the only image in an image set, delete the set also
	var imageSet models.ImageSet
	result := db.Org(i.OrgID, "").Preload("Images").Where("(name = ?)", i.Name).First(&imageSet)
	if result.Error != nil {
		s.log.WithFields(
			log.Fields{"Image_id": i.ID, "error": result.Error},
		).Error("Error when getting image set")
		return result.Error
	}

	if result := db.DB.Delete(&i); result.Error != nil {
		s.log.WithFields(
			log.Fields{"Image_id": i.ID, "error": result.Error},
		).Error("Error when deleting image")
		return result.Error
	}

	if len(imageSet.Images) <= 1 {
		if result := db.DB.Delete(&imageSet); result.Error != nil {
			s.log.WithFields(
				log.Fields{"Image_id": i.ID, "error": result.Error},
			).Error("Error when deleting image set")
			return result.Error
		}
	}

	return nil
}
