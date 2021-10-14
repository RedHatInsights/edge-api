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

const Key key = iota

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		services := Init(r.Context())
		ctx := context.WithValue(r.Context(), Key, services)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
