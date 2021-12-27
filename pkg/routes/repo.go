package routes

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

const (
	TrailingSlashIndex = 1
)

// MakeReposRouter defines the available actions for Repos
func MakeReposRouter(sub chi.Router) {
	sub.Get("/{repoUrl}", ServeRepo)
}

func getPathPrefix(path string, name string) string {
	_r := strings.Index(path, "/"+name+"/")
	log.Debugf("getNameAndPrefix::_r: %#v", _r)
	pathPrefix := string(path[:_r+TrailingSlashIndex])
	log.Debugf("getNameAndPrefix::pathPrefix: %#v", pathPrefix)
	return pathPrefix
}

func getNameAndPrefix(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "name")
	log.Debugf("getNameAndPrefix::name: %#v", name)
	if name == "" {
		return "", "", fmt.Errorf("repo name not provided")
	}
	pathPrefix := getPathPrefix(r.URL.Path, name)
	return name, pathPrefix, nil
}

func ServeRepo(w http.ResponseWriter, r *http.Request) {
	name, prefix, err := getNameAndPrefix(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := filepath.Join("/tmp", name)
	log.Debugf("FileServer::ServeRepo::path: %#v", path)
	fs := http.StripPrefix(prefix, http.FileServer(http.Dir(path)))
	fs.ServeHTTP(w, r)
}
