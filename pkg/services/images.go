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
	"path/filepath"
	"strconv"
	"sync"
	"text/template"
	"time"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/imagebuilder"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	log "github.com/sirupsen/logrus"

	"gorm.io/gorm"

	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"gopkg.in/confluentinc/confluent-kafka-go.v1/kafka"
)

// WaitGroup is the waitg roup for pending image builds
var WaitGroup sync.WaitGroup

// ImageServiceInterface defines the interface that helps handle
// the business logic of creating RHEL For Edge Images
type ImageServiceInterface interface {
	CreateImage(image *models.Image, account string) error
	ResumeBuilds()
	UpdateImage(image *models.Image, previousImage *models.Image) error
	AddUserInfo(image *models.Image) error
	UpdateImageStatus(image *models.Image) (*models.Image, error)
	SetErrorStatusOnImage(err error, i *models.Image)
	CreateRepoForImage(i *models.Image) (*models.Repo, error)
	CreateInstallerForImage(i *models.Image) (*models.Image, chan error, error)
	GetImageByID(id string) (*models.Image, error)
	GetUpdateInfo(image models.Image) ([]models.ImageUpdateAvailable, error)
	AddPackageInfo(image *models.Image) (ImageDetail, error)
	GetImageByOSTreeCommitHash(commitHash string) (*models.Image, error)
	CheckImageName(name, account string) (bool, error)
	RetryCreateImage(image *models.Image) error
	GetMetadata(image *models.Image) (*models.Image, error)
	SetFinalImageStatus(i *models.Image)
	CheckIfIsLatestVersion(previousImage *models.Image) error
	SetBuildingStatusOnImageToRetryBuild(image *models.Image) error
	GetRollbackImage(image *models.Image) (*models.Image, error)
	SendImageNotification(image *models.Image) (ImageNotification, error)
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

// ValidateAllImageReposAreFromAccount validates the account for Third Party Repositories
func ValidateAllImageReposAreFromAccount(account string, repos []models.ThirdPartyRepo) error {

	if account == "" {
		return errors.NewBadRequest("repository information is not valid")
	}
	if len(repos) == 0 {
		return nil
	}
	var ids []uint
	for _, repo := range repos {
		ids = append(ids, repo.ID)
	}

	var existingRepos []models.ThirdPartyRepo

	if res := db.DB.Where(models.ThirdPartyRepo{Account: account}).Find(&existingRepos, ids); res.Error != nil {
		return res.Error
	}

	return nil
}

// CreateImage creates an Image for an Account on Image Builder and on our database
func (s *ImageService) CreateImage(image *models.Image, account string) error {

	// s.SendImageNotification(image) SHOULD BE INCLUDED ONCE WE COMPLETE THE NOTIFICATION

	// Check for existing ImageSet and return if exists
	// TODO: this routine needs to become a function under imagesets
	var imageSetExists bool
	var imageSetModel models.ImageSet
	var imageSet models.ImageSet

	// query to check if there are more than 0 matching imagesets
	// NOTE: see fix below, we iterate over the DB twice if one exists
	// just use the second query and return on first match
	err := db.DB.Model(imageSetModel).
		Select("count(*) > 0").
		Where("name = ? AND account = ?", image.Name, account).
		Find(&imageSetExists).
		Error

	// gorm error check
	if err != nil {
		return err
	}

	// requery to get imageset details and then return
	// FIXME: this is leftover from previous functionality. Do one query or the other.
	if imageSetExists {
		result := db.DB.Where("name = ? AND account = ?", image.Name, account).First(&imageSet)
		if result.Error != nil {
			s.log.WithField("error", result.Error.Error()).Error("Error checking for previous image set existence")
			return result.Error
		}
		s.log.WithField("imageSetName", image.Name).Error("ImageSet already exists, UpdateImage transaction expected and not CreateImage", image.Name)
		return new(ImageSetAlreadyExists)
	}

	// Create a new imageset
	// TODO: this should be a function under imagesets
	imageSet.Account = account
	imageSet.Name = image.Name

	imageSet.Version = image.Version
	set := db.DB.Create(&imageSet)
	if set.Error != nil {
		return set.Error
	}
	s.log.WithField("imageSetName", image.Name).Debug("Imageset created")

	// create an image under the new imageset
	image.Account = account
	image.ImageSetID = &imageSet.ID
	// make the initial call to Image Builder
	image, err = s.ImageBuilder.ComposeCommit(image)
	if err != nil {
		return err
	}
	image.Commit.Account = account
	// FIXME: Status below is already set in the call to ComposeCommit()
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	// TODO: Remove code when frontend is not using ImageType on the table
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.ImageType = models.ImageTypeInstaller
	} else {
		image.ImageType = models.ImageTypeCommit
	}
	// TODO: End of remove block
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.Installer.Status = models.ImageStatusCreated
		image.Installer.Account = image.Account
		tx := db.DB.Create(&image.Installer)
		if tx.Error != nil {
			return tx.Error
		}
	}

	if err := ValidateAllImageReposAreFromAccount(account, image.ThirdPartyRepositories); err != nil {
		return err
	}
	tx := db.DB.Create(&image.Commit)
	if tx.Error != nil {
		return tx.Error
	}
	tx = db.DB.Create(&image)
	if tx.Error != nil {
		return tx.Error
	}

	go s.postProcessImage(image.ID)

	return nil
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

	// important: update the image imageSet for any previous image build status,
	// otherwise image will be orphaned from its imageSet if previous build failed
	image.ImageSetID = previousImage.ImageSetID
	image.Account = previousImage.Account

	if previousImage.Status == models.ImageStatusSuccess {
		// Previous image was built successfully
		var currentImageSet models.ImageSet
		result := db.DB.Where("Id = ?", previousImage.ImageSetID).First(&currentImageSet)

		if result.Error != nil {
			s.log.WithField("error", result.Error.Error()).Error("Error retrieving the image set from parent image")
			return result.Error
		}
		currentImageSet.Version = currentImageSet.Version + 1
		if err := db.DB.Save(currentImageSet).Error; err != nil {
			return result.Error
		}

		// Always get the repo URL from the previous Image's commit
		repo, err := s.RepoService.GetRepoByID(previousImage.Commit.RepoID)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Commit repo wasn't found on the database")
			err := errors.NewBadRequest(fmt.Sprintf("Commit Repo wasn't found in the database: #%v", image.Commit.ID))
			return err
		}

		image.Commit.OSTreeParentCommit = repo.URL
		if image.Commit.OSTreeRef == "" {
			if previousImage.Commit.OSTreeRef != "" {
				image.Commit.OSTreeRef = previousImage.Commit.OSTreeRef
			} else {
				image.Commit.OSTreeRef = config.Get().DefaultOSTreeRef
			}
		}
	} else {
		// Previous image was not built sucessfully
		s.log.WithField("previousImageID", previousImage.ID).Info("Creating an update based on a image with a status that is not success")
	}
	image, err = s.ImageBuilder.ComposeCommit(image)
	if err != nil {
		return err
	}
	image.Commit.Account = previousImage.Account
	image.Commit.Status = models.ImageStatusBuilding
	image.Status = models.ImageStatusBuilding
	// TODO: Remove code when frontend is not using ImageType on the table
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.ImageType = models.ImageTypeInstaller
	} else {
		image.ImageType = models.ImageTypeCommit
	}
	// TODO: End of remove block
	if image.HasOutputType(models.ImageTypeInstaller) {
		image.Installer.Status = models.ImageStatusCreated
		image.Installer.Account = image.Account
		tx := db.DB.Create(&image.Installer)
		if tx.Error != nil {
			s.log.WithField("error", tx.Error.Error()).Error("Error creating installer")
			return tx.Error
		}
	}
	if err := ValidateAllImageReposAreFromAccount(image.Account, image.ThirdPartyRepositories); err != nil {
		return err
	}
	tx := db.DB.Create(&image.Commit)
	if tx.Error != nil {
		s.log.WithField("error", tx.Error.Error()).Error("Error creating commit")
		return tx.Error
	}
	tx = db.DB.Create(&image)
	if tx.Error != nil {
		s.log.WithField("error", tx.Error.Error()).Error("Error creating image")
		return tx.Error
	}

	s.log = s.log.WithFields(log.Fields{"updatedImageID": image.ID, "updatedCommitID": image.Commit.ID})

	s.log.Info("Image Updated successfully - starting bulding processs")

	go s.postProcessImage(image.ID)

	return nil
}

func (s *ImageService) postProcessInstaller(image *models.Image) error {
	s.log.Debug("Post processing the installer for the image")
	for {
		i, err := s.UpdateImageStatus(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Update image status error")
			return err
		}

		// the Image Builder status has changed from BUILDING
		if i.Installer.Status != models.ImageStatusBuilding {

			// if clowder is enabled, send an event on Image Build completion
			// TODO: break this out into its own function
			if clowder.IsClowderEnabled() {
				fmt.Printf("Public Port: %d\n", clowder.LoadedConfig.PublicPort)

				// get the list of brokers from the config
				brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))
				for i, b := range clowder.LoadedConfig.Kafka.Brokers {
					brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
					fmt.Println(brokers[i])
				}

				topic := "platform.edge.fleetmgmt.image-build"

				// Create Producer instance
				p, err := kafka.NewProducer(&kafka.ConfigMap{
					"bootstrap.servers": brokers[0]})
				if err != nil {
					fmt.Printf("Failed to create producer: %s", err)
					os.Exit(1)
				}

				// assemble the message to be sent
				// TODO: formalize message formats
				recordKey := "postProcessInstaller"
				recordValue, _ := json.Marshal(&image)
				s.log.WithField("message", recordValue).Debug("Preparing record for producer")
				perr := p.Produce(&kafka.Message{
					TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
					Key:            []byte(recordKey),
					Value:          []byte(recordValue),
				}, nil)
				if perr != nil {
					fmt.Println("Error sending message")
				}

				// Wait for all messages to be delivered
				p.Flush(15 * 1000)
				p.Close()

				fmt.Printf("postProcessInstaller message was produced to topic %s!\n", topic)
			}

			break
		}
		time.Sleep(1 * time.Minute)
	}

	if image.Installer.Status == models.ImageStatusSuccess {
		// Post process the installer ISO
		//	User, kickstart, checksum, etc.
		err := s.AddUserInfo(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Kickstart file injection failed")
			return err
		}
	}
	// Regardless of the status, call this method to make sure the status will be updated
	// It updates the status across the image and not just the installer
	s.log.Debug("Setting final image status")
	s.SetFinalImageStatus(image)
	s.log.WithField("status", image.Status).Debug("Processing image installer is done")
	return nil
}

func (s *ImageService) postProcessCommit(image *models.Image) error {
	s.log.Debug("Processing image build commit")
	for {
		i, err := s.UpdateImageStatus(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Update image status error")
			return err
		}
		if i.Commit.Status != models.ImageStatusBuilding {

			// if clowder is enabled, send an event on Image Build completion
			// TODO: break this out into its own function
			if clowder.IsClowderEnabled() {
				fmt.Printf("Public Port: %d\n", clowder.LoadedConfig.PublicPort)

				// get the list of brokers from the config
				brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))
				for i, b := range clowder.LoadedConfig.Kafka.Brokers {
					brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
					fmt.Println(brokers[i])
				}

				topic := "platform.edge.fleetmgmt.image-build"

				// Create Producer instance
				p, err := kafka.NewProducer(&kafka.ConfigMap{
					"bootstrap.servers": brokers[0]})
				if err != nil {
					fmt.Printf("Failed to create producer: %s", err)
					os.Exit(1)
				}

				// assemble the message to be sent
				// TODO: formalize message formats
				recordKey := "postProcessCommit"
				recordValue, _ := json.Marshal(&image)
				s.log.WithField("message", recordValue).Debug("Preparing record for producer")
				// send the message
				perr := p.Produce(&kafka.Message{
					TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
					Key:            []byte(recordKey),
					Value:          []byte(recordValue),
				}, nil)
				if perr != nil {
					fmt.Println("Error sending message")
				}

				// Wait for all messages to be delivered
				p.Flush(15 * 1000)
				p.Close()

				fmt.Printf("postProcessCommit Message was produced to topic %s!\n", topic)
			}

			break
		}
		time.Sleep(1 * time.Minute)
	}

	if image.Commit.Status == models.ImageStatusSuccess {
		i, err := s.ImageBuilder.GetMetadata(image)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Failed getting metadata from image builder")
			s.SetErrorStatusOnImage(err, i)
			return err
		}

		// Create the repo for the image
		_, err = s.CreateRepoForImage(image)
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
}

// ResumeBuilds resumes only the builds that were running when the application restarted.
func (s *ImageService) ResumeBuilds() {
	// TODO: refactor this out to ibvents pod
	s.log.Debug("Resuming builds in progress.")
	var images []models.Image
	db.DB.Debug().Where(&models.Image{Status: models.ImageStatusBuilding}).Find(&images)
	// loop through the results and start up a new process for each image
	for _, image := range images {
		log.WithField("imageID", image.ID).Debug("Resuming build process for image")

		// go s.postProcessImage(image.ID)
	}
}

func (s *ImageService) postProcessImage(id uint) {
	// NOTE: Every log message in this method already has commit id and image id injected

	s.log.Debug("Processing image build")
	// get image data from DB based on image.ID
	var i *models.Image
	db.DB.Debug().Joins("Commit").Joins("Installer").First(&i, id)

	// Request a commit from Image Builder for the image
	s.log.WithField("imageID", i.ID).Debug("Creating a commit for this image")
	err := s.postProcessCommit(i)
	if err != nil {
		s.SetErrorStatusOnImage(err, i)
		s.log.WithField("error", err.Error()).Error("Failed creating commit for image")
	}

	if i.Commit.Status == models.ImageStatusSuccess {
		s.log.Debug("Commit is successful")

		// Request an installer ISO from Image Builder for the image
		if i.HasOutputType(models.ImageTypeInstaller) {
			s.log.WithField("imageID", i.ID).Debug("Creating an installer for this image")
			i, c, err := s.CreateInstallerForImage(i)
			/* CreateInstallerForImage is also called directly from an endpoint.
			If called from the endpoint it will not block
				the caller returns the channel output to _
			Here, we catch the channel with c and use it in the next if--so it blocks.
			*/
			if c != nil {
				err = <-c
			}
			if err != nil {
				s.SetErrorStatusOnImage(err, i)
				s.log.WithField("error", err.Error()).Error("Failed creating installer for image")
			}
		}
	}
	s.log.WithField("status", i.Status).Debug("Processing image build is done")
}

// CreateRepoForImage creates the OSTree repo to host that image
func (s *ImageService) CreateRepoForImage(i *models.Image) (*models.Repo, error) {
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

	i.Commit.Repo = repo
	i.Commit.RepoID = &repo.ID

	tx = db.DB.Save(i.Commit)
	if tx.Error != nil {
		return nil, tx.Error
	}
	s.log.Debug("OSTree repo was saved to commit")

	repo, err := s.RepoBuilder.ImportRepo(repo)
	if err != nil {
		return nil, err
	}
	s.log.Infof("OSTree repo is ready")

	return repo, nil
}

// SetErrorStatusOnImage is a helper functions that sets the error status on images
func (s *ImageService) SetErrorStatusOnImage(err error, i *models.Image) {
	if i.Status != models.ImageStatusError {
		i.Status = models.ImageStatusError
		tx := db.DB.Save(i)
		s.log.Debug("Image saved with error status")
		if tx.Error != nil {
			s.log.WithField("error", tx.Error.Error()).Error("Error saving image")
		}
		if i.Commit != nil {
			i.Commit.Status = models.ImageStatusError
			tx := db.DB.Save(i.Commit)
			if tx.Error != nil {
				s.log.WithField("error", tx.Error.Error()).Error("Error saving commit")
			}
		}
		if i.Installer != nil {
			i.Installer.Status = models.ImageStatusError
			tx := db.DB.Save(i.Installer)
			if tx.Error != nil {
				s.log.WithField("error", tx.Error.Error()).Error("Error saving installer")
			}
		}
		if err != nil {
			s.log.WithField("error", tx.Error.Error()).Error("Error setting image final status")
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
	kickstart := fmt.Sprintf("%sfinalKickstart-%s_%d.ks", destPath, image.Account, image.ID)

	s.log.Debug("Downloading ISO...")
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
		return fmt.Errorf("error execuiting fleetkick script :: %s", err.Error())
	}

	s.log.Debug("Calculating the checksum for the ISO image...")
	err = s.calculateChecksum(imageName, image)
	if err != nil {
		return fmt.Errorf("error calculating checksum for ISO :: %s", err.Error())
	}

	s.log.Debug("Uploading the ISO...")
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

	s.log.WithField("isoName", isoName).Debug("Creating iso")
	iso, err := os.Create(isoName)
	if err != nil {
		return err
	}
	defer func() {
		if err := iso.Close(); err != nil {
			s.log.WithField("error", err.Error()).Error("Error closing file")
		}
	}()

	s.log.WithField("url", url).Debug("Downloading iso")
	res, err := http.Get(url) // #nosec G107
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = io.Copy(iso, res.Body)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Failed downloading iso")
		return err
	}

	return nil
}

// Upload finished ISO to S3
func (s *ImageService) uploadISO(image *models.Image, imageName string) error {

	uploadPath := fmt.Sprintf("%s/isos/%s.iso", image.Account, image.Name)
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

// CheckImageName returns false if the image doesnt exist and true if the image exists
func (s *ImageService) CheckImageName(name, account string) (bool, error) {
	s.log.WithField("name", name).Debug("Checking image name")
	var imageFindByName *models.Image
	result := db.DB.Where("name = ? AND account = ?", name, account).First(&imageFindByName)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return false, nil
		}
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
		s.log.WithField("error", err.Error()).Error("Error saving installer")
		return tx.Error
	}

	return nil
}

//ImageDetail return the structure to inform package info to images
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
		imgDetail.UpdateAdded = len(upd[len(upd)-1].PackageDiff.Removed)
		imgDetail.UpdateRemoved = len(upd[len(upd)-1].PackageDiff.Added)
		imgDetail.UpdateUpdated = len(upd[len(upd)-1].PackageDiff.Upgraded)
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
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		s.log.WithField("error", err).Error("Error retrieving account")
		return nil, new(AccountNotSet)
	}
	id, err := strconv.Atoi(imageID)
	if err != nil {
		s.log.WithField("error", err).Debug("Request related error - ID is not integer")
		return nil, new(IDMustBeInteger)
	}
	result := db.DB.Preload("Commit.Repo").Preload("Commit.InstalledPackages").Where("images.account = ?", account).Joins("Commit").First(&image, id)
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
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		s.log.Error("Error retreving account")
		return nil, new(AccountNotSet)
	}
	result := db.DB.Where("images.account = ?", account).Joins("JOIN commits ON commits.id = images.commit_id AND commits.os_tree_commit = ?", commitHash).Joins("Installer").Preload("Packages").Preload("Commit.InstalledPackages").Preload("Commit.Repo").First(&image)
	if result.Error != nil {
		s.log.WithField("error", result.Error).Error("Error retrieving image by OSTreeHash")
		return nil, new(ImageNotFoundError)
	}
	s.log = s.log.WithField("imageID", image.ID)
	s.log.Info("Image successfully retrieved by its OSTreeHash")
	return &image, nil
}

// RetryCreateImage retries the whole post process of the image creation
func (s *ImageService) RetryCreateImage(image *models.Image) error {
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
	go s.postProcessImage(image.ID)
	return nil
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
		s.log.Debug("Saving commit status")
		tx := db.DB.Save(image.Commit)
		if tx.Error != nil {
			return tx.Error
		}
	}
	if image.Installer != nil {
		s.log.Debug("Setting installer status")
		image.Installer.Status = models.ImageStatusCreated
		s.log.Debug("Saving installer status")
		tx := db.DB.Save(image.Installer)
		if tx.Error != nil {
			return tx.Error
		}
	}
	s.log.Debug("Saving image status")
	tx := db.DB.Save(image)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

// CheckIfIsLatestVersion make sure that there is no same image version present
func (s *ImageService) CheckIfIsLatestVersion(previousImage *models.Image) error {
	/*
		nowImage is the version of current Image which we are updating i.e if we are updating image1 to image 2 then image1 is the nowImage.
		currentHighestImageVersion is the latest version present in the DB of image we're updating.
		nextImageVersion is the next version of current image to which we are updating the image i.e if image3 is the next version of image2.
	*/
	var nowImage *models.Image
	var currentHighestImageVersion *models.Image
	var nextImageVersion *models.Image

	nowVersionImage := db.DB.Where("version = ?", previousImage.Version).First(&nowImage)
	if nowVersionImage.Error != nil {
		return nowVersionImage.Error
	}
	currentVersionImage := db.DB.Select("version").Where("name = ? ", previousImage.Name).Order("version desc").First(&currentHighestImageVersion)
	if currentVersionImage.Error != nil {
		return currentVersionImage.Error
	}
	var compareImageVersion = nowImage.Version + 1
	newImageVersion := db.DB.Select("version").Where("version = ? and name = ? ", compareImageVersion, previousImage.Name).Find(&nextImageVersion)
	if newImageVersion.Error != nil {
		return newImageVersion.Error
	}
	if currentHighestImageVersion.Version == compareImageVersion || nextImageVersion.Version == compareImageVersion {
		return new(ImageVersionAlreadyExists)
	}
	return nil
}

//GetUpdateInfo return package info when has an update to the image
func (s *ImageService) GetUpdateInfo(image models.Image) ([]models.ImageUpdateAvailable, error) {
	var images []models.Image
	var imageDiff []models.ImageUpdateAvailable
	updates := db.DB.Where("Image_set_id = ? and Images.Status = ? and Images.Id < ?",
		image.ImageSetID, models.ImageStatusSuccess, image.ID).Joins("Commit").
		Order("Images.updated_at desc").Find(&images)

	if updates.Error != nil {
		s.log.WithField("error", updates.Error.Error()).Error("Error retrieving update")
		return nil, new(UpdateNotFoundError)
	}
	if updates.RowsAffected == 0 {
		s.log.Info("No rows affected")
		return nil, nil
	}
	for _, upd := range images {
		upd := upd // this will prevent implicit memory aliasing in the loop
		db.DB.First(&upd.Commit, upd.CommitID)
		if err := db.DB.Model(&upd.Commit).Association("InstalledPackages").Find(&upd.Commit.InstalledPackages); err != nil {
			s.log.WithField("error", err.Error()).Error("Error retrieving installed packages")
			return nil, err
		}
		if err := db.DB.Model(&upd).Association("Packages").Find(&upd.Packages); err != nil {
			s.log.WithField("error", err.Error()).Error("Error retrieving updated packages")
			return nil, err
		}
		var delta models.ImageUpdateAvailable
		diff := GetDiffOnUpdate(image, upd)
		upd.Commit.InstalledPackages = nil // otherwise the frontend will get the whole list of installed packages
		delta.Image = upd
		delta.PackageDiff = diff
		imageDiff = append(imageDiff, delta)
	}
	return imageDiff, nil
}

//GetMetadata return package info when has an update to the image
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
func (s *ImageService) CreateInstallerForImage(image *models.Image) (*models.Image, chan error, error) {
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
		err := s.postProcessInstaller(image)
		c <- err
	}(c)
	return image, c, nil
}

// GetRollbackImage returns the previous image from the image set in case of a rollback
func (s *ImageService) GetRollbackImage(image *models.Image) (*models.Image, error) {
	s.log.Info("Getting rollback image")
	var rollback models.Image
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		s.log.Error("Error retreving account")
		return nil, new(AccountNotSet)
	}
	result := db.DB.Joins("Commit").Joins("Installer").Preload("Packages").Preload("Commit.InstalledPackages").Preload("Commit.Repo").Where(&models.Image{ImageSetID: image.ImageSetID, Account: account}).Last(&rollback, "images.id < ?", image.ID)
	if result.Error != nil {
		s.log.WithField("error", result.Error).Error("Error retrieving rollback image")
		return nil, new(ImageNotFoundError)
	}
	s.log = s.log.WithField("imageID", image.ID)
	s.log.Info("Rollback image successfully retrieved")
	return &rollback, nil
}

// SendImageNotification connects to platform.notifications.ingress on image topic
func (s *ImageService) SendImageNotification(i *models.Image) (ImageNotification, error) {
	//the code will be modified due to stage results
	var notify ImageNotification
	notify.Version = NotificationConfigVersion
	notify.Bundle = NotificationConfigBundle
	notify.Application = NotificationConfigApplication
	notify.EventType = NotificationConfigEventTypeImage
	notify.Timestamp = fmt.Sprintf("%v", time.Now().UnixNano())

	if clowder.IsClowderEnabled() {
		var users []string
		var events []EventNotification
		var event EventNotification
		var recipients []RecipientNotification
		var recipient RecipientNotification
		brokers := make([]string, len(clowder.LoadedConfig.Kafka.Brokers))
		fmt.Printf("\nSendImageNotification:brokers %v\n", brokers)
		for i, b := range clowder.LoadedConfig.Kafka.Brokers {
			brokers[i] = fmt.Sprintf("%s:%d", b.Hostname, *b.Port)
			fmt.Println(brokers[i])
		}

		topic := NotificationTopic
		fmt.Printf("\nSendImageNotification:topic: %v\n", topic)
		// Create Producer instance
		p, err := kafka.NewProducer(&kafka.ConfigMap{
			// "bootstrap.servers": "platform-mq-kafka-bootstrap.platform-mq-stage.svc:9092"})
			"bootstrap.servers": brokers[0]})
		if err != nil {
			fmt.Printf("Failed to create producer: %s", err)
			os.Exit(1)
		}

		event.Metadata = "{}"
		// payload, _ := json.Marshal(&i.ID)
		event.Payload = fmt.Sprintf("{  \"ImageId:\" : \"%v\"}", &i.ID)
		events = append(events, event)
		// fmt.Printf("\nSendImageNotification:event: %v\n", event)

		recipient.IgnoreUserPreferences = false
		recipient.OnlyAdmins = false
		users = append(users, "anferrei@redhat.com")
		recipient.Users = users
		recipients = append(recipients, recipient)
		fmt.Printf("\nSendImageNotification:recipient: %v\n", recipient)

		notify.Account = i.Account
		notify.Context = fmt.Sprintf("{  \"ImageName:\" : \"%v\"}", &i.Name)
		notify.Events = events
		notify.Recipients = recipients
		// fmt.Printf("\n ############## notify: ############ %v\n", notify)
		s.log.WithField("message", notify).Debug("Message to be sent")

		// assemble the message to be sent
		// TODO: formalize message formats
		recordKey := "ImageCreationStarts"
		recordValue, _ := json.Marshal(notify)
		s.log.WithField("message", recordKey).Debug("Preparing record for producer")
		s.log.WithField("message", recordValue).Debug("RecordValue")
		// send the message
		perr := p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Key:            []byte(recordKey),
			Value:          []byte(recordValue),
		}, nil)
		s.log.WithField("message", perr).Debug("after p.Produce")
		if perr != nil {
			s.log.WithField("message", perr.Error()).Error("Error on produce")
			fmt.Printf("\nError sending message: %v\n", perr)
			return notify, err
		}
		// Wait for all messages to be delivered
		p.Flush(15 * 1000)
		p.Close()
		return notify, nil
	}
	return notify, nil
}
