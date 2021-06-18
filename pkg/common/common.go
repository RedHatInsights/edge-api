package common

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/errors"
	"github.com/redhatinsights/platform-go-middlewares/identity"
)

const (
	PaginationKey = 1
	defaultLimit  = 100
	defaultOffset = 0
)

// GetAccount from http request header
func GetAccount(r *http.Request) (string, error) {
	if config.Get() != nil {
		if config.Get().Debug {
			return "0000000", nil
		}

		ident := identity.Get(r.Context())
		if ident.Identity.AccountNumber != "" {
			return ident.Identity.AccountNumber, nil
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
		defer file.Close()
		_, err = io.Copy(file, tarReader)
		if err != nil {
			return err
		}
	}
	return nil
}

type Pagination struct {
	Limit  int
	Offset int
}

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

func GetPagination(r *http.Request) Pagination {
	pagination, ok := r.Context().Value(PaginationKey).(Pagination)
	if !ok {
		return Pagination{Offset: defaultOffset, Limit: defaultLimit}
	}
	return pagination
}
