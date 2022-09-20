// FIXME: golangci-lint
// nolint:revive
package common

// APIResponse generic model for API responses
type APIResponse struct {
	Message string `json:"message"`
}
