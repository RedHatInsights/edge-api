package jobs

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/osbuild/logging/pkg/logrus"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
)

func init() {
	// makes UUID generation faster
	uuid.EnableRandPool()
}

// JobType represents a "channel" for jobs. Each channel must have exactly one handler.
type JobType string

// JobHandler is a function that processes a job. Does not return error, the function must
// handle all errors internally. Panics will cause the job to be retried and failure handler
// will be called.
type JobHandler func(ctx context.Context, job *Job)

// JobQueue represents a queue where job is enqueued.
type JobQueue int

const (
	// SlowQueue is a queue for jobs that can be processed slowly.
	SlowQueue JobQueue = iota

	// FastQueue is a queue for jobs that should be processed quickly.
	FastQueue
)

// Job represents a single job. It is a message that is sent to a worker.
// Job arguments are serialized so do not store pointers - workers must not share any
// memory with the caller.
type Job struct {
	// Random UUID for logging and tracing. It is generated randomly by Enqueue function when blank.
	ID uuid.UUID

	// Job queue. Fast queue is for jobs that should be processed quickly, slow queue is for jobs
	// that can be processed slowly. This allows better resource management. When not specified,
	// job is enqueued to the slow queue.
	Queue JobQueue

	// Job type or "queue"
	Type JobType

	// Red Hat platform identity encoded with base64
	Identity string

	// Red Hat platform request id
	CorrelationID string

	// Job arguments
	Args any
}

// New creates new job and sets identity and correlation id from passed context.
func New(ctx context.Context, jobType JobType, jq JobQueue, args any) *Job {
	return &Job{
		ID:            uuid.New(),
		Queue:         jq,
		Type:          jobType,
		Identity:      identity.GetRawIdentity(ctx),
		CorrelationID: request_id.GetReqID(ctx),
		Args:          args,
	}
}

var ErrJobNotFound = errors.New("job not found")
var ErrHandlerNotFound = errors.New("handler not registered")

// IgnoredJobHandler is a handler that does nothing. It is used when no handler is registered for a job.
var IgnoredJobHandler JobHandler = func(_ context.Context, _ *Job) {
	// do nothing
}

// JobEnqueuer sends Job messages into worker queue.
type JobEnqueuer interface {
	// Enqueue delivers a job to one of the backend workers.
	Enqueue(context.Context, *Job) error
}

// JobWorker receives and handles Job messages.
type JobWorker interface {
	JobEnqueuer

	// RegisterHandlers registers an event listener for a particular type with an associated handler. The first handler
	// is for business logic, the second handler is for error handling. The second handler is called when job is processing
	// for too long, on graceful shutdown, panic or SIGINT.
	RegisterHandlers(JobType, JobHandler, JobHandler)

	// Start starts one or more goroutines to dispatch incoming jobs.
	Start(ctx context.Context)

	// Stop let's background workers to finish all jobs and terminates them. It blocks until workers are done.
	Stop(ctx context.Context)

	// Stats returns statistics. Not all implementations supports stats, some may return zero values.
	Stats(ctx context.Context) (Stats, error)
}

func (jt JobType) String() string {
	return string(jt)
}

// Stats provides monitoring statistics.
type Stats struct {
	// Number of jobs currently in the queue.
	Enqueued int64

	// Number of jobs currently being processed.
	Active int64
}

type jobKeyID int

const (
	jobIDCtxKey  jobKeyID = iota
	corrIDCtxKey jobKeyID = iota
)

// JobID returns job id or an empty string when not set.
func JobID(ctx context.Context) string {
	value := ctx.Value(jobIDCtxKey)
	if value == nil {
		return ""
	}
	return value.(string)
}

// WithJobID returns context copy with trace id value.
func WithJobID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, jobIDCtxKey, id)
}

// CorrID returns job id or an empty string when not set.
func CorrID(ctx context.Context) string {
	value := ctx.Value(corrIDCtxKey)
	if value == nil {
		return ""
	}
	return value.(string)
}

// WithCorrID returns context copy with trace id value.
func WithCorrID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, corrIDCtxKey, id)
}

func initJobContext(origCtx context.Context, job *Job) (context.Context, logrus.FieldLogger) {
	ctx := WithJobID(origCtx, job.ID.String())
	ctx = WithCorrID(ctx, job.CorrelationID)

	id, err := identity.DecodeIdentity(job.Identity)
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Warnf("Error decoding identity: %s", err)
		id = identity.XRHID{}
	}
	ctx = identity.WithIdentity(ctx, id)
	ctx = identity.WithRawIdentity(ctx, job.Identity)

	return ctx, logrus.WithContext(ctx).WithFields(
		logrus.Fields{
			"job_id":         job.ID,
			"job_type":       job.Type,
			"org_id":         id.Identity.OrgID,
			"correlation_id": job.CorrelationID,
		},
	)
}
