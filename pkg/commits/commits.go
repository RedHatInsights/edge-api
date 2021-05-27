package commits

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	"gorm.io/gorm"
)

// Commit represents an OSTree commit from image builder
type Commit struct {
	gorm.Model
	Name                 string
	Account              string
	ImageBuildHash       string
	ImageBuildParentHash string
	ImageBuildTarURL     string
	OSTreeCommit         string
	OSTreeParentCommit   string
	OSTreeRef            string
	BuildDate            string
	BuildNumber          uint
	BlueprintToml        string
	NEVRAManifest        string
	Arch                 string
}

func commitFromReadCloser(rc io.ReadCloser) (*Commit, error) {
	defer rc.Close()
	var commit Commit
	err := json.NewDecoder(rc).Decode(&commit)
	return &commit, err
}

// MakeRouter adds support for operations on commits
func MakeRouter(sub chi.Router) {
	sub.Post("/", Add)
	sub.Get("/", GetAll)
	sub.Route("/{commitId}", func(r chi.Router) {
		r.Use(CommitCtx)
		r.Get("/", GetByID)
		r.Get("/repo/*", ServeRepo)
		r.Put("/", Update)
		r.Patch("/", Patch)
	})
}

// This provides type safety in the context object for our "commit" key.  We
// _could_ use a string but we shouldn't just in case someone else decides that
// "commit" would make the perfect key in the context object.  See the
// documentation: https://golang.org/pkg/context/#WithValue for further
// rationale.
type key int

const commitKey key = 0

// CommitCtx is a handler for Commit requests
func CommitCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var commit Commit
		account, err := getAccount(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if commitID := chi.URLParam(r, "commitId"); commitID != "" {
			id, err := strconv.Atoi(commitID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			result := db.DB.Where("account = ?", account).First(&commit, id)
			if result.Error != nil {
				http.Error(w, result.Error.Error(), http.StatusNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), commitKey, &commit)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

func getAccount(r *http.Request) (string, error) {

	if config.Get().Debug {
		return "0000000", nil
	}

	ident := identity.Get(r.Context())
	if ident.Identity.AccountNumber != "" {
		return ident.Identity.AccountNumber, nil
	}
	return "", fmt.Errorf("cannot find account number")

}

// Add a commit object to the database for an account
func Add(w http.ResponseWriter, r *http.Request) {

	commit, err := commitFromReadCloser(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	commit.Account, err = getAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	db.DB.Create(&commit)
}

// GetAll commit objects from the database for an account
func GetAll(w http.ResponseWriter, r *http.Request) {
	var commits []Commit
	account, err := getAccount(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := db.DB.Where("account = ?", account).Find(&commits)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(&commits)
}

// GetByID obtains a commit from the database for an account
func GetByID(w http.ResponseWriter, r *http.Request) {
	if commit := getCommit(w, r); commit != nil {
		json.NewEncoder(w).Encode(commit)
	}
}

// Update a commit object in the database for an an account
func Update(w http.ResponseWriter, r *http.Request) {
	commit := getCommit(w, r)
	if commit == nil {
		return
	}

	incoming, err := commitFromReadCloser(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	incoming.ID = commit.ID
	incoming.CreatedAt = now
	incoming.UpdatedAt = now
	incoming.Account = commit.Account
	db.DB.Save(&incoming)

	json.NewEncoder(w).Encode(incoming)
}

func applyPatch(commit *Commit, incoming *Commit) {
	if incoming.Name != "" {
		commit.Name = incoming.Name
	}

	if incoming.ImageBuildHash != "" {
		commit.ImageBuildHash = incoming.ImageBuildHash
	}

	if incoming.ImageBuildParentHash != "" {
		commit.ImageBuildParentHash = incoming.ImageBuildParentHash
	}

	if incoming.ImageBuildTarURL != "" {
		commit.ImageBuildTarURL = incoming.ImageBuildTarURL
	}

	if incoming.OSTreeCommit != "" {
		commit.OSTreeCommit = incoming.OSTreeCommit
	}

	if incoming.OSTreeParentCommit != "" {
		commit.OSTreeParentCommit = incoming.OSTreeParentCommit
	}

	if incoming.OSTreeRef != "" {
		commit.OSTreeRef = incoming.OSTreeRef
	}

	if incoming.BuildDate != "" {
		commit.BuildDate = incoming.BuildDate
	}

	if incoming.BuildNumber != 0 {
		commit.BuildNumber = incoming.BuildNumber
	}

	if incoming.BlueprintToml != "" {
		commit.BlueprintToml = incoming.BlueprintToml
	}

	if incoming.NEVRAManifest != "" {
		commit.NEVRAManifest = incoming.NEVRAManifest
	}

	if incoming.Arch != "" {
		commit.Arch = incoming.Arch
	}

	commit.UpdatedAt = time.Now()
}

// Patch a commit object in the database for an account
func Patch(w http.ResponseWriter, r *http.Request) {
	commit := getCommit(w, r)
	if commit == nil {
		return
	}

	incoming, err := commitFromReadCloser(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	applyPatch(commit, incoming)

	db.DB.Save(&commit)
	json.NewEncoder(w).Encode(commit)
}

// ServeRepo creates a file server for a commit object
func ServeRepo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	commit, ok := ctx.Value(commitKey).(*Commit)
	if !ok {
		http.Error(w, "must pass id", http.StatusBadRequest)
		return
	}

	path := filepath.Join("/tmp", commit.ImageBuildHash)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		resp, err := http.Get(commit.ImageBuildTarURL)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		common.Untar(resp.Body, path)
	}

	_r := strings.Index(r.URL.Path, "/repo")

	pathPrefix := string(r.URL.Path[:_r+5])
	fs := http.StripPrefix(pathPrefix, http.FileServer(http.Dir(path)))
	fs.ServeHTTP(w, r)

}

func getCommit(w http.ResponseWriter, r *http.Request) *Commit {
	ctx := r.Context()
	commit, ok := ctx.Value(commitKey).(*Commit)
	if !ok {
		http.Error(w, "must pass id", http.StatusBadRequest)
		return nil
	}
	return commit
}
