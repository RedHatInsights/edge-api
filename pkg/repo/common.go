package repo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	log "github.com/sirupsen/logrus"
)

func getNameAndPrefix(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "name")
	log.Debugf("getNameAndPrefix::name: %#v", name)
	if name == "" {
		return "", "", fmt.Errorf("repo name not provided")
	}
	_r := strings.Index(r.URL.Path, name)
	log.Debugf("getNameAndPrefix::_r: %#v", _r)
	pathPrefix := string(r.URL.Path[:_r])
	log.Debugf("getNameAndPrefix::pathPrefix: %#v", pathPrefix)
	return name, pathPrefix, nil
}
