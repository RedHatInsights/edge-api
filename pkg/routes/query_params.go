package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/redhatinsights/edge-api/pkg/dependencies"
)

var m map[string][]string

func initalizeQueryParamsArray() map[string][]string {

	if len(m) == 0 {
		m = make(map[string][]string)
		m["device-groups"] = []string{"name", "created_at", "updated_at", "sort_by"}
		m["devices"] = []string{"name", "uuid", "update_available", "image_id"}
		m["images"] = []string{"status", "name", "distribution", "created_at"}
	}
	return m
}

// GetQueryParamsArray get the name of the service and return the supported query params
func GetQueryParamsArray(endpoint string) []string {
	switch endpoint {
	case "device-groups":
		return []string{"name", "created_at", "updated_at", "sort_by"}
	case "devices":
		return []string{"name", "uuid", "update_available", "image_id"}
	case "images":
		return []string{"status", "name", "distribution", "created_at"}
	default:
		return nil
	}

	//qpa := initalizeQueryParamsArray()
	//return qpa[endpoint]
	//paramsArray := make([]string, len(qpa[endpoint]))
	//copy(paramsArray, qpa[endpoint])
	//return paramsArray
}

// ValidateQueryParams validate the query params from the url are supported
func ValidateQueryParams(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var errs []validationError
		path := strings.Split(r.URL.Path, "/")
		endpoint := path[len(path)-1]
		filtersMap := r.URL.Query()
		queriesKeys := reflect.ValueOf(filtersMap).MapKeys()
		qparamsArray := GetQueryParamsArray(endpoint)
		// interating over the queries keys to validate we support those
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
