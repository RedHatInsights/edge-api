package dependencies

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/edge-api/pkg/services"
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
	OwnershipVoucherService services.OwnershipVoucherServiceInterface
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
		RepoService:             services.NewRepoService(ctx, log),
		ImageSetService:         services.NewImageSetsService(ctx),
		UpdateService:           services.NewUpdateService(ctx),
		ThirdPartyRepoService:   services.NewThirdPartyRepoService(ctx, log),
		DeviceService:           services.NewDeviceService(ctx, log),
		OwnershipVoucherService: services.NewOwnershipVoucherService(ctx, log),
		Log:                     log,
	}
}

type servicesKeyType string

// servicesKey is the context key for dependencies on the request context
const servicesKey = servicesKeyType("services")

// ContextWithServices add edge apis services to context
func ContextWithServices(ctx context.Context, services *EdgeAPIServices) context.Context {
	return context.WithValue(ctx, servicesKey, services)
}

// ServicesFromContext return the edge api services from context
func ServicesFromContext(ctx context.Context) *EdgeAPIServices {
	edgeAPIServices, ok := ctx.Value(servicesKey).(*EdgeAPIServices)
	if !ok {
		log.Fatal("Could not get EdgeAPIServices from Context")
	}

	return edgeAPIServices
}

// Middleware is the dependencies Middleware that serves all Edge API services on the current request context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		edgeAPIServices := Init(r.Context())
		ctx := ContextWithServices(r.Context(), edgeAPIServices)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
