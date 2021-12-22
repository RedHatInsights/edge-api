//go:build !fdo
// +build !fdo

package routes

import "github.com/go-chi/chi"

// MakeFDORouter do nothing for non-fdo builds
func MakeFDORouter(sub chi.Router) {}
