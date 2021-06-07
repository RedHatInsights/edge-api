package updates

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"

	"github.com/redhatinsights/edge-api/pkg/commits"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"gorm.io/gorm"

	"github.com/cavaliercoder/grab"
)

// Update reporesents the combination of an OSTree commit and a set of Inventory
// hosts that need to have the commit deployed to them
//
// This will ultimately kick off a transaction where the old version(s) of
// OSTree commit that are currently deployed onto those devices are combined
// with the new commit into a new OSTree repo, static deltas are computed, and
// then the result is stored in a way that can be served(proxied) by a
// Server (pkg/repo/server.go).
type UpdateRecord struct {
	gorm.Model
	UpdateCommit   *commits.Commit
	OldCommits     []*commits.Commit
	InventoryHosts []string
}

func updateFromReadCloser(rc io.ReadCloser) (*UpdateRecord, error) {
	defer rc.Close()
	var update UpdateRecord
	err := json.NewDecoder(rc).Decode(&update)
	return &update, err
}

// MakeRouter adds support for operations on commits
func MakeRouter(sub chi.Router) {
	sub.Post("/", Add)
	sub.Get("/", GetAll)
	sub.Route("/{updateID}", func(r chi.Router) {
		r.Use(UpdateCtx)
		r.Get("/", GetByID)
		r.Put("/", Update)
	})
}

// This provides type safety in the context object for our "update" key.  We
// _could_ use a string but we shouldn't just in case someone else decides that
// "update" would make the perfect key in the context object.  See the
// documentation: https://golang.org/pkg/context/#WithValue for further
// rationale.
type key int

const updateKey key = 0

// UpdateCtx is a handler for Update requests
func UpdateCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var update UpdateRecord
		account, err := common.GetAccount(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if updateID := chi.URLParam(r, "updateID"); updateID != "" {
			id, err := strconv.Atoi(updateID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			result := db.DB.Where("account = ?", account).First(&update, id)
			if result.Error != nil {
				http.Error(w, result.Error.Error(), http.StatusNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), updateKey, &update)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// Add an object to the database for an account
func Add(w http.ResponseWriter, r *http.Request) {

	update, err := updateFromReadCloser(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.DB.Create(&update)
}

// GetAll update objects from the database for an account
func GetAll(w http.ResponseWriter, r *http.Request) {
	var updates []UpdateRecord
	account, err := common.GetAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// FIXME - need to sort out how to get this query to be against commit.account
	result := db.DB.Where("account = ?", account).Find(&updates)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(&updates)
}

// GetByID obtains an update from the database for an account
func GetByID(w http.ResponseWriter, r *http.Request) {
	if update := getUpdate(w, r); update != nil {
		json.NewEncoder(w).Encode(update)
	}
}

// Update a update object in the database for an an account
func Update(w http.ResponseWriter, r *http.Request) {
	update := getUpdate(w, r)
	if update == nil {
		return
	}

	incoming, err := updateFromReadCloser(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	incoming.ID = update.ID
	incoming.CreatedAt = now
	incoming.UpdatedAt = now
	db.DB.Save(&incoming)

	json.NewEncoder(w).Encode(incoming)
}

func getUpdate(w http.ResponseWriter, r *http.Request) *UpdateRecord {
	ctx := r.Context()
	update, ok := ctx.Value(updateKey).(*UpdateRecord)
	if !ok {
		http.Error(w, "must pass id", http.StatusBadRequest)
		return nil
	}
	return update
}

// CreateRepo creates a repository from a tar file
func RepoBuilder(ur *UpdateRecord) (string, error) {

	path := filepath.Join("/tmp/update/", strconv.FormatUint(uint64(ur.ID), 10))
	err := os.MkdirAll(path, os.FileMode(int(0755)))
	if err != nil {
		return path, err
	}
	err := os.Chdir(path)
	if err != nil {
		return path, err
	}

	stagePath := filepath.Join(path, "staging")
	err := os.MkdirAll(stagePath, os.FileMode(int(0755)))
	if err != nil {
		return stagePath, err
	}
	err := os.Chdir(stagePath)
	if err != nil {
		return stagePath, err
	}

	for _, commit := range ur.OldCommits {
		oldCommitPath := filepath.Join(stagePath, commit.OSTreeCommit)
		err := os.MkdirAll(oldCommitPath, os.FileMode(int(0755)))
		if err != nil {
			return "", err
		}
		err := os.Chdir(oldCommitPath)
		if err != nil {
			return oldCommitPath, err
		}

		// Save the tarball to the OSBuild Hash ID
		resp, err := grab.Get(strings.Join([]string{commit.ImageBuildHash, "tar"}, "."), commit.ImageBuildTarURL)
		if err != nil {
			log.Fatal(err)
		}

		tarFile, err := os.Open(filepath.Join(commit.ImageBuildHash, ".tar"))
		if err != nil {
			return "", err
		}
		defer tarFile.Close()
		common.Untar(tarFile)

		oldCommitRepoPath := filepath.Join(oldCommitPath, commit.OSTreeCommit, "repo")
		err := os.Chdir(oldCommitRepoPath)
		if err != nil {
			return oldCommitRepoPath, err
		}

	}
	err := os.Chdir(stagePath)
	if err != nil {
		return stagePath, err
	}

	updateCommitPath := filepath.Join(stagePath, ur.UpdateCommit.OSTreeCommit)
	err := os.MkdirAll(updateCommitPath, os.FileMode(int(0755)))
	if err != nil {
		return "", err
	}
	resp, err := grab.Get(".", commit.ImageBuildTarURL)
	if err != nil {
		log.Fatal(err)
	}

	/*
		# directory setup
		mkdir tmp
		cd tmp
		mkdir tar1 tar2

		# grab first tarball
		pushd tar1
		curl -LO https://builds.coreos.fedoraproject.org/prod/streams/stable/builds/33.20210412.3.0/x86_64/fedora-coreos-33.20210412.3.0-ostree.x86_64.tar
		tar xf fedora-coreos-33.20210412.3.0-ostree.x86_64.tar
		ostree --repo=./ refs
		popd

		# grab second tarball
		cd tar2
		curl -LO https://builds.coreos.fedoraproject.org/prod/streams/stable/builds/34.20210427.3.0/x86_64/fedora-coreos-34.20210427.3.0-ostree.x86_64.tar
		tar xf fedora-coreos-34.20210427.3.0-ostree.x86_64.tar
		ostree --repo=./ refs

		# combine into one repo
		ostree --repo=./ pull-local ../tar1/ 33.20210412.3.0

		# now both are in the repo in the tar2 directory. compare commits
		rpm-ostree --repo=./ db diff 33.20210412.3.0 34.20210427.3.0

		# now generate a static delta
		ostree --repo=./ static-delta generate --from=33.20210412.3.0 --to=34.20210427.3.0

		# static delta files are under the `deltas` directory
	*/

	if len(ur.OldCommits) > 0 {
		// FIXME : need to deal with this
	}

	return path, nil
}
