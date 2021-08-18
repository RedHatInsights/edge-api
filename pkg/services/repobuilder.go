package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/playbookdispatcher"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"

	"github.com/cavaliercoder/grab"
	log "github.com/sirupsen/logrus"
)

// RepoBuilderInterface defines the interface of a repository builder
type RepoBuilderInterface interface {
	BuildUpdateRepo(ut *models.UpdateTransaction) (*models.UpdateTransaction, error)
	ImportRepo(r *models.Repo) (*models.Repo, error)
}

// RepoBuilder is the implementation of a RepoBuilderInterface
type RepoBuilder struct {
	ctx context.Context
}

// InitRepoBuilder initializes the repository builder in this package
func InitRepoBuilder(ctx context.Context) *RepoBuilder {
	return &RepoBuilder{ctx: ctx}
}

// BuildUpdateRepo build an update repo with the set of commits all merged into a single repo
// with static deltas generated between them all
func (rb *RepoBuilder) BuildUpdateRepo(ut *models.UpdateTransaction) (*models.UpdateTransaction, error) {
	log.Infof("Repobuilder::BuildUpdateRepo:: Begin")
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
	result := db.DB.Preload("Devices").Preload("DispatchRecords").Preload("OldCommits").First(&update, ut.ID)
	if result.Error != nil {
		return nil, result.Error
	}
	update.Status = models.UpdateStatusCreated
	db.DB.Save(&update)

	log.Infof("RepoBuilder::updateCommit: %#v", ut.Commit)

	path := filepath.Join(cfg.RepoTempPath, strconv.FormatUint(uint64(ut.RepoID), 10))
	log.Infof("RepoBuilder::path: %#v", path)
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
	log.Infof("::BuildUpdateRepo:uploader.UploadRepo: BEGIN")
	repoURL, err := uploader.UploadRepo(filepath.Join(path, "repo"), strconv.FormatUint(uint64(ut.RepoID), 10))
	log.Infof("::BuildUpdateRepo:uploader.UploadRepo: FINISH")
	log.Infof("::BuildUpdateRepo:repoURL: %#v", repoURL)
	if err != nil {
		return nil, err
	}

	update.Status = models.UpdateStatusSuccess
	if update.Repo == nil {
		//  Check for the existence of a Repo that already has this commit and don't duplicate
		var repo *models.Repo
		repo, err = common.GetRepoByCommitID(update.CommitID)
		if err == nil {
			update.Repo = repo
		} else {
			if err.Error() != "record not found" {
				log.Errorf("updateFromHTTP::GetRepoByCommitID::repo: %#v, %#v", repo, err)
			} else {
				log.Infof("Old Repo not found in database for CommitID, creating new one: %d", update.CommitID)
				update.Repo = &models.Repo{
					Commit: update.Commit,
				}
			}
		}
	}
	update.Repo.URL = repoURL
	update.Repo.Status = models.RepoStatusSuccess
	db.DB.Save(&update)

	// FIXME - implement playbook dispatcher scheduling
	// 1. Create template Playbook
	// 2. Upload templated playbook
	var remoteInfo TemplateRemoteInfo
	remoteInfo.RemoteURL = update.Repo.URL
	remoteInfo.RemoteName = "main-test"
	remoteInfo.ContentURL = update.Repo.URL
	remoteInfo.UpdateTransaction = int(update.ID)
	remoteInfo.GpgVerify = "true"
	playbookURL, err := WriteTemplate(remoteInfo, update.Account)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	log.Debugf("playbooks:WriteTemplate: %#v", playbookURL)
	// 3. Loop through all devices in UpdateTransaction
	dispatchRecords := update.DispatchRecords
	for _, device := range update.Devices {
		var updateDevice *models.Device
		updateDevice, err = common.GetDeviceByUUID(device.UUID)
		if err != nil {
			log.Errorf("Error on common.GetDeviceByUUID: %#v ", err.Error())
			return nil, err
		}
		// Create new &DispatcherPayload{}
		payloadDispatcher := playbookdispatcher.DispatcherPayload{
			Recipient:   device.RHCClientID,
			PlaybookURL: playbookURL,
			Account:     update.Account,
		}
		log.Infof("Call Execute Dispatcher: : %#v", payloadDispatcher)
		client := playbookdispatcher.InitClient(rb.ctx)
		exc, err := client.ExecuteDispatcher(payloadDispatcher)

		if err != nil {
			log.Errorf("Error on playbook-dispatcher-executuin: %#v ", err)
			return nil, err
		}
		for _, excPlaybook := range exc {
			if excPlaybook.StatusCode == http.StatusCreated {
				device.Connected = true
				dispatchRecord := &models.DispatchRecord{
					Device:               updateDevice,
					PlaybookURL:          repoURL,
					Status:               models.DispatchRecordStatusCreated,
					PlaybookDispatcherID: excPlaybook.PlaybookDispatcherID,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			} else {
				device.Connected = false
				dispatchRecord := &models.DispatchRecord{
					Device:      updateDevice,
					PlaybookURL: repoURL,
					Status:      models.DispatchRecordStatusError,
				}
				dispatchRecords = append(dispatchRecords, *dispatchRecord)
			}

		}
		update.DispatchRecords = dispatchRecords
	}
	db.DB.Save(&update)
	log.Infof("Repobuild::ends: update record %#v ", update)
	return &update, nil
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
		r.Status = models.RepoStatusError
		result := db.DB.Save(&r)
		if result.Error != nil {
			log.Error(err)
		}
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

	r.URL = repoURL
	r.Status = models.RepoStatusSuccess
	result := db.DB.Save(&r)
	if result.Error != nil {
		return nil, result.Error
	}

	return r, nil
}

// DownloadExtractVersionRepo Download and Extract the repo tarball to dest dir
func DownloadExtractVersionRepo(c *models.Commit, dest string) error {
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
