package repo

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/pkg/common"
)

func MakeRouter(sub chi.Router) {
	sub.Post("/", CreateRepo)
	sub.Get("/", GetAll)
	sub.Get("/{name}/*", ServeRepo)
}

type createRequest struct {
	TarURL string
	Name   string
}

type createResponse struct {
	RepoURL string
}

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
			chi.RouteContext(r.Context()).RoutePath,
			path),
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func GetAll(w http.ResponseWriter, r *http.Request) {
}

func ServeRepo(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		http.Error(w, "repo name not provided", http.StatusBadRequest)
		return
	}
	_r := strings.Index(r.URL.Path, name)
	pathPrefix := string(r.URL.Path[:_r+len(name)])

	path := filepath.Join("/tmp", name)
	fs := http.StripPrefix(pathPrefix, http.FileServer(http.Dir(path)))
	fs.ServeHTTP(w, r)
}
