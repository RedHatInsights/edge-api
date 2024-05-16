package services

import (
	"context"
	"net/http"

	"github.com/redhatinsights/edge-api/pkg/jobs"
	feature "github.com/redhatinsights/edge-api/unleash/features"
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
		err := jobs.NewAndEnqueue(ctx, "NoopJob", &NoopJob{})
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
		err := jobs.NewAndEnqueue(ctx, "FallbackJob", &NoopJob{})
		if err != nil {
			log.WithContext(ctx).Errorf("Cannot enqueue job: %s", err)
		}
		w.WriteHeader(http.StatusOK)
	} else {
		log.WithContext(ctx).Info("Not enqueuing NoopJob - job queue not enabled")
		w.WriteHeader(http.StatusBadRequest)
	}
}
