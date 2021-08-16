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
