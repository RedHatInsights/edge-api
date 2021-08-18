package services

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/platform-go-middlewares/identity"
	log "github.com/sirupsen/logrus"
)

type paginationContextKey int

const (
	// PaginationKey is used to store pagination data in request context
	PaginationKey paginationContextKey = 1
	defaultLimit  int                  = 100
	defaultOffset int                  = 0
	// TrailingSlashIndex is the index used to remove trailing slashs from path prefixes
	TrailingSlashIndex int = 1
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

// GetAccount from http request header
func GetAccount(r *http.Request) (string, error) {
	if config.Get() != nil {
		if !config.Get().Auth {
			return "0000000", nil
		}
		if r.Context().Value(identity.Key) != nil {
			ident := identity.Get(r.Context())
			if ident.Identity.AccountNumber != "" {
				return ident.Identity.AccountNumber, nil
			}
		}
	}
	return "", fmt.Errorf("cannot find account number")

}

// StatusOK returns a simple 200 status code
func StatusOK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

// Untar file to destination path
func Untar(rc io.ReadCloser, dst string) error {
	defer rc.Close()
	tarReader := tar.NewReader(rc)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		path := filepath.Join(dst, header.Name)
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(path, info.Mode()); err != nil {
				return err
			}
			continue
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode())
		if err != nil {
			return err
		}
		_, err = io.Copy(file, tarReader)
		if err != nil {
			file.Close()
			return err
		}
		file.Close()
	}
	return nil
}

// Pagination represents pagination parameters
type Pagination struct {
	// Limit represents how many items to return
	Limit int
	// Offset represents from what item to start
	Offset int
}

// Paginate is a middleware to get pagination params from the request and store it in
// the request context. If no pagination was set in the request URL search parameters
// they are set to default (see defaultLimit and defaultOffset).
// To read pagination parameters from context use PaginationKey.
func Paginate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pagination := Pagination{Limit: defaultLimit, Offset: defaultOffset}
		if val, ok := r.URL.Query()["limit"]; ok {
			valInt, err := strconv.Atoi(val[0])
			if err != nil {
				errors.NewBadRequest(err.Error())
				return
			}
			pagination.Limit = valInt
		}
		if val, ok := r.URL.Query()["offset"]; ok {
			valInt, err := strconv.Atoi(val[0])
			if err != nil {
				errors.NewBadRequest(err.Error())
				return
			}
			pagination.Offset = valInt
		}
		ctx := context.WithValue(r.Context(), PaginationKey, pagination)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetPagination is a function helper to get pagination parameters from the request context.
// In case the router doesn't use Paginate before serving the request we still return
// the default parameters: defaultOffset, defaultLimit
func GetPagination(r *http.Request) Pagination {
	pagination, ok := r.Context().Value(PaginationKey).(Pagination)
	if !ok {
		return Pagination{Offset: defaultOffset, Limit: defaultLimit}
	}
	return pagination
}
