package dependencies

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/clients/imagebuilder"
)

type DependenciesKey int

const Key DependenciesKey = iota

type EdgeAPIDependencies struct {
	ImageBuilderClient *imagebuilder.Client
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := &EdgeAPIDependencies{
			ImageBuilderClient: imagebuilder.InitClient(r.Context()),
		}
		ctx := context.WithValue(r.Context(), Key, d)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type EdgeAPIRouter struct {
	Deps *EdgeAPIDependencies
}

func (router *EdgeAPIRouter) AddDependencies(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		router.Deps = r.Context().Value(Key).(*EdgeAPIDependencies)
		next.ServeHTTP(w, r)
	})
}
