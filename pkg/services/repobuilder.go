package services

import (
	"context"
	"errors"
	"fmt"

	"bytes"
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
	DownloadExtractVersionRepo(c *models.Commit, dest string) error
}

// RepoBuilder is the implementation of a RepoBuilderInterface
type RepoBuilder struct {
	ctx          context.Context
	filesService FilesService
	repoService  RepoServiceInterface
	log          *log.Entry
}

// NewRepoBuilder initializes the repository builder in this package
func NewRepoBuilder(ctx context.Context, log *log.Entry) RepoBuilderInterface {
	return &RepoBuilder{
		ctx:          ctx,
		filesService: NewFilesService(),
		repoService:  NewRepoService(ctx),
		log:          log,
	}
}

// BuildUpdateRepo build an update repo with the set of commits all merged into a single repo
// with static deltas generated between them all
func (rb *RepoBuilder) BuildUpdateRepo(id uint) (*models.UpdateTransaction, error) {
	var update *models.UpdateTransaction
	db.DB.Preload("DispatchRecords").Preload("Devices").Joins("Commit").Joins("Repo").Find(&update, id)

	log.Infof("Repobuilder::BuildUpdateRepo:: Begin")
	if update == nil {
		log.Error("nil pointer to models.UpdateTransaction provided")
		return nil, errors.New("invalid models.UpdateTransaction Provided: nil pointer")
	}
	if update.Commit == nil {
		log.Error("nil pointer to models.UpdateTransaction.Commit provided")
		return nil, errors.New("invalid models.UpdateTransaction.Commit Provided: nil pointer")
	}
	if update.Repo == nil {
		log.Errorf("updateFromHTTP::Update:Repo is unavailable %#v", update.ID)
		return nil, errors.New("repo unavailable")
	}
	cfg := config.Get()

	log.Infof("RepoBuilder::updateCommitID %d and UpdateTransactionID %d", update.Commit.ID, update.ID)

	path := filepath.Join(cfg.RepoTempPath, "upd/", strconv.FormatUint(uint64(update.RepoID), 10))
	log.Infof("RepoBuilder::path: %#v", path)
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}
	err = rb.DownloadExtractVersionRepo(update.Commit, path)
	if err != nil {
		return nil, fmt.Errorf("error downloading repo :: %s", err.Error())
	}

	if len(update.OldCommits) > 0 {
		stagePath := filepath.Join(path, "staging")
		err = os.MkdirAll(stagePath, os.FileMode(int(0755)))
		if err != nil {
			return nil, fmt.Errorf("error mkdir :: %s", err.Error())
		}
		err = os.Chdir(stagePath)
		if err != nil {
			return nil, fmt.Errorf("error chdir :: %s", err.Error())
		}

		// If there are any old commits, we need to download them all to be merged
		// into the update commit repo
		//
		// FIXME: hardcoding "repo" in here because that's how it comes from osbuild
		for _, commit := range update.OldCommits {
			rb.DownloadExtractVersionRepo(&commit, filepath.Join(stagePath, commit.OSTreeCommit))
			if err != nil {
				return nil, err
			}
			RepoPullLocalStaticDeltas(update.Commit, &commit, filepath.Join(path, "repo"), filepath.Join(stagePath, commit.OSTreeCommit, "repo"))
			if err != nil {
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
	// FIXME: Need to actually do something with the return string for Server

	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/
	log.Infof("::BuildUpdateRepo:uploader.UploadRepo: BEGIN")
	repoURL, err := rb.filesService.GetUploader().UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(update.RepoID), 10))
	log.Infof("::BuildUpdateRepo:uploader.UploadRepo: FINISH")
	log.Infof("::BuildUpdateRepo:repoURL: %#v", repoURL)
	if err != nil {
		return nil, err
	}

	update.Repo.URL = repoURL
	update.Repo.Status = models.RepoStatusSuccess
	if err := db.DB.Save(&update).Error; err != nil {
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
	path := filepath.Join(cfg.RepoTempPath, strconv.FormatUint(uint64(r.ID), 10))
	log.Debugf("RepoBuilder::path: %#v", path)
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		log.Error(err)
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}

	err = rb.DownloadExtractVersionRepo(&cmt, path)
	if err != nil {
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			log.Error(err)
		}
		log.Error(err)
		return nil, fmt.Errorf("error downloading repo and extracting repo :: %s", err.Error())
	}
	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/
	repoURL, err := rb.filesService.GetUploader().UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(r.ID), 10))
	if err != nil {
		log.Error(err)
		return nil, fmt.Errorf("error uploading repo :: %s", err.Error())
	}

	r.URL = repoURL
	r.Status = models.RepoStatusSuccess
	result := db.DB.Save(&r)
	if result.Error != nil {
		return nil, fmt.Errorf("error saving status :: %s", result.Error.Error())
	}

	return r, nil
}

// DownloadExtractVersionRepo Download and Extract the repo tarball to dest dir
func (rb *RepoBuilder) DownloadExtractVersionRepo(c *models.Commit, dest string) error {
	// ensure we weren't passed a nil pointer
	if c == nil {
		log.Error("nil pointer to models.Commit provided")
		return errors.New("invalid Commit Provided: nil pointer")
	}
	log.Debugf("DownloadExtractVersionRepo::CommitD: %d", c.ID)
	log.Debugf("DownloadExtractVersionRepo::ImageBuildTarURL: %#v", c.ImageBuildTarURL)

	// ensure the destination directory exists and then chdir there
	log.Debugf("DownloadExtractVersionRepo::dest: %#v", dest)
	err := os.MkdirAll(dest, os.FileMode(int(0755)))
	if err != nil {
		return err
	}
	err = os.Chdir(dest)
	if err != nil {
		return err
	}

	// Save the tarball to the OSBuild Hash ID and then extract it
	tarFileName := strings.Join([]string{c.ImageBuildHash, "tar"}, ".")
	log.Debugf("DownloadExtractVersionRepo::tarFileName: %#v", tarFileName)
	_, err = grab.Get(filepath.Join(dest, tarFileName), c.ImageBuildTarURL)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Debugf("Download finished::tarFileName: %#v", tarFileName)

	tarFile, err := os.Open(filepath.Join(dest, tarFileName))
	if err != nil {
		log.Errorf("Failed to open file: %s", filepath.Join(dest, tarFileName))
		log.Error(err)
		return err
	}
	err = rb.filesService.GetExtractor().Extract(tarFile, filepath.Join(dest))
	if err != nil {
		log.Errorf("Failed to untar file: %s", filepath.Join(dest, tarFileName))
		log.Error(err)
		return err
	}
	tarFile.Close()
	log.Debugf("Unpacking tarball finished::tarFileName: %#v", tarFileName)

	err = os.Remove(filepath.Join(dest, tarFileName))
	if err != nil {
		log.Errorf("Failed to remove file: %s", filepath.Join(dest, tarFileName))
		log.Error(err)
		return err
	}

	// FIXME: The repo path is hard coded because this is how it comes from
	//		  osbuild composer but we might want to revisit this later
	//
	// commit the version metadata to the current ref
	var cmd *exec.Cmd
	if c.OSTreeRef == "" {
		cfg := config.Get()
		cmd = exec.Command("ostree", "--repo", "./repo", "commit", cfg.DefaultOSTreeRef, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber))
	} else {
		cmd = exec.Command("ostree", "--repo", "./repo", "commit", c.OSTreeRef, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber))
	}
	err = cmd.Run()
	if err != nil {
		log.Error("'ostree --repo ./ commit --add-metadata-string' command failed", err)
		log.Errorf("Failed Command: %s %s %s %s %s %s %s", "ostree", "--repo", "./repo", "commit", c.OSTreeRef, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber))
	}

	return nil
}

// RepoPullLocalStaticDeltas pull local repo into the new update repo and compute static deltas
//  uprepo should be where the update commit lives, u is the update commit
//  oldrepo should be where the old commit lives, o is the commit to be merged
func RepoPullLocalStaticDeltas(u *models.Commit, o *models.Commit, uprepo string, oldrepo string) error {
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
	cmd := exec.Command("ostree", "--repo", uprepo, "pull-local", oldrepo, oldRevParse)
	err = cmd.Run()
	if err != nil {
		return err
	}

	// generate static delta
	cmd = exec.Command("ostree", "--repo", uprepo, "static-delta", "generate", "--from", oldRevParse, "--to", updateRevParse)
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
