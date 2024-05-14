package services

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/jobs"
	feature "github.com/redhatinsights/edge-api/unleash/features"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	log "github.com/sirupsen/logrus"
)

type NoopJob struct {
}

func NoopHandler(ctx context.Context, _ *jobs.Job) {
	log.WithContext(ctx).Info("NoopHandler called")
}

func NoopFailureHandler(ctx context.Context, _ *jobs.Job) {
	log.WithContext(ctx).Info("NoopFailureHandler called")
}

func FallbackHandler(ctx context.Context, _ *jobs.Job) {
	log.WithContext(ctx).Info("FallbackHandler called")
	panic("failure")
}

func FallbackFailureHandler(ctx context.Context, _ *jobs.Job) {
	log.WithContext(ctx).Info("FallbackFailureHandler called")
}

func init() {
	jobs.RegisterHandlers("NoopJob", NoopHandler, NoopFailureHandler)
	jobs.RegisterHandlers("FallbackJob", FallbackHandler, FallbackFailureHandler)
}

func CreateNoopJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if feature.JobQueue.IsEnabledCtx(ctx) {
		orgID := identity.GetIdentity(ctx).Identity.OrgID
		log.WithContext(ctx).Infof("Enqueuing NoopJob for org %s", orgID)

		job := jobs.Job{
			Type:     "NoopJob",
			Args:     &NoopJob{},
			Identity: identity.GetRawIdentity(ctx),
		}

		err := jobs.Enqueue(ctx, &job)
		if err != nil {
			log.WithContext(ctx).Errorf("Cannot enqueue job: %s", err)
		}
		w.WriteHeader(http.StatusOK)
	} else {
		log.WithContext(ctx).Info("Not enqueuing NoopJob - job queue not enabled")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func CreateFallbackJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if feature.JobQueue.IsEnabledCtx(ctx) {
		orgID := identity.GetIdentity(ctx).Identity.OrgID
		log.WithContext(ctx).Infof("Enqueuing NoopJob for org %s", orgID)

		job := jobs.Job{
			Type:     "FallbackJob",
			Args:     &NoopJob{},
			Identity: identity.GetRawIdentity(ctx),
		}

		err := jobs.Enqueue(ctx, &job)
		if err != nil {
			log.WithContext(ctx).Errorf("Cannot enqueue job: %s", err)
		}
		w.WriteHeader(http.StatusOK)
	} else {
		log.WithContext(ctx).Info("Not enqueuing NoopJob - job queue not enabled")
		w.WriteHeader(http.StatusBadRequest)
	}
}
