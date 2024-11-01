package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"

	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
	"github.com/redhatinsights/edge-api/pkg/services/repostore"
	feature "github.com/redhatinsights/edge-api/unleash/features"

	"github.com/cavaliergopher/grab/v3"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// BuildCommand references the exec.Command for calls to the system
var BuildCommand = exec.Command

// RepoBuilderInterface defines the interface of a repository builder
type RepoBuilderInterface interface {
	BuildUpdateRepo(id uint) (*models.UpdateTransaction, error)
	StoreRepo(context.Context, *models.Repo) (*models.Repo, error)
	ImportRepo(r *models.Repo) (*models.Repo, error)
	CommitTarDownload(c *models.Commit, dest string) (string, error)
	CommitTarExtract(c *models.Commit, tarFileName string, dest string) error
	CommitTarUpload(c *models.Commit, tarFileName string) error
	CommitTarDelete(tarFileName string) error
	RepoPullLocalStaticDeltas(u *models.Commit, o *models.Commit, uprepo string, oldrepo string) error
}

// RepoBuilder is the implementation of a RepoBuilderInterface
type RepoBuilder struct {
	Service
	FilesService  FilesService
	repoService   RepoServiceInterface
	commitService CommitServiceInterface
	Log           log.FieldLogger
}

// NewRepoBuilder initializes the repository builder in this package
func NewRepoBuilder(ctx context.Context, log log.FieldLogger) RepoBuilderInterface {
	return &RepoBuilder{
		Service:       Service{ctx: ctx, log: log.WithField("service", "repobuilder")},
		FilesService:  NewFilesService(log),
		repoService:   NewRepoService(ctx, log),
		commitService: NewCommitService(ctx, log),
		Log:           log,
	}
}

// BuildUpdateRepo build an update repo with the set of commits all merged into a single repo
// with static deltas generated between them all
func (rb *RepoBuilder) BuildUpdateRepo(id uint) (*models.UpdateTransaction, error) {
	var update *models.UpdateTransaction
	if err := db.DB.Preload("DispatchRecords").
		Preload("Devices").
		Joins("Commit").
		Joins("Repo").
		Preload("OldCommits").
		First(&update, id).Error; err != nil {

		if err == gorm.ErrRecordNotFound {
			rb.log.WithField("updateID", id).Error("update transaction does not exist")
			return nil, new(UpdateNotFoundError)
		}
		rb.log.WithField("error", err.Error()).Error("error occurred retrieving update-transaction")
		return nil, err
	}

	if update.Commit == nil {
		rb.log.Error("nil pointer to models.UpdateTransaction.Commit provided")
		return nil, errors.New("invalid models.UpdateTransaction.Commit Provided: nil pointer")
	}

	rb.log = rb.log.WithFields(log.Fields{
		"to_commit_id": update.Commit.ID,
		"update_id":    update.ID})

	if update.Repo == nil {
		rb.log.Error("Repo is unavailable")
		return nil, errors.New("repo unavailable")
	}

	// Skipping this process if the feature flag is enabled.
	// Without static-deltas, this just downloads a commit repo and re-uploads as an update repo.
	// This will retain the original commit repo URL as the update URL.
	// (e.g., for a 2GB commit, this saves 4GB+ in traffic and local disk space on the pod)
	if !feature.SkipUpdateRepo.IsEnabled() {
		rb.log.WithField("update_transaction", update).Info("UPGRADE: update repo build started")

		fromCommitHash := "undefined"
		fromCommit := &models.StaticDeltaCommit{
			Rev: fromCommitHash,
		}

		oldCommitCount := len(update.OldCommits)
		if oldCommitCount > 0 {
			fromCommit.Rev = update.OldCommits[0].OSTreeCommit
		}

		toCommit := &models.StaticDeltaCommit{
			Rev: update.Commit.OSTreeCommit,
		}

		// FIXME: replace with new struct
		rb.log.WithFields(log.Fields{
			"old_commit_count": oldCommitCount,
			"from_commit_hash": fromCommit.Rev,
			"to_commit_hash":   toCommit.Rev,
		}).Info("UPGRADE: static delta commit to and from is set")

		staticDeltaState := &models.StaticDeltaState{
			Name:   models.GetStaticDeltaName(fromCommit.Rev, toCommit.Rev),
			OrgID:  update.OrgID,
			Status: models.StaticDeltaStatusDownloading,
		}

		if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
			rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			return nil, errors.New("error saving static delta state")
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		// create the local FQ path
		cfg := config.Get()
		path := filepath.Clean(filepath.Join(cfg.RepoTempPath, "upd/", strconv.FormatUint(uint64(update.ID), 10)))
		err := os.MkdirAll(path, os.FileMode(0755))
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error creating update path")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}
			return nil, err
		}
		rb.log.WithField("path", path).Info("UPGRADE: local update path set")

		// change to FQ path directory
		err = os.Chdir(path)
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error changing directory to update path")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, err
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		// download the commit tarfile
		tarFileName, err := rb.CommitTarDownload(update.Commit, path)
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error downloading tar")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, fmt.Errorf("error download repo repo :: %s", err.Error())
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		// extract the tarfile to local FQ path
		err = rb.CommitTarExtract(update.Commit, tarFileName, path)
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error extracting tar")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, fmt.Errorf("error extracting repo :: %s", err.Error())
		}
		rb.log.WithFields(log.Fields{
			"updateID":   update.ID,
			"OldCommits": len(update.OldCommits)}).
			Info("Old commits found to this commit")

		err = rb.CommitTarDelete(tarFileName)
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error deleting commit tarfile")
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		// create local FQ path for static delta work
		stagePath := filepath.Clean(filepath.Join(path, "staging"))
		err = os.MkdirAll(stagePath, os.FileMode(0755))
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error making dir")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, fmt.Errorf("error mkdir :: %s", err.Error())
		}
		rb.log.WithField("stage_path", stagePath).Info("UPGRADE: static delta stage path set")

		err = os.Chdir(stagePath)
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error changing dir")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, fmt.Errorf("error chdir :: %s", err.Error())
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		// If there are any old commits, we need to download them all to be merged
		// into the update commit repo
		for _, commit := range update.OldCommits {
			rb.log.WithFields(log.Fields{
				"updateID":            update.ID,
				"commit.OSTreeCommit": commit.OSTreeCommit,
				"OldCommits":          commit.ID}).
				Info("Calculate diff from previous commit")
			commit := commit // this will prevent implicit memory aliasing in the loop
			stageCommitPath := filepath.Clean(filepath.Join(stagePath, commit.OSTreeCommit))
			tarFileName, err := rb.CommitTarDownload(&commit, stageCommitPath)
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error downloading tar")
				staticDeltaState.Status = models.StaticDeltaStatusError
				if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
					rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
				}

				return nil, fmt.Errorf("error Upload repo repo :: %s", err.Error())
			}

			rb.log.WithField("update_transaction", update).Info("UPGRADE: to_commit repo tarfile downloaded")

			err = rb.CommitTarExtract(update.Commit, tarFileName, stageCommitPath)
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error extracting repo")
				staticDeltaState.Status = models.StaticDeltaStatusError
				if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
					rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
				}

				return nil, err
			}

			err = rb.CommitTarDelete(tarFileName)
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error deleting commit tarfile")
			}

			rb.log.WithField("update_transaction", update).Info("UPGRADE: to_commit repo tarfile extracted")

			staticDeltaState.Status = models.StaticDeltaStatusGenerating
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")

				return nil, errors.New("Error saving static delta state")
			}

			rb.log.WithField("update_transaction", update).Info("UPGRADE: updating repo with static delta and summary")
			err = rb.RepoPullLocalStaticDeltas(update.Commit, &commit, filepath.Clean(filepath.Join(path, "repo")),
				filepath.Clean(filepath.Join(stageCommitPath, "repo")))
			if err != nil {
				rb.log.WithField("error", err.Error()).Error("Error pulling static deltas")
				staticDeltaState.Status = models.StaticDeltaStatusError
				if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
					rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
				}

				return nil, err
			}
			rb.log.WithField("update_transaction", update).Info("UPGRADE: updating repo complete")
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		// Once all the old commits have been pulled into the update commit's repo
		// and has static deltas generated, then we don't need the old commits
		// anymore.
		err = os.RemoveAll(stagePath)
		if err != nil {
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, err
		}

		// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/

		rb.log.Info("Upload repo")

		staticDeltaState.Status = models.StaticDeltaStatusUploading
		if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
			rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")

			return nil, errors.New("Error saving static delta state")
		}

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")

		repoURL, err := rb.FilesService.GetUploader().UploadRepo(filepath.Clean(filepath.Join(path, "repo")), strconv.FormatUint(uint64(update.ID), 10), "private")
		if err != nil {
			rb.log.WithField("error", err.Error()).Error("error occurred while uploading repo")
			staticDeltaState.Status = models.StaticDeltaStatusError
			if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
				rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")
			}

			return nil, err
		}
		rb.log.Info("Finished uploading repo")

		staticDeltaState.Status = models.StaticDeltaStatusReady
		staticDeltaState.URL = repoURL
		if saveErr := staticDeltaState.Save(rb.log); saveErr != nil {
			rb.log.WithField("error", saveErr.Error()).Error("error saving static delta state")

			return nil, errors.New("Error saving static delta state")
		}

		update.Repo.URL = repoURL
		rb.log.WithField("update_transaction", update).Info("UPGRADE: point update to new update repo")

		rb.log.WithField("static_delta_state", staticDeltaState).Info("UPGRADE: static delta state")
	}

	// FIXME: (remove) moved this logic to the calling function to avoid extra DB lookups when static delta is not generated
	// 			remove when tested successfully
	if feature.SkipUpdateRepo.IsEnabled() {
		// grab the original commit URL
		updateCommit, err := rb.commitService.GetCommitByID(update.CommitID, update.OrgID)
		if err != nil {
			return nil, err
		}
		update.Repo.URL = updateCommit.Repo.URL
		rb.log.WithField("update_transaction", update).Info("UPGRADE: point update to commit repo")
	}

	rb.log.WithField("repo", update.Repo.URL).Info("Update repo URL")
	update.Repo.Status = models.RepoStatusSuccess
	if err := db.DB.Omit("Devices.*").Save(&update).Error; err != nil {
		return nil, err
	}
	if err := db.DB.Omit("Devices.*").Save(&update.Repo).Error; err != nil {
		return nil, err
	}

	return update, nil
}

// StoreRepo requests Pulp to create/update an ostree repo from an IB commit
func (rb *RepoBuilder) StoreRepo(ctx context.Context, repo *models.Repo) (*models.Repo, error) {
	var cmt models.Commit
	cmtDB := db.DB.Where("repo_id = ?", repo.ID).First(&cmt)
	if cmtDB.Error != nil {
		return repo, cmtDB.Error
	}

	var err error
	log.WithContext(ctx).Debug("Storing repo via Pulp")
	repo.PulpURL, err = repostore.PulpRepoStore(ctx, cmt.OrgID, *cmt.RepoID, cmt.ImageBuildTarURL)
	if err != nil {
		log.WithContext(ctx).WithField("error", err.Error()).Error("Error storing Image Builder commit in Pulp OSTree repo")

		repo.PulpStatus = models.RepoStatusError
		result := db.DB.Save(&repo)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo")
			return repo, fmt.Errorf("error saving status :: %s", result.Error.Error())
		}

		return repo, err
	}

	repo.PulpStatus = models.RepoStatusSuccess
	result := db.DB.Save(&repo)
	if result.Error != nil {
		rb.log.WithField("error", result.Error.Error()).Error("Error saving repo")
		return repo, fmt.Errorf("error saving status :: %s", result.Error.Error())
	}

	redactedURL, _ := url.Parse(repo.PulpURL)
	log.WithContext(ctx).WithField("repo_url", redactedURL.Redacted()).Info("Commit stored in Pulp OSTree repo")

	return repo, nil
}

// ImportRepo (unpack and upload) a single repo
func (rb *RepoBuilder) ImportRepo(r *models.Repo) (*models.Repo, error) {
	// FIXME: delete after Pulp Store Repo is stable
	var cmt models.Commit
	cmtDB := db.DB.Where("repo_id = ?", r.ID).First(&cmt)
	if cmtDB.Error != nil {
		return nil, cmtDB.Error
	}

	cfg := config.Get()
	path := filepath.Clean(filepath.Join(cfg.RepoTempPath, strconv.FormatUint(uint64(r.ID), 10)))
	rb.log.WithField("path", path).Debug("Storing repo via AWS S3")
	err := os.MkdirAll(path, os.FileMode(0755))
	if err != nil {
		rb.log.Error(err)
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}

	tarFileName, err := rb.CommitTarDownload(&cmt, path)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error downloading repo...")
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo...")
		}
		return nil, fmt.Errorf("error downloading repo")
	}
	errUpload := rb.CommitTarUpload(&cmt, tarFileName)
	if errUpload != nil {
		rb.log.WithField("error", errUpload.Error()).Error("Error uploading repo...")
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo...")
		}
		return nil, fmt.Errorf("error Upload repo repo :: %s", errUpload.Error())
	}
	err = rb.CommitTarExtract(&cmt, tarFileName, path)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error extracting repo")
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			rb.log.WithField("error", result.Error.Error()).Error("Error saving repo")
		}
		return nil, fmt.Errorf("error extracting repo :: %s", err.Error())
	}

	err = rb.CommitTarDelete(tarFileName)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error deleting commit tarfile")
	}

	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/
	repoURL, err := rb.FilesService.GetUploader().UploadRepo(filepath.Clean(filepath.Join(path, "repo")), strconv.FormatUint(uint64(r.ID), 10), "public-read")
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

	redactedURL, _ := url.Parse(r.URL)
	rb.log.WithField("repo_url", redactedURL.Redacted()).Info("Commit stored in AWS OSTree repo")

	return r, nil
}

// CommitTarDownload downloads and extracts the repo tarball to dest dir
func (rb *RepoBuilder) CommitTarDownload(c *models.Commit, dest string) (string, error) {
	// ensure we weren't passed a nil pointer
	if c == nil {
		rb.log.Error("nil pointer to models.Commit provided")
		return "", errors.New("invalid Commit Provided: nil pointer")
	}
	rb.log = rb.log.WithField("commitID", c.ID)
	rb.log.Info("Downloading repo")

	err := os.MkdirAll(dest, os.FileMode(0755))
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

	if c.ExternalURL {
		rb.log.WithFields(log.Fields{"filepath": tarFileName, "imageBuildTarURL": c.ImageBuildTarURL}).Debug("Grabbing tar file")
		_, err = grab.Get(tarFileName, c.ImageBuildTarURL)

		if err != nil {
			rb.log.WithField("error", err.Error()).Error("Error grabbing tar file")
			return "", err
		}
	} else {
		rb.log.WithFields(log.Fields{"filepath": tarFileName, "imageBuildTarURL": c.ImageBuildTarURL}).Debug("Downloading tar file")
		downloader := rb.FilesService.GetDownloader()
		if err := downloader.DownloadToPath(c.ImageBuildTarURL, tarFileName); err != nil {
			rb.log.WithField("error", err.Error()).Error("Error downloading tar file")
			return "", err
		}
	}
	rb.log.Info("Download finished")

	return tarFileName, nil
}

func (rb *RepoBuilder) uploadTarRepo(orgID, imageName string, repoID int) (string, error) {
	rb.log.Info("Start upload tar repo")
	uploadPath := fmt.Sprintf("v2/%s/tar/%v/%s", orgID, repoID, imageName)
	uploadPath = filepath.Clean(uploadPath)
	url, err := rb.FilesService.GetUploader().UploadFile(imageName, uploadPath)

	if err != nil {
		return "error", fmt.Errorf("error uploading the Tar :: %s :: %s", uploadPath, err.Error())
	}
	rb.log.Info("Finish upload tar repo")

	return url, nil
}

// UploadVersionRepo uploads the repo tarball to the repository storage
func (rb *RepoBuilder) CommitTarUpload(c *models.Commit, tarFileName string) error {
	if c == nil {
		rb.log.Error("nil pointer to models.Commit provided")
		return errors.New("invalid Commit Provided: nil pointer")
	}
	if c.RepoID == nil {
		rb.log.Error("nil pointer to models.Commit.RepoID provided")
		return errors.New("invalid Commit.RepoID Provided: nil pointer")
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
	c.ExternalURL = false
	result := db.DB.Save(c)
	if result.Error != nil {
		rb.log.WithField("error", result.Error.Error()).Error("Error saving tar file")
		return result.Error
	}
	rb.log.Info("Repo uploaded")
	return nil
}

// ExtractVersionRepo Download and Extract the repo tarball to dest dir
func (rb *RepoBuilder) CommitTarExtract(c *models.Commit, tarFileName string, dest string) error {
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
	err = rb.FilesService.GetExtractor().Extract(tarFile, filepath.Clean(dest))
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error extracting tar file")
		return err
	}

	rb.log.WithField("filepath", tarFileName).Debug("Unpacking tarball finished")

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

func (rb *RepoBuilder) CommitTarDelete(tarFileName string) error {
	err := os.Remove(tarFileName)
	if err != nil {
		rb.log.WithField("error", err.Error()).Error("Error removing tar file")
		return err
	}

	return nil
}

// RepoPullLocalStaticDeltas pull local repo into the new update repo and compute static deltas
// uprepo should be where the update commit lives, u is the update commit
// oldrepo should be where the old commit lives, o is the commit to be merged
func (rb *RepoBuilder) RepoPullLocalStaticDeltas(u *models.Commit, o *models.Commit, uprepo string, oldrepo string) error {
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

	var output []byte

	// pull the local repo at the exact rev (which was HEAD of o.OSTreeRef)
	cmd := BuildCommand("/usr/bin/ostree", "pull-local", "--repo", uprepo, oldrepo, oldRevParse)
	if output, err := cmd.CombinedOutput(); err != nil {
		rb.log.WithFields(
			log.Fields{"error": err.Error(), "OSTreeCommit": oldRevParse, "output": string(output)},
		).Error("error occurred while running pull-local command")
		return err
	}
	rb.log.WithFields(log.Fields{
		"command":       cmd,
		"output":        string(output),
		"to_repo":       uprepo,
		"from_repo":     oldrepo,
		"from_revparse": oldRevParse,
	}).Info("UPGRADE: from_commit pulled into to_commit")

	// generate static delta
	cmd = BuildCommand("/usr/bin/ostree", "static-delta", "generate", "--repo", uprepo, "--from", oldRevParse, "--to", updateRevParse)
	if output, err := cmd.CombinedOutput(); err != nil {
		rb.log.WithFields(
			log.Fields{"error": err.Error(), "OSTreeCommit": oldRevParse, "output": string(output)},
		).Error("error occurred while running static-delta command")
		return err
	}
	rb.log.WithFields(log.Fields{
		"command":       cmd,
		"output":        string(output),
		"to_repo":       uprepo,
		"from_revparse": oldRevParse,
		"to_revparse":   updateRevParse,
	}).Info("UPGRADE: static delta generated")

	// confirm static delta
	cmd = BuildCommand("/usr/bin/ostree", "static-delta", "list", "--repo", uprepo)
	if output, err := cmd.CombinedOutput(); err != nil {
		rb.log.WithFields(
			log.Fields{"error": err.Error(), "OSTreeCommit": oldRevParse, "output": string(output)},
		).Error("error occurred while running static-delta command")
		return err
	}
	rb.log.WithFields(log.Fields{
		"command": cmd,
		"output":  string(output),
		"to_repo": uprepo,
	}).Info("UPGRADE: static delta info")

	// update ostree summary
	cmd = BuildCommand("/usr/bin/ostree", "summary", "--repo", uprepo, "-u")
	if output, err := cmd.CombinedOutput(); err != nil {
		rb.log.WithFields(
			log.Fields{"error": err.Error(), "OSTreeSummary": uprepo, "output": string(output)},
		).Error("error occurred while running summary update command")
		return err
	}
	rb.log.WithFields(log.Fields{
		"command": cmd,
		"output":  string(output),
		"to_repo": uprepo,
	}).Info("UPGRADE: ostree summary updated")

	// confirm ostree summary
	cmd = BuildCommand("/usr/bin/ostree", "summary", "--repo", uprepo, "-v")
	if output, err := cmd.CombinedOutput(); err != nil {
		rb.log.WithFields(
			log.Fields{"error": err.Error(), "OSTreeSummary": uprepo, "output": string(output)},
		).Error("error occurred while running summary view command")
		return err
	}
	rb.log.WithFields(log.Fields{
		"command": cmd,
		"output":  string(output),
		"to_repo": uprepo,
	}).Info("UPGRADE: ostree summary info")

	return nil
}

// RepoRevParse Handle the RevParse separate since we need the stdout parsed
func RepoRevParse(path string, ref string) (string, error) {
	cmd := BuildCommand("ostree", "rev-parse", "--repo", path, ref)

	var res bytes.Buffer
	cmd.Stdout = &res

	err := cmd.Run()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(res.String()), nil
}
