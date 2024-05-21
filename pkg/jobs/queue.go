package jobs

import (
	"context"

	"github.com/sirupsen/logrus"
)

var Queue JobWorker

type handlers struct {
	h  map[JobType]JobHandler
	fh map[JobType]JobHandler
}

var hHap = make(map[JobType]handlers)

// InitMemoryWorker initializes the default worker queue with an in-memory worker. Call
// RegisterHandlers() before calling this function to register job handlers.
func InitMemoryWorker() {
	Queue = NewMemoryClient()
	registerHandlers()
}

// InitDummyWorker initializes the dummy (testing) worker queue with an in-memory worker. Call
// RegisterHandlers() before calling this function to register job handlers.
func InitDummyWorker() {
	Queue = NewDummyWorker()
	registerHandlers()
}

func registerHandlers() {
	for k, v := range hHap {
		logrus.Debugf("Registering handlers for job type: %s", k)
		Queue.RegisterHandlers(k, v.h[k], v.fh[k])
	}
}

// Worker returns the default worker queue.
func Worker() JobWorker {
	return Queue
}

// Enqueue sends an existing job to the worker queue.
func Enqueue(ctx context.Context, job *Job) error {
	return Queue.Enqueue(ctx, job)
}

// NewAndEnqueue sends an existing job to the fast worker queue.
func NewAndEnqueue(ctx context.Context, jobType JobType, args any) error {
	job := New(ctx, jobType, FastQueue, args)
	return Queue.Enqueue(ctx, job)
}

// NewAndEnqueueSlow sends an existing job to the slow worker queue.
func NewAndEnqueueSlow(ctx context.Context, jobType JobType, args any) error {
	job := New(ctx, jobType, SlowQueue, args)
	return Queue.Enqueue(ctx, job)
}

// RegisterHandlers registers a job handler for a specific job type. This function must be called
// before InitMemoryWorker() or InitDummyWorker(). All previously registered handlers are passed into
// Queue.Worker.RegisterHandlers().
//
// To register a handler after worker initialization, use Worker().RegisterHandlers() instead.
func RegisterHandlers(jobType JobType, jobHandler JobHandler, failureHandler JobHandler) {
	hHap[jobType] = handlers{
		h:  map[JobType]JobHandler{jobType: jobHandler},
		fh: map[JobType]JobHandler{jobType: failureHandler},
	}
}
