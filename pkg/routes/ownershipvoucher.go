//go:build !fdo
// +build !fdo

// FIXME: golangci-lint
// nolint:revive
package routes

import "github.com/go-chi/chi"

// MakeFDORouter do nothing for non-fdo builds
func MakeFDORouter(sub chi.Router) {}
