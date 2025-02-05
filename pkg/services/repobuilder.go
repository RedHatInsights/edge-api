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

	"github.com/cavaliergopher/grab/v3"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// BuildCommand references the exec.Command for calls to the system
var BuildCommand = exec.Command

// RepoBuilderInterface defines the interface of a repository builder
type RepoBuilderInterface interface {
	BuildUpdateRepo(ctx context.Context, id uint) (*models.UpdateTransaction, error)
	StoreRepo(ctx context.Context, repo *models.Repo) (*models.Repo, error)
	ImportRepo(ctx context.Context, r *models.Repo) (*models.Repo, error)
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
func (rb *RepoBuilder) BuildUpdateRepo(ctx context.Context, id uint) (*models.UpdateTransaction, error) {
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

	// grab the original commit URL
	updateCommit, err := rb.commitService.GetCommitByID(update.CommitID, update.OrgID)
	if err != nil {
		return nil, err
	}
	update.Repo.URL = updateCommit.Repo.ContentURL(ctx)
	rb.log.WithField("update_transaction", update).Info("UPGRADE: point update to commit repo")

	rb.log.WithField("repo", update.Repo.DistributionURL(ctx)).Info("Update repo URL")
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
	repo.PulpID, repo.PulpURL, err = repostore.PulpRepoStore(ctx, cmt.OrgID, *cmt.RepoID, cmt.ImageBuildTarURL,
		repo.PulpID, repo.PulpURL, cmt.OSTreeParentRef)
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

	return repo, nil
}

// ImportRepo (unpack and upload) a single repo
func (rb *RepoBuilder) ImportRepo(ctx context.Context, r *models.Repo) (*models.Repo, error) {
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

	logURL, _ := url.Parse(r.DistributionURL(ctx))
	rb.log.WithField("repo_url", logURL.Redacted()).Info("Commit stored in AWS OSTree repo")

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
