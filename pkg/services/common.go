package services

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

const (
	TrailingSlashIndex = 1
)

func getNameAndPrefix(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "name")
	log.Debugf("getNameAndPrefix::name: %#v", name)
	if name == "" {
		return "", "", fmt.Errorf("repo name not provided")
	}
	pathPrefix := getPathPrefix(r.URL.Path, name)
	return name, pathPrefix, nil
}

func getPathPrefix(path string, name string) string {
	_r := strings.Index(path, "/"+name+"/")
	log.Debugf("getNameAndPrefix::_r: %#v", _r)
	pathPrefix := string(path[:_r+TrailingSlashIndex])
	log.Debugf("getNameAndPrefix::pathPrefix: %#v", pathPrefix)
	return pathPrefix
}
