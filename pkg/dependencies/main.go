package dependencies

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/services"
)

type EdgeAPIServices struct {
	CommitService   services.CommitServiceInterface
	DeviceService   services.DeviceServiceInterface
	ImageService    services.ImageServiceInterface
	RepoService     services.RepoServiceInterface
	ImageSetService services.ImageSetsServiceInterface
	UpdateService   services.UpdateServiceInterface
}

// Init creates all services that Edge API depends on in order to have dependency injection on context
func Init(ctx context.Context) *EdgeAPIServices {
	return &EdgeAPIServices{
		CommitService:   services.NewCommitService(ctx),
		ImageService:    services.NewImageService(ctx),
		RepoService:     services.NewRepoService(ctx),
		ImageSetService: services.NewImageSetsService(ctx),
		UpdateService:   services.NewUpdateService(ctx),
		DeviceService:   services.NewDeviceService(ctx),
	}
}

type key int

// Key is the context key for dependencies on the request context
const Key key = iota

// Middleware is the dependencies Middleware that serves all Edge API services on the current request context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		services := Init(r.Context())
		ctx := context.WithValue(r.Context(), Key, services)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
