package commits

import (
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
	BuildRepo(ur *models.UpdateTransaction) (*models.UpdateTransaction, error)
	ImportRepo(c *models.Commit) error
}

// RepoBuilder is the implementation of a RepoBuilderInterface
type RepoBuilder struct{}

// BuildRepo build an update repo with the set of commits all merged into a single repo
// with static deltas generated between them all
func (rb *RepoBuilder) BuildRepo(ur *models.UpdateTransaction) (*models.UpdateTransaction, error) {
	cfg := config.Get()

	var update models.UpdateTransaction
	db.DB.First(&update, ur.ID)
	update.Status = models.UpdateStatusCreated
	db.DB.Save(&update)

	log.Debugf("RepoBuilder::updateCommit: %#v", ur.Commit)

	path := filepath.Join(cfg.UpdateTempPath, strconv.FormatUint(uint64(ur.ID), 10))
	log.Debugf("RepoBuilder::path: %#v", path)
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		return nil, err
	}
	err = os.Chdir(path)
	if err != nil {
		return nil, err
	}
	DownloadExtractVersionRepo(ur.Commit, path)

	if len(ur.OldCommits) > 0 {
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
		for _, commit := range ur.OldCommits {
			DownloadExtractVersionRepo(&commit, filepath.Join(stagePath, commit.OSTreeCommit))
			RepoPullLocalStaticDeltas(ur.Commit, &commit, filepath.Join(path, "repo"), filepath.Join(stagePath, commit.OSTreeCommit, "repo"))
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

	// NOTE: This relies on the file path being cfg.UpdateTempPath/models.Repo.ID/
	repoURL, err := uploader.UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(ur.Commit.RepoID), 10))
	if err != nil {
		return nil, err
	}

	var updateDone models.UpdateTransaction
	db.DB.First(&updateDone, ur.ID)
	updateDone.Status = models.UpdateStatusSuccess
	updateDone.Commit.Repo.URL = repoURL
	db.DB.Save(&updateDone)

	return &updateDone, nil
}

// ImportRepo (unpack and upload) a single repo
func (rb *RepoBuilder) ImportRepo(c *models.Commit) error {
	cfg := config.Get()

	path := filepath.Join(cfg.UpdateTempPath, strconv.FormatUint(uint64(c.RepoID), 10))
	log.Debugf("RepoBuilder::path: %#v", path)
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		return err
	}
	err = os.Chdir(path)
	if err != nil {
		return err
	}
	DownloadExtractVersionRepo(c, path)

	var uploader Uploader
	uploader = &FileUploader{
		BaseDir: path,
	}
	if cfg.BucketName != "" {
		uploader = NewS3Uploader()
	}

	// NOTE: This relies on the file path being cfg.UpdateTempPath/models.Repo.ID/
	repoURL, err := uploader.UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(c.RepoID), 10))
	if err != nil {
		return err
	}

	var repo models.Repo
	db.DB.First(&repo, c.RepoID)
	repo.Status = models.UpdateStatusSuccess
	repo.URL = repoURL
	db.DB.Save(&repo)

	return nil
}

// DownloadExtractVersionRepo Download and Extract the repo tarball to dest dir
func DownloadExtractVersionRepo(c *models.Commit, dest string) error {
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
		return err
	}
	log.Debugf("Download finished::tarFileName: %#v", tarFileName)

	tarFile, err := os.Open(filepath.Join(dest, tarFileName))
	if err != nil {
		return err
	}
	err = common.Untar(tarFile, filepath.Join(dest))
	if err != nil {
		return err
	}
	tarFile.Close()

	err = os.Remove(filepath.Join(dest, tarFileName))
	if err != nil {
		return err
	}

	// FIXME: The repo path is hard coded because this is how it comes from
	//		  osbuild composer but we might want to revisit this later
	//
	// commit the version metadata to the current ref
	cmd := exec.Command("ostree", "--repo", "./repo", "commit", c.OSTreeRef, "--add-metadata-string", fmt.Sprintf("version=%s.%d", c.BuildDate, c.BuildNumber))
	err = cmd.Run()
	if err != nil {
		return err
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
