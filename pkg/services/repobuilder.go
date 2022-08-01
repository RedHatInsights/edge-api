package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/cavaliercoder/grab"
	log "github.com/sirupsen/logrus"
)

// RepoBuilderInterface defines the interface of a repository builder
type RepoBuilderInterface interface {
	BuildUpdateRepo(id uint) (*models.UpdateTransaction, error)
	ImportRepo(r *models.Repo) (*models.Repo, error)
	DownloadVersionRepo(c *models.Commit, dest string, external bool) (string, error)
	ExtractVersionRepo(c *models.Commit, tarFileName string, dest string) error
	UploadVersionRepo(c *models.Commit, tarFileName string) error
}

// RepoBuilder is the implementation of a RepoBuilderInterface
type RepoBuilder struct {
	Service
	filesService FilesService
	repoService  RepoServiceInterface
	log          *log.Entry
}

// NewRepoBuilder initializes the repository builder in this package
func NewRepoBuilder(ctx context.Context, log *log.Entry) RepoBuilderInterface {
	return &RepoBuilder{
		Service:      Service{ctx: ctx, log: log.WithField("service", "repobuilder")},
		filesService: NewFilesService(log),
		repoService:  NewRepoService(ctx, log),
		log:          log,
	}
}

// BuildUpdateRepo build an update repo with the set of commits all merged into a single repo
// with static deltas generated between them all
func (rb *RepoBuilder) BuildUpdateRepo(id uint) (*models.UpdateTransaction, error) {
	var update *models.UpdateTransaction
	db.DB.Preload("DispatchRecords").Preload("Devices").Joins("Commit").Joins("Repo").Find(&update, id)

	rb.log.Info("Starts building update repo...")
	if update == nil {
		rb.log.Error("nil pointer to models.UpdateTransaction provided")
		return nil, errors.New("invalid models.UpdateTransaction Provided: nil pointer")
	}
	if update.Commit == nil {
		rb.log.Error("nil pointer to models.UpdateTransaction.Commit provided")
		return nil, errors.New("invalid models.UpdateTransaction.Commit Provided: nil pointer")
	}
	rb.log = rb.log.WithFields(log.Fields{
		"commitID": update.Commit.ID,
		"updateID": update.ID})
	if update.Repo == nil {
		rb.log.Error("Repo is unavailable")
		return nil, errors.New("repo unavailable")
	}
	cfg := config.Get()
	path := filepath.Clean(filepath.Join(cfg.RepoTempPath, "upd/", strconv.FormatUint(uint64(update.ID), 10)))
	rb.log.WithField("path", path).Debug("Update path will be created")
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}
	tarFileName, err := rb.DownloadVersionRepo(update.Commit, path, false)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error downloading tar")
		return nil, fmt.Errorf("error Upload repo repo :: %s", err.Error())
	}
	err = rb.ExtractVersionRepo(update.Commit, tarFileName, path)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error extracting tar")
		return nil, fmt.Errorf("error extracting repo :: %s", err.Error())
	}

	if len(update.OldCommits) > 0 {
		rb.log.WithFields(log.Fields{
			"updateID":   update.ID,
			"OldCommits": len(update.OldCommits)}).
			Info("Old commits found to this commit")
		stagePath := filepath.Clean(filepath.Join(path, "staging"))
		err = os.MkdirAll(stagePath, os.FileMode(int(0755)))
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error making dir")
			return nil, fmt.Errorf("error mkdir :: %s", err.Error())
		}
		err = os.Chdir(stagePath)
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error changing dir")
			return nil, fmt.Errorf("error chdir :: %s", err.Error())
		}

		// If there are any old commits, we need to download them all to be merged
		// into the update commit repo
		for _, commit := range update.OldCommits {
			rb.log.WithFields(log.Fields{
				"updateID":            update.ID,
				"commit.OSTreeCommit": commit.OSTreeCommit,
				"OldCommits":          commit.ID}).
				Info("Calculate diff from previous commit")
			commit := commit // this will prevent implicit memory aliasing in the loop
			tarFileName, err := rb.DownloadVersionRepo(&commit, filepath.Clean(filepath.Join(stagePath, commit.OSTreeCommit)), false)
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error downloading tar")
				return nil, fmt.Errorf("error Upload repo repo :: %s", err.Error())
			}
			err = rb.ExtractVersionRepo(update.Commit, tarFileName, path)
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error extracting repo")
				return nil, err
			}
			// FIXME: hardcoding "repo" in here because that's how it comes from osbuild
			err = rb.repoPullLocalStaticDeltas(update.Commit, &commit, filepath.Clean(filepath.Join(path, "repo")),
				filepath.Clean(filepath.Join(stagePath, commit.OSTreeCommit, "repo")))
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error pulling static deltas")
				return nil, err
			}
		}

		// Once all the old commits have been pulled into the update commit's repo
		// and has static deltas generated, then we don't need the old commits
		// anymore.
		err = os.RemoveAll(stagePath)
		if err != nil {
			return nil, err
		}

	}
	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/

	rb.log.Info("Upload repo")
	repoURL, err := rb.filesService.GetUploader().UploadRepo(filepath.Clean(filepath.Join(path, "repo")), strconv.FormatUint(uint64(update.ID), 10))
	rb.log.Info("Finished uploading repo")
	if err != nil {
		return nil, err
	}

	update.Repo.URL = repoURL
	update.Repo.Status = models.RepoStatusSuccess
	if err := db.DB.Save(&update).Error; err != nil {
		return nil, err
	}
	if err := db.DB.Save(&update.Repo).Error; err != nil {
		return nil, err
	}

	return update, nil
}

// ImportRepo (unpack and upload) a single repo
func (rb *RepoBuilder) ImportRepo(r *models.Repo) (*models.Repo, error) {

	var cmt models.Commit
	cmtDB := db.DB.Where("repo_id = ?", r.ID).Find(&cmt)
	if cmtDB.Error != nil {
		return nil, cmtDB.Error
	}
	cfg := config.Get()
	path := filepath.Clean(filepath.Join(cfg.RepoTempPath, strconv.FormatUint(uint64(r.ID), 10)))
	rb.log.WithField("path", path).Debug("Importing repo...")
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		rb.log.Error(err)
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}

	tarFileName, err := rb.DownloadVersionRepo(&cmt, path, true)
	if err != nil {
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo...")
		}
		rb.log.WithField("error", err.Error()).Error("Error downloading repo...")
		return nil, fmt.Errorf("error downloading repo")
	}
	errUpload := rb.UploadVersionRepo(&cmt, tarFileName)
	if errUpload != nil {
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo...")
		}
		rb.log.WithField("error", errUpload.Error()).Error("Error uploading repo...")
		return nil, fmt.Errorf("error Upload repo repo :: %s", errUpload.Error())
	}
	err = rb.ExtractVersionRepo(&cmt, tarFileName, path)
	if err != nil {
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo")
		}
		rb.log.WithField("error", err.Error()).Error("Error extracting repo")
		return nil, fmt.Errorf("error extracting repo :: %s", err.Error())
	}
	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/
	repoURL, err := rb.filesService.GetUploader().UploadRepo(filepath.Clean(filepath.Join(path, "repo")), strconv.FormatUint(uint64(r.ID), 10))
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error uploading repo")
		return nil, fmt.Errorf("error uploading repo :: %s", err.Error())
	}

	r.URL = repoURL
	r.Status = models.RepoStatusSuccess
	result := db.DB.Save(&r)
	if result.Error != nil {
		rb.log.WithField("error", result.Error.Error()).Error("Error saving repo")
		return nil, fmt.Errorf("error saving status :: %s", result.Error.Error())
	}

	return r, nil
}

// DownloadVersionRepo Download and Extract the repo tarball to dest dir
func (rb *RepoBuilder) DownloadVersionRepo(c *models.Commit, dest string, external bool) (string, error) {
	// ensure we weren't passed a nil pointer
	if c == nil {
		rb.log.Error("nil pointer to models.Commit provided")
		return "", errors.New("invalid Commit Provided: nil pointer")
	}
	rb.log = rb.log.WithField("commitID", c.ID)
	rb.log.Info("Downloading repo")

	err := os.MkdirAll(dest, os.FileMode(int(0755)))
	if err != nil {
		return "", err
	}
	err = os.Chdir(dest)
	if err != nil {
		return "", err
	}

	// Save the tarball to the OSBuild Hash ID and then extract it
	tarFileName := "repo.tar"
	if c.ImageBuildHash != "" {
		tarFileName = strings.Join([]string{c.ImageBuildHash, "tar"}, ".")
	}
	tarFileName = filepath.Clean(filepath.Join(dest, tarFileName))

	if external {
		rb.log.WithFields(log.Fields{"filepath": tarFileName, "imageBuildTarURL": c.ImageBuildTarURL}).Debug("Grabbing tar file")
		_, err = grab.Get(tarFileName, c.ImageBuildTarURL)

		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error grabbing tar file")
			return "", err
		}
	} else {
		rb.log.WithFields(log.Fields{"filepath": tarFileName, "imageBuildTarURL": c.ImageBuildTarURL}).Debug("Downloading tar file")

		filesService := NewFilesService(rb.log.WithField("url", c.ImageBuildTarURL))
		if err := filesService.GetDownloader().DownloadToPath(c.ImageBuildTarURL, tarFileName); err != nil {
			rb.log.WithField("error", err.Error()).Error("Error downloading tar file")
			return "", err
		}
	}
	rb.log.Info("Download finished")

	return tarFileName, nil
}

func (rb *RepoBuilder) uploadTarRepo(OrgID, imageName string, repoID int) (string, error) {
	uploadPath := fmt.Sprintf("v2/%s/tar/%v/%s", OrgID, repoID, imageName)
	uploadPath = filepath.Clean(uploadPath)
	rb.log.WithField("url", uploadPath).Info("Start upload tar repo")
	filesService := NewFilesService(rb.log)
	url, err := filesService.GetUploader().UploadFile(imageName, uploadPath)

	if err != nil {
		return "error", fmt.Errorf("error uploading the Tar :: %s :: %s", uploadPath, err.Error())
	}
	rb.log.Info("Finish upload tar repo")

	return url, nil
}

// UploadVersionRepo uploads the repo tarball to the repository storage
func (rb *RepoBuilder) UploadVersionRepo(c *models.Commit, tarFileName string) error {
	if c == nil {
		rb.log.Error("nil pointer to models.Commit provided")
		return errors.New("invalid Commit Provided: nil pointer")
	}
	if c.RepoID == nil {
		rb.log.Error("nil pointer to models.Commit.RepoID provided")
		return errors.New("invalid Commit Provided: nil pointer")
	}
	repoID := int(*c.RepoID)
	rb.log = rb.log.WithFields(log.Fields{"commitID": c.ID, "filepath": tarFileName, "repoID": repoID})
	rb.log.Info("Uploading repo")
	repoTarURL, err := rb.uploadTarRepo(c.OrgID, tarFileName, repoID)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Failed to upload repo")
		return err
	}
	c.ImageBuildTarURL = repoTarURL
	result := db.DB.Save(c)
	if result.Error != nil {
		rb.log.WithField("error", result.Error.Error()).Error("Error saving tar file URL")
		return result.Error
	}
	rb.log.Info("Repo uploaded")
	return nil
}

// ExtractVersionRepo Download and Extract the repo tarball to dest dir
func (rb *RepoBuilder) ExtractVersionRepo(c *models.Commit, tarFileName string, dest string) error {
	if c == nil {
		rb.log.Error("nil pointer to models.Commit provided")
		return errors.New("invalid Commit Provided: nil pointer")
	}
	rb.log = rb.log.WithField("commitID", c.ID)
	rb.log.Info("Extracting repo")
	tarFile, err := os.Open(filepath.Clean(tarFileName))
	if err != nil {
		rb.log.WithFields(log.Fields{
			"error":    err.Error(),
			"filepath": tarFileName,
		}).Error("Failed to open file")
		return err
	}
	err = rb.filesService.GetExtractor().Extract(tarFile, filepath.Clean(filepath.Join(dest)))
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error extracting tar file")
		return err
	}

	rb.log.WithField("filepath", tarFileName).Debug("Unpacking tarball finished")

	err = os.Remove(tarFileName)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error removing tar file")
		return err
	}

	// Commenting this Block, as failing, and seems never was working, fixing this block will create a new commit with
	// a new checksum that need to be recorded back to the current commit, this will also need to change the logic of
	// the caller function of this function, this need more discussion, a bug has been created.
	/*
		var cmd *exec.Cmd
		if c.OSTreeRef == "" {
			refs := config.DistributionsRefs[config.DefaultDistribution]
			cmd = &exec.Cmd{
				Path: "/usr/bin/ostree",
				Args: []string{
					"--repo", "./repo", "commit", refs, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber),
				},
			}
		} else {
			cmd = &exec.Cmd{
				Path: "/usr/bin/ostree",
				Args: []string{
					"--repo", "./repo", "commit", c.OSTreeRef, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber),
				},
			}
		}
		err = cmd.Run()
		if err != nil {
			rb.log.WithFields(log.Fields{
				"error":   err.Error(),
				"command": fmt.Sprintf("%s %s %s %s %s %s %s", "ostree", "--repo", "./repo", "commit", c.OSTreeRef, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber)),
			}).Error("OSTree command failed")
		}
	*/
	return nil
}

// RepoPullLocalStaticDeltas pull local repo into the new update repo and compute static deltas
//  uprepo should be where the update commit lives, u is the update commit
//  oldrepo should be where the old commit lives, o is the commit to be merged
func (rb *RepoBuilder) repoPullLocalStaticDeltas(u *models.Commit, o *models.Commit, uprepo string, oldrepo string) error {
	err := os.Chdir(uprepo)
	if err != nil {
		return err
	}

	updateRevParse, err := RepoRevParse(uprepo, u.OSTreeRef)
	if err != nil {
		return err
	}
	oldRevParse, err := RepoRevParse(oldrepo, o.OSTreeRef)
	if err != nil {
		return err
	}

	// pull the local repo at the exact rev (which was HEAD of o.OSTreeRef)
	cmd := &exec.Cmd{
		Path: "/usr/bin/ostree",
		Args: []string{
			"--repo", uprepo, "pull-local", oldrepo, oldRevParse,
		},
	}
	err = cmd.Run()
	if err != nil {
		return err
	}

	// generate static delta
	cmd = &exec.Cmd{
		Path: "/usr/bin/ostree",
		Args: []string{
			"--repo", uprepo, "static-delta", "generate", "--from", oldRevParse, "--to", updateRevParse,
		},
	}
	err = cmd.Run()
	if err != nil {
		return err
	}
	return nil

}

// RepoRevParse Handle the RevParse separate since we need the stdout parsed
func RepoRevParse(path string, ref string) (string, error) {
	cmd := exec.Command("ostree", "rev-parse", "--repo", path, ref)

	var res bytes.Buffer
	cmd.Stdout = &res

	err := cmd.Run()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(res.String()), nil
}
