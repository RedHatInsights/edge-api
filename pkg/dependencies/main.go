package dependencies

import (
	"context"
	"errors"
	"net/http"

	"github.com/redhatinsights/edge-api/logger"
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
	DeviceGroupsService     services.DeviceGroupsServiceInterface
	Log                     *log.Entry
}

// Init creates all services that Edge API depends on in order to have dependency injection on context
// Context is the environment for a request (think Bash environment variables)
func Init(ctx context.Context) *EdgeAPIServices {
	account, _ := common.GetAccountFromContext(ctx)
	orgID, _ := common.GetOrgIDFromContext(ctx)
	log := log.WithFields(log.Fields{
		"requestId": request_id.GetReqID(ctx),
		"accountId": account,
		"orgID":     orgID,
	})
	return &EdgeAPIServices{
		CommitService:           services.NewCommitService(ctx, log),
		ImageService:            services.NewImageService(ctx, log),
		RepoService:             services.NewRepoService(ctx, log),
		ImageSetService:         services.NewImageSetsService(ctx, log),
		UpdateService:           services.NewUpdateService(ctx, log),
		ThirdPartyRepoService:   services.NewThirdPartyRepoService(ctx, log),
		DeviceService:           services.NewDeviceService(ctx, log),
		OwnershipVoucherService: services.NewOwnershipVoucherService(ctx, log),
		DeviceGroupsService:     services.NewDeviceGroupsService(ctx, log),
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
	// If there is problem with retrieving context key value, there is a critical issue with the
	// environment or code and we need to raise an alert and panic the container
	if !ok {
		err := errors.New("could not get EdgeAPIServices key value from context")
		logger.LogErrorAndPanic("could not get EdgeAPIServices key value from context", err)
	}

	return edgeAPIServices
}

// Middleware is the dependencies Middleware that serves all Edge API services on the current request context
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		edgeAPIServices := Init(r.Context())
		ctx := ContextWithServices(r.Context(), edgeAPIServices)
		ctx = common.SetOriginalIdentity(ctx, r.Header.Get("X-Rh-Identity"))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
