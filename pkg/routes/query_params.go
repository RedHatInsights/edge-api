package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
)

var m map[string][]string

func initalizeQueryParamsArray() map[string][]string {

	if len(m) == 0 {
		m = make(map[string][]string)
		m["device-groups"] = []string{"limit", "offset", "name", "created_at", "updated_at", "sort_by"}
		m["devices"] = []string{"per_page", "page", "sort_by", "order_how", "hostname_or_id"}
		m["devicesview"] = []string{"limit", "offset", "name", "uuid", "update_available", "image_id", "sort_by", "created_at"}
		m["images"] = []string{"limit", "offset", "status", "name", "distribution", "created_at", "sort_by"}
		m["image-sets"] = []string{"limit", "offset", "status", "name", "version", "sort_by"}
		m["thirdpartyrepo"] = []string{"limit", "offset", "name", "created_at", "updated_at", "imageID", "sort_by"}
		m["updates"] = []string{"limit", "offset", "created_at", "updated_at", "status", "sort_by"}
	}
	return m
}

// GetQueryParamsArray get the name of the service and return the supported query params
func GetQueryParamsArray(endpoint string) []string {
	qpa := initalizeQueryParamsArray()
	paramsArray := make([]string, len(qpa[endpoint]))
	copy(paramsArray, qpa[endpoint])
	return paramsArray
}

// ValidateQueryParams validate the query params from the url are supported
func ValidateQueryParams(endpoint string) func(next http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var errs []validationError
			filtersMap := r.URL.Query()
			queriesKeys := reflect.ValueOf(filtersMap).MapKeys()
			qparamsArray := GetQueryParamsArray(endpoint)
			// iterating over the queries keys to validate we support those
			for _, key := range queriesKeys {
				if !(contains(qparamsArray, key.String())) {
					qkey := key.String()
					errs = append(errs, validationError{Key: qkey, Reason: fmt.Sprintf("%s is not a valid query param, supported query params: %s", qkey, qparamsArray)})
				}
			}

			if len(errs) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(&errs); err != nil {
				ctxServices := dependencies.ServicesFromContext(r.Context())
				ctxServices.Log.WithField("error", errs).Error("Error while trying to encode device groups query-params validation errors")
			}
		})
	}
}
