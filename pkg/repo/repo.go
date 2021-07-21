package repo

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/common"
	"github.com/redhatinsights/edge-api/pkg/db"
	"github.com/redhatinsights/edge-api/pkg/models"
)

//MakeRouter defines the available actions for Repos
func MakeRouter(server Server) func(sub chi.Router) {
	return func(sub chi.Router) {
		sub.Post("/", CreateRepo)
		sub.Get("/", GetAll)
		sub.Get("/{name}/*", server.ServeRepo)
	}
}

type createRequest struct {
	TarURL string
	Name   string
}

type createResponse struct {
	RepoURL string
}

// CreateRepo creates a repository from a tar file
func CreateRepo(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var cr createRequest
	err := json.NewDecoder(r.Body).Decode(&cr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if cr.TarURL == "" {
		http.Error(w, "tarUrl must be set", http.StatusBadRequest)
		return
	}

	if cr.Name == "" {
		cr.Name = "default" // should be randomized?
	}

	resp, err := http.Get(cr.TarURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	path := filepath.Join("/tmp", cr.Name)

	common.Untar(resp.Body, path)

	res := &createResponse{
		RepoURL: filepath.Join(
			chi.RouteContext(r.Context()).RoutePattern(),
			cr.Name),
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

// GetAll repositories. Mostly used for the development team to debug.
func GetAll(w http.ResponseWriter, r *http.Request) {
	var repos []models.Repo
	account, err := common.GetAccount(r)
	pagination := common.GetPagination(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result := db.DB.Limit(pagination.Limit).Offset(pagination.Offset).Joins("Commit").Where("commit.account = ?", account).Find(&repos)
	if result.Error != nil {
		http.Error(w, result.Error.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(&repos)
}
