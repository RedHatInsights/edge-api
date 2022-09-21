// FIXME: golangci-lint
// nolint:errcheck,govet,revive
package common

import (
	"context"
	"net/http"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/errors"
)

type paginationContextKey int

const (
	// PaginationKey is used to store pagination data in request context
	PaginationKey paginationContextKey = 1
	defaultLimit  int                  = 30
	defaultOffset int                  = 0
)

// Pagination represents pagination parameters
type Pagination struct {
	// Limit represents how many items to return
	Limit int
	// Offset represents from what item to start
	Offset int
}

// EdgeAPIPaginatedResponse represents pagination response
type EdgeAPIPaginatedResponse struct {
	Count int64
	Data  interface{}
}

// ValidationError represents validation error
type ValidationError struct {
	Key    string
	Reason string
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
			} // FIXME: golangci-lint
			// nolint:revive

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
