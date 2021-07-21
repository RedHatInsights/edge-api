package commits

import (
	"errors"
	"fmt"

	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/cavaliercoder/grab"
	log "github.com/sirupsen/logrus"
)

var RepoBuilderInstance RepoBuilderInterface

// InitRepoBuilder initializes the repository builder in this package
func InitRepoBuilder() {
	RepoBuilderInstance = &RepoBuilder{}
}

// RepoBuilderInterface defines the interface of a repository builder
type RepoBuilderInterface interface {
	BuildUpdateRepo(ut *models.UpdateTransaction) (*models.UpdateTransaction, error)
	ImportRepo(r *models.Repo) (*models.Repo, error)
}

// RepoBuilder is the implementation of a RepoBuilderInterface
type RepoBuilder struct{}

// BuildUpdateRepo build an update repo with the set of commits all merged into a single repo
// with static deltas generated between them all
func (rb *RepoBuilder) BuildUpdateRepo(ut *models.UpdateTransaction) (*models.UpdateTransaction, error) {
	if ut == nil {
		log.Error("nil pointer to models.UpdateTransaction provided")
		return &models.UpdateTransaction{}, errors.New("Invalid models.UpdateTransaction Provided: nil pointer")
	}
	if ut.Commit == nil {
		log.Error("nil pointer to models.UpdateTransaction.Commit provided")
		return &models.UpdateTransaction{}, errors.New("Invalid models.UpdateTransaction.Commit Provided: nil pointer")
	}
	cfg := config.Get()

	var update models.UpdateTransaction
	result := db.DB.First(&update, ut.ID)
	if result.Error != nil {
		return nil, result.Error
	}
	update.Status = models.UpdateStatusCreated
	db.DB.Save(&update)

	log.Debugf("RepoBuilder::updateCommit: %#v", ut.Commit)

	path := filepath.Join(cfg.RepoTempPath, strconv.FormatUint(uint64(ut.RepoID), 10))
	log.Debugf("RepoBuilder::path: %#v", path)
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}
	err = DownloadExtractVersionRepo(ut.Commit, path)
	if err != nil {
		return nil, err
	}

	if len(ut.OldCommits) > 0 {
		stagePath := filepath.Join(path, "staging")
		err = os.MkdirAll(stagePath, os.FileMode(int(0755)))
		if err != nil {
			return nil, err
		}
		err = os.Chdir(stagePath)
		if err != nil {
			return nil, err
		}

		// If there are any old commits, we need to download them all to be merged
		// into the update commit repo
		//
		// FIXME: hardcoding "repo" in here because that's how it comes from osbuild
		for _, commit := range ut.OldCommits {
			DownloadExtractVersionRepo(&commit, filepath.Join(stagePath, commit.OSTreeCommit))
			if err != nil {
				return nil, err
			}
			RepoPullLocalStaticDeltas(ut.Commit, &commit, filepath.Join(path, "repo"), filepath.Join(stagePath, commit.OSTreeCommit, "repo"))
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

	var uploader Uploader
	uploader = &FileUploader{
		BaseDir: path,
	}
	if cfg.BucketName != "" {
		uploader = NewS3Uploader()
	}
	// FIXME: Need to actually do something with the return string for Server

	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/
	log.Debug("::BuildUpdateRepo:uploader.UploadRepo: BEGIN")
	repoURL, err := uploader.UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(ut.RepoID), 10))
	log.Debug("::BuildUpdateRepo:uploader.UploadRepo: FINISH")
	log.Debugf("::BuildUpdateRepo:repoURL: %#v", repoURL)
	if err != nil {
		return nil, err
	}

	var updateDone models.UpdateTransaction
	result = db.DB.First(&updateDone, ut.ID)
	if result.Error != nil {
		return nil, result.Error
	}
	updateDone.Status = models.UpdateStatusSuccess
	if updateDone.Repo == nil {
		//  Check for the existence of a Repo that already has this commit and don't duplicate
		var repo *models.Repo
		repo, err = common.GetRepoByCommitID(update.Commit.ID)
		if err == nil {
			update.Repo = repo
		} else {
			if !(err.Error() == "record not found") {
				log.Errorf("updateFromHTTP::GetRepoByCommitID::repo: %#v, %#v", repo, err)
			} else {
				log.Infof("Old Repo not found in database for CommitID, creating new one: %d", update.Commit.ID)
				updateDone.Repo = &models.Repo{}
				updateDone.Repo.Commit = update.Commit
			}
		}

	}
	updateDone.Repo.URL = repoURL
	db.DB.Save(&updateDone)

	return &updateDone, nil
}

// ImportRepo (unpack and upload) a single repo
func (rb *RepoBuilder) ImportRepo(r *models.Repo) (*models.Repo, error) {
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
	err = DownloadExtractVersionRepo(r.Commit, path)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	var uploader Uploader
	uploader = &FileUploader{
		BaseDir: path,
	}
	if cfg.BucketName != "" {
		uploader = NewS3Uploader()
	}

	// NOTE: This relies on the file path being cfg.RepoTempPath/models.Repo.ID/
	repoURL, err := uploader.UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(r.ID), 10))
	if err != nil {
		log.Error(err)
		return nil, err
	}

	var repo models.Repo
	result := db.DB.First(&repo, r.ID)
	if result.Error != nil {
		log.Error(err)
		return nil, result.Error
	}
	repo.URL = repoURL
	db.DB.Save(&repo)

	return &repo, nil
}

// DownloadExtractVersionRepo Download and Extract the repo tarball to dest dir
func DownloadExtractVersionRepo(c *models.Commit, dest string) error {
	// ensure we weren't passed a nil pointer
	if c == nil {
		log.Error("nil pointer to models.Commit provided")
		return errors.New("Invalid Commit Provided: nil pointer")
	}

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
	err = common.Untar(tarFile, filepath.Join(dest))
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
