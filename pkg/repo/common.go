package repo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
)

func getNameAndPrefix(r *http.Request) (string, string, error) {
	name := chi.URLParam(r, "name")
	if name == "" {
		return "", "", fmt.Errorf("repo name not provided")
	}
	_r := strings.Index(r.URL.Path, name)
	pathPrefix := string(r.URL.Path[:_r+len(name)])
	return name, pathPrefix, nil
}
