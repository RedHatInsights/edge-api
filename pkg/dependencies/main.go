package dependencies

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
	"github.com/redhatinsights/edge-api/pkg/services/ownershipvoucher"
	"github.com/redhatinsights/platform-go-middlewares/request_id"
	log "github.com/sirupsen/logrus"
)

// EdgeAPIServices is the list of Edge API services
type EdgeAPIServices struct {
	CommitService           services.CommitServiceInterface
	DeviceService           services.DeviceServiceInterface
	ImageService            services.ImageServiceInterface
	RepoService             services.RepoServiceInterface
	ImageSetService         services.ImageSetsServiceInterface
	UpdateService           services.UpdateServiceInterface
	ThirdPartyRepoService   services.ThirdPartyRepoServiceInterface
	OwnershipVoucherService ownershipvoucher.ServiceInterface
	Log                     *log.Entry
}

// Init creates all services that Edge API depends on in order to have dependency injection on context
func Init(ctx context.Context) *EdgeAPIServices {
	account, _ := common.GetAccountFromContext(ctx)
	log := log.WithFields(log.Fields{
		"requestId": request_id.GetReqID(ctx),
		"accountId": account,
	})
	return &EdgeAPIServices{
		CommitService:           services.NewCommitService(ctx),
		ImageService:            services.NewImageService(ctx, log),
		RepoService:             services.NewRepoService(ctx),
		ImageSetService:         services.NewImageSetsService(ctx),
		UpdateService:           services.NewUpdateService(ctx),
		DeviceService:           services.NewDeviceService(ctx),
		ThirdPartyRepoService:   services.NewThirdPartyRepoService(ctx),
		OwnershipVoucherService: ownershipvoucher.NewService(ctx, log),
		Log:                     log,
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
