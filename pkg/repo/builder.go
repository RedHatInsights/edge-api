package repo

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/updates"
	"gorm.io/gorm"
)

// LocalRepo is the representation of the OSTree repo built locally into a
// directory such that we can then push it to S3 compatible storage.
type LocalRepo struct {
	gorm.Model
	UpdateCommit string // The new update target commmit
	// A slice of old commits that we need to pull and
	// generate static deltas for
	OldCommits []string
}

type RepoMode string

const (
	BARE           RepoMode = "bare"
	BARE_USER      RepoMode = "bare-user"
	BARE_USER_ONLY RepoMode = "bare-user-only"
	ARCHIVE        RepoMode = "archive"
)

func (mode RepoMode) String() string {
	return string(mode)
}

func (repo *LocalRepo) Path() string {
	return repo.path
}

func (repo *LocalRepo) Init(mode RepoMode) error {
	err := os.MkdirAll(repo.path, 0700)
	if err != nil {
		return err
	}

	cmd := exec.Command("ostree", "init", "--repo", repo.path, "--mode", mode.String())
	err = cmd.Run()

	return err
}

func (repo *LocalRepo) GetParentCommit(commit string) (string, error) {
	ref := fmt.Sprintf("%s^", commit)
	return repo.RevParse(ref)
}

func (repo *LocalRepo) PullLocal(source string, ref string) error {
	target := repo.path
	cmd := exec.Command("ostree", "pull-local", source, "--repo", target, ref)
	err := cmd.Run()

	return err
}

func (repo *LocalRepo) RevParse(ref string) (string, error) {
	target := repo.path
	cmd := exec.Command("ostree", "rev-parse", "--repo", target, ref)

	var res bytes.Buffer
	cmd.Stdout = &res

	err := cmd.Run()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(res.String()), nil
}

func (repo *LocalRepo) UpdateSummary() error {
	target := repo.path
	cmd := exec.Command("ostree", "summary", "-u", "--repo", target)
	err := cmd.Run()

	return err
}

// CreateRepo creates a repository from a tar file
func RepoBuilder(ur *updates.UpdateRecord) (string, error) {

	path := filepath.Join("/tmp/repobuilder/", rbr.UpdateCommit)
	err := os.MkdirAll(path)
	if err != nil {
		http.Error(w, strings.join("Unable to create ", path), http.StatusInternalServerError)
	}

	if len(rbr.OldCommits) > 0 {
		// FIXME : need to deal with this
	}

	return path, nil
}
