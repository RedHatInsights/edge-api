// FIXME: golangci-lint
// nolint:revive
package routes

import "net/http"

// StatusOK returns a simple 200 status code
func StatusOK(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}
