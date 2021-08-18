package routes

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/services"
)

// MakeReposRouter defines the available actions for Repos
func MakeReposRouter(sub chi.Router) {
	cfg := config.Get()
	var server services.Server
	server = &services.FileServer{
		BasePath: "/tmp",
	}
	if cfg.BucketName != "" {
		server = services.NewS3Proxy()
	}
	sub.Post("/", CreateRepo)
	sub.Get("/{name}/*", server.ServeRepo)
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

	services.Untar(resp.Body, path)

	res := &createResponse{
		RepoURL: filepath.Join(
			chi.RouteContext(r.Context()).RoutePattern(),
			cr.Name),
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}
