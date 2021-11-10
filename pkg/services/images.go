package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
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
)

// WaitGroup is the waitg roup for pending image builds
var WaitGroup sync.WaitGroup

// ImageServiceInterface defines the interface that helps handle
// the business logic of creating RHEL For Edge Images
type ImageServiceInterface interface {
	CreateImage(image *models.Image, account string) error
	UpdateImage(image *models.Image, account string, previousImage *models.Image) error
	AddUserInfo(image *models.Image) error
	UpdateImageStatus(image *models.Image) (*models.Image, error)
	SetErrorStatusOnImage(err error, i *models.Image)
	CreateRepoForImage(i *models.Image) (*models.Repo, error)
	GetImageByID(id string) (*models.Image, error)
	GetImageByOSTreeCommitHash(commitHash string) (*models.Image, error)
	CheckImageName(name, account string) (bool, error)
	RetryCreateImage(image *models.Image) error
}

// NewImageService gives a instance of the main implementation of a ImageServiceInterface
func NewImageService(ctx context.Context, log *log.Entry) ImageServiceInterface {
	return &ImageService{
		ctx:          ctx,
		imageBuilder: imagebuilder.InitClient(ctx, log),
		log:          log,
		repoBuilder:  NewRepoBuilder(ctx, log),
	}
}

// ImageService is the main implementation of a ImageServiceInterface
type ImageService struct {
	ctx context.Context
	log *log.Entry

	imageBuilder imagebuilder.ClientInterface
	repoBuilder  RepoBuilderInterface
}

// CreateImage creates an Image for an Account on Image Builder and on our database
func (s *ImageService) CreateImage(image *models.Image, account string) error {
	var imageSet models.ImageSet
	imageSet.Account = account
	imageSet.Name = image.Name
	imageSet.Version = image.Version
	set := db.DB.Create(&imageSet)
	if set.Error == nil {
		image.ImageSetID = &imageSet.ID
	}
	image, err := s.imageBuilder.ComposeCommit(image)
	if err != nil {
		return err
	}
	image.Account = account
	image.Commit.Account = account
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
func (s *ImageService) UpdateImage(image *models.Image, account string, previousImage *models.Image) error {
	if previousImage != nil {
		var currentImageSet models.ImageSet
		result := db.DB.Where("Id = ?", previousImage.ImageSetID).First(&currentImageSet)
		if result.Error != nil {
			return result.Error
		}
		currentImageSet.Version = currentImageSet.Version + 1
		image.ImageSetID = previousImage.ImageSetID
		if err := db.DB.Save(currentImageSet).Error; err != nil {
			return result.Error
		}
	}
	if image.Commit.OSTreeParentCommit == "" {
		if previousImage.Commit.OSTreeParentCommit != "" {
			image.Commit.OSTreeParentCommit = previousImage.Commit.OSTreeParentCommit
		} else {
			var repo *RepoService

			repoURL, err := repo.GetRepoByID(previousImage.Commit.RepoID)
			if err != nil {
				err := errors.NewBadRequest(fmt.Sprintf("Commit Repo wasn't found in the database: #%v", image.Commit.ID))
				return err
			}
			image.Commit.OSTreeParentCommit = repoURL.URL
		}
	}
	if image.Commit.OSTreeRef == "" {
		if previousImage.Commit.OSTreeRef != "" {
			image.Commit.OSTreeRef = previousImage.Commit.OSTreeRef

		}
		image.Commit.OSTreeRef = config.Get().DefaultOSTreeRef

	}

	image, err := s.imageBuilder.ComposeCommit(image)
	if err != nil {
		return err
	}
	image.Account = account
	image.Commit.Account = account
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
	tx := db.DB.Create(&image.Commit)
	if tx.Error != nil {
		return tx.Error
	}
	tx = db.DB.Create(&image)
	if tx.Error != nil {
		return tx.Error
	}

	s.log = s.log.WithFields(log.Fields{"imageID": image.ID, "commitID": image.Commit.ID})
	go s.postProcessImage(image.ID)

	return nil
}

func (s *ImageService) postProcessInstaller(i *models.Image) error {
	i, err := s.imageBuilder.ComposeInstaller(i)
	if err != nil {
		return err
	}
	i.Installer.Status = models.ImageStatusBuilding
	tx := db.DB.Save(&i.Installer)
	if tx.Error != nil {
		s.log.WithField("error", err.Error()).Error("Error setting installer building status")
		return err
	}
	for {
		i, err := s.UpdateImageStatus(i)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Update image status error")
			return err
		}
		if i.Installer.Status != models.ImageStatusBuilding {
			break
		}
		time.Sleep(1 * time.Minute)
	}

	if i.Installer.Status == models.ImageStatusSuccess {
		err = s.AddUserInfo(i)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Kickstart file injection failed")
			return err
		}
	}
	return nil
}

func (s *ImageService) postProcessCommit(i *models.Image) error {
	for {
		i, err := s.UpdateImageStatus(i)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Update image status error")
			return err
		}
		if i.Commit.Status != models.ImageStatusBuilding {
			break
		}
		time.Sleep(1 * time.Minute)
	}

	go func(imageBuilder imagebuilder.ClientInterface, i *models.Image) {
		i, err := imageBuilder.GetMetadata(i)
		if err != nil {
			s.log.WithField("error", err.Error()).Error("Failed getting metatada from image builder")
		} else {
			db.DB.Save(&i.Commit)
		}
	}(s.imageBuilder, i)

	_, err := s.CreateRepoForImage(i)
	if err != nil {
		s.log.WithField("error", err.Error()).Error("Failed creating repo for image")
		return err
	}
	return nil
}

func (s *ImageService) setFinalImageStatus(i *models.Image) error {
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
				i.Commit.Status = models.ImageStatusError
				db.DB.Save(i.Commit)
			}
		}

		if out == models.ImageTypeInstaller {
			if i.Installer == nil || i.Installer.Status != models.ImageStatusSuccess {
				success = false
			}
			if i.Installer.Status == models.ImageStatusBuilding {
				i.Installer.Status = models.ImageStatusError
				db.DB.Save(i.Commit)
			}
		}
	}

	if success {
		i.Status = models.ImageStatusSuccess
	} else {
		i.Status = models.ImageStatusError
	}

	tx := db.DB.Save(i.Commit)
	return tx.Error
}

// Every log message in this method already has commit id and image id injected
func (s *ImageService) postProcessImage(id uint) {
	var i *models.Image
	db.DB.Joins("Commit").Joins("Installer").First(&i, id)

	WaitGroup.Add(1) // Processing one image
	defer func() {
		WaitGroup.Done() // Done with one image (sucessfuly or not)
		s.log.Debug("Done with one image - sucessfuly or not")
		if err := recover(); err != nil {
			s.log.Fatalf("%s", err)
		}
	}()
	go func(i *models.Image) {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		sig := <-sigint
		// Reload image to get updated status
		db.DB.Joins("Commit").Joins("Installer").First(&i, i.ID)
		if i.Status == models.ImageStatusBuilding {
			s.log.WithField("signal", sig).Info("Captured signal marking image as error", sig)
			s.SetErrorStatusOnImage(nil, i)
			WaitGroup.Done()
		}
	}(i)

	err := s.postProcessCommit(i)
	if err != nil {
		s.SetErrorStatusOnImage(err, i)
		s.log.WithField("error", err.Error()).Fatal("Failed creating commit for image")
	}

	// TODO: We need to discuss this whole thing post-July deliverable
	if i.HasOutputType(models.ImageTypeInstaller) {
		err = s.postProcessInstaller(i)
		if err != nil {
			s.SetErrorStatusOnImage(err, i)
			s.log.WithField("error", err.Error()).Fatal("Failed creating installer for image")
		}
	} else {
		// Cleaning up possible non nil installers if doesnt have output type installer
		i.Installer = nil
	}

	err = s.setFinalImageStatus(i)
	if err != nil {
		s.log.WithField("error", err.Error()).Fatal("Couldn't set final image status")
	}
}

// CreateRepoForImage creates the OSTree repo to host that image
func (s *ImageService) CreateRepoForImage(i *models.Image) (*models.Repo, error) {
	s.log.Infof("Creating OSTree repo.")
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
	log.Debug("OSTree repo was saved to commit")

	repo, err := s.repoBuilder.ImportRepo(repo)
	if err != nil {
		return nil, err
	}
	log.Infof("OSTree repo is ready")

	return repo, nil
}

// SetErrorStatusOnImage is a helper functions that sets the error status on images
func (s *ImageService) SetErrorStatusOnImage(err error, i *models.Image) {
	if i.Status != models.ImageStatusError {
		i.Status = models.ImageStatusError
		tx := db.DB.Save(i)
		s.log.Debug("Image saved with error status")
		if tx.Error != nil {
			s.log.Error(tx.Error)
			panic(tx.Error)
		}
		if i.Commit != nil {
			i.Commit.Status = models.ImageStatusError
			tx := db.DB.Save(i.Commit)
			if tx.Error != nil {
				s.log.Error(tx.Error)
				panic(tx.Error)
			}
		}
		if i.Installer != nil {
			i.Installer.Status = models.ImageStatusError
			tx := db.DB.Save(i.Installer)
			if tx.Error != nil {
				s.log.Error(tx.Error)
				panic(tx.Error)
			}
		}
		if err != nil {
			s.log.Error(err)
			panic(err)
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

	err := s.downloadISO(imageName, downloadURL)
	if err != nil {
		return fmt.Errorf("error downloading ISO file :: %s", err.Error())
	}

	err = s.addSSHKeyToKickstart(sshKey, username, kickstart)
	if err != nil {
		return fmt.Errorf("error adding ssh key to kickstart file :: %s", err.Error())
	}

	err = s.exeInjectionScript(kickstart, imageName, image.ID)
	if err != nil {
		return fmt.Errorf("error execuiting fleetkick script :: %s", err.Error())
	}

	err = s.calculateChecksum(imageName, image)
	if err != nil {
		return fmt.Errorf("error calculating checksum for ISO :: %s", err.Error())
	}

	err = s.uploadISO(image, imageName)
	if err != nil {
		return fmt.Errorf("error uploading ISO :: %s", err.Error())
	}

	err = s.cleanFiles(kickstart, imageName, image.ID)
	if err != nil {
		return fmt.Errorf("error cleaning files :: %s", err.Error())
	}

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

	log.Infof("Opening file %s", cfg.TemplatesPath)
	t, err := template.ParseFiles(cfg.TemplatesPath + "templateKickstart.ks")
	if err != nil {
		return err
	}

	log.Infof("Creating file %s", kickstart)
	file, err := os.Create(kickstart)
	if err != nil {
		return err
	}

	log.Infof("Injecting username %s and key %s into template", username, sshKey)
	err = t.Execute(file, td)
	if err != nil {
		return err
	}
	file.Close()

	return nil
}

// Download created ISO into the file system.
func (s *ImageService) downloadISO(isoName string, url string) error {
	log.Infof("Creating iso %s", isoName)
	iso, err := os.Create(isoName)
	if err != nil {
		return err
	}
	defer iso.Close()

	log.Infof("Downloading ISO %s", url)
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	_, err = io.Copy(iso, res.Body)
	if err != nil {
		return err
	}

	return nil
}

// Upload finished ISO to S3
func (s *ImageService) uploadISO(image *models.Image, imageName string) error {

	uploadPath := fmt.Sprintf("%s/isos/%s.iso", image.Account, image.Name)
	filesService := NewFilesService()
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
		return err
	}
	log.Info("Kickstart file " + kickstart + " removed!")

	err = os.Remove(isoName)
	if err != nil {
		return err
	}
	log.Info("ISO file " + isoName + " removed!")

	workDir := fmt.Sprintf("/var/tmp/workdir%d", imageID)
	err = os.RemoveAll(workDir)
	if err != nil {
		return err
	}
	log.Info("work dir file " + workDir + " removed!")

	return nil
}

// UpdateImageStatus updates the status of an commit and/or installer based on Image Builder's status
func (s *ImageService) UpdateImageStatus(image *models.Image) (*models.Image, error) {
	if image.Commit.Status == models.ImageStatusBuilding {
		image, err := s.imageBuilder.GetCommitStatus(image)
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
		image, err := s.imageBuilder.GetInstallerStatus(image)
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
	err := os.Mkdir(workDir, 0755)
	if err != nil {
		return err
	}

	cmd := exec.Command(fleetBashScript, kickstart, image, image, workDir)
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	log.Infof("fleetkick output: %s\n", output)
	return nil
}

// Calculate the checksum of the final ISO.
func (s *ImageService) calculateChecksum(isoPath string, image *models.Image) error {
	log.Infof("Calculating sha256 checksum for ISO %s", isoPath)

	fh, err := os.Open(isoPath)
	if err != nil {
		return err
	}
	defer fh.Close()

	sumCalculator := sha256.New()
	_, err = io.Copy(sumCalculator, fh)
	if err != nil {
		return err
	}

	image.Installer.Checksum = hex.EncodeToString(sumCalculator.Sum(nil))
	log.Infof("Checksum (sha256): %s", image.Installer.Checksum)
	tx := db.DB.Save(&image.Installer)
	if tx.Error != nil {
		return tx.Error
	}

	return nil
}

func addImageExtraData(image *models.Image) (*models.Image, error) {
	if image.InstallerID != nil {
		result := db.DB.First(&image.Installer, image.InstallerID)
		if result.Error != nil {
			return nil, result.Error
		}
	}
	err := db.DB.Model(image).Association("Packages").Find(&image.Packages)
	if err != nil {
		return nil, err
	}
	return image, nil
}

// GetImageByID retrieves an image by its identifier
func (s *ImageService) GetImageByID(imageID string) (*models.Image, error) {
	var image models.Image
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return nil, new(AccountNotSet)
	}
	id, err := strconv.Atoi(imageID)
	if err != nil {
		return nil, new(IDMustBeInteger)
	}
	result := db.DB.Where("images.account = ?", account).Joins("Commit").First(&image, id)
	if result.Error != nil {
		return nil, new(ImageNotFoundError)
	}
	return addImageExtraData(&image)
}

// GetImageByOSTreeCommitHash retrieves an image by its ostree commit hash
func (s *ImageService) GetImageByOSTreeCommitHash(commitHash string) (*models.Image, error) {
	var image models.Image
	account, err := common.GetAccountFromContext(s.ctx)
	if err != nil {
		return nil, new(AccountNotSet)
	}
	result := db.DB.Where("images.account = ? and os_tree_commit = ?", account, commitHash).Joins("Commit").First(&image)
	if result.Error != nil {
		return nil, new(ImageNotFoundError)
	}
	return addImageExtraData(&image)
}

// RetryCreateImage retries the whole post process of the image creation
func (s *ImageService) RetryCreateImage(image *models.Image) error {
	go s.postProcessImage(image.ID)

	return nil
}
