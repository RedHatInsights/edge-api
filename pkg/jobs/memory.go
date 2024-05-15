package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/redhatinsights/edge-api/pkg/metrics"
	"github.com/sirupsen/logrus"
)

type Config struct {
	QueueSize int
	Workers   int
	Timeout   time.Duration
	IntSignal os.Signal
}

type MemoryWorker struct {
	cfg Config
	hs  map[JobType]JobHandler
	fhs map[JobType]JobHandler
	q   chan *Job
	oc  *sync.Once
	wg  sync.WaitGroup
	cf  []context.CancelFunc
	cfm sync.Mutex
	sen atomic.Int64
	sac atomic.Int64
	sig os.Signal
}

func NewMemoryClientWithConfig(config Config) *MemoryWorker {
	return &MemoryWorker{
		cfg: config,
		hs:  make(map[JobType]JobHandler),
		fhs: make(map[JobType]JobHandler),
		q:   make(chan *Job, config.QueueSize),
		oc:  &sync.Once{},
		wg:  sync.WaitGroup{},
		cf:  make([]context.CancelFunc, 0, config.Workers+1),
		sig: config.IntSignal,
	}
}

func NewMemoryClient() *MemoryWorker {
	return NewMemoryClientWithConfig(Config{
		QueueSize: 10,
		Workers:   10,
		Timeout:   4 * time.Hour,
		IntSignal: os.Interrupt,
	})
}

// RegisterHandlers registers job handlers for a specific job type.
func (w *MemoryWorker) RegisterHandlers(jtype JobType, handler, failureHandler JobHandler) {
	w.hs[jtype] = handler
	w.fhs[jtype] = failureHandler
}

// Enqueue sends a job to the worker queue.
func (w *MemoryWorker) Enqueue(ctx context.Context, job *Job) error {
	if job == nil {
		return fmt.Errorf("unable to enqueue job: %w", ErrJobNotFound)
	}

	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}

	_, logger := initJobContext(ctx, job)
	logger.WithField("job_args", job.Args).Infof("Enqueuing job %s of type %s", job.ID, job.Type)
	w.q <- job
	w.sen.Add(1)
	metrics.JobEnqueuedCount.WithLabelValues(string(job.Type)).Inc()
	return nil
}

// Start managed goroutines to process jobs from the queue. Additionally, start
// goroutine to handle interrupt signal if provided. This method does not block.
// Worker must be gracefully stopped via Stop().
func (w *MemoryWorker) Start(ctx context.Context) {
	w.cfm.Lock()
	defer w.cfm.Unlock()

	logrus.WithContext(ctx).Infof("Starting %d job workers", w.cfg.Workers)
	for i := 0; i < w.cfg.Workers; i++ {
		uid := uuid.New()
		w.wg.Add(1)
		logrus.WithContext(ctx).Infof("Started worker with uuid %s", uid)
		gctx, cf := context.WithCancel(ctx)
		w.cf = append(w.cf, cf)
		go w.dequeueLoop(gctx, uid)
	}

	// Handle interrupt signal
	if w.sig != nil {
		intC := make(chan os.Signal, 1)
		signal.Notify(intC, w.sig)

		ctxInt, cfInt := context.WithCancel(ctx)
		w.cf = append(w.cf, cfInt)
		go func() {
			select {
			case <-intC:
				logrus.WithContext(ctx).Debugf("Interrupt detected, sending cancel to all workers")
				w.cancelAll()
			case <-ctxInt.Done():
				logrus.WithContext(ctx).Debugf("Stopping interrupt signal goroutine")
				return
			}
		}()
	}
}

// Stop processing of all free goroutines, queue is discarded but all active jobs are left
// to finish. It blocks until all workers are done which may be terminated by kubernetes.
func (w *MemoryWorker) Stop(ctx context.Context) {
	w.oc.Do(func() {
		w.cfm.Lock()
		defer w.cfm.Unlock()

		// Print some stats before we start the procedure
		s, err := w.Stats(ctx)
		if err != nil {
			logrus.WithContext(ctx).Errorf("Error getting stats: %v", err)
		}
		logrus.WithContext(ctx).Infof("Stopping jobs, %d active jobs, %d queued jobs (waiting started)", s.Active, s.Enqueued)

		// Stop all idle workers by closing the queue
		close(w.q)

		w.cancelAll()

		// Wait for active workers to finish
		w.wg.Wait()
		logrus.WithContext(ctx).Info("All goroutines stopped")
	})
}

// Stop all running workers
func (w *MemoryWorker) cancelAll() {
	for _, cf := range w.cf {
		cf()
	}
}

func (w *MemoryWorker) dequeueLoop(ctx context.Context, wid uuid.UUID) {
	defer w.wg.Done()

	// Handle also stats updates, in extreme case when all workers are busy, stats
	// will not be updated for a while. Not
	statsTick := time.NewTicker(1 * time.Second)
	defer statsTick.Stop()

	for {
		select {
		case job := <-w.q:
			if job == nil {
				logrus.WithContext(ctx).Debug("Stopping worker goroutine (closed channel)")
				return
			}

			w.sen.Add(-1)
			w.processJob(ctx, job, wid)
		case <-statsTick.C:
			s, _ := w.Stats(ctx)
			metrics.JobActiveSize.Set(float64(s.Active))
			metrics.JobQueueSize.Set(float64(s.Enqueued))
		case <-ctx.Done():
			logrus.WithContext(ctx).Debug("Stopping worker goroutine (context done)")
			return
		}
	}
}

func (w *MemoryWorker) processJob(ctx context.Context, job *Job, wid uuid.UUID) {
	if job == nil {
		logrus.WithContext(ctx).Error(ErrJobNotFound)
		return
	}
	w.sac.Add(1)
	defer w.sac.Add(-1)

	ctx, logger := initJobContext(ctx, job)
	logger = logger.WithFields(logrus.Fields{
		"worker_id": wid,
		"job_args":  job.Args,
	})

	if h, ok := w.hs[job.Type]; ok {
		ctx, cFunc := context.WithTimeout(ctx, w.cfg.Timeout)
		defer cFunc()

		if fh, ok := w.fhs[job.Type]; ok {
			defer func() {
				// call failure handler if job panics or context is cancelled/expired
				call := false
				if r := recover(); r != nil {
					logger.Warningf("Job %s of type %s panic: %s, calling interrupt handler", job.ID, job.Type, r)
					call = true
					metrics.JobProcessedCount.WithLabelValues(string(job.Type), "panicked").Inc()
				} else if ctx.Err() != nil {
					logger.Warningf("Job %s of type %s was cancelled: %s, calling interrupt handler", job.ID, job.Type, ctx.Err().Error())
					call = true
					if errors.Is(ctx.Err(), context.DeadlineExceeded) {
						metrics.JobProcessedCount.WithLabelValues(string(job.Type), "timeouted").Inc()
					} else {
						metrics.JobProcessedCount.WithLabelValues(string(job.Type), "cancelled").Inc()
					}
				}

				if call {
					start := time.Now()
					fh(ctx, job)
					elapsed := time.Since(start)
					logger.Infof("Failure handler %s of type %s completed in %s seconds", job.ID, job.Type, elapsed.Seconds())
				}
			}()
		}

		logger.Infof("Processing job %s of type %s", job.ID, job.Type)
		start := time.Now()
		h(ctx, job)
		elapsed := time.Since(start)
		logger.Infof("Job %s of type %s completed in %.02f seconds", job.ID, job.Type, elapsed.Seconds())
		metrics.JobProcessedCount.WithLabelValues(string(job.Type), "finished").Inc()
		metrics.BackgroundJobDuration.WithLabelValues(string(job.Type)).Observe(elapsed.Seconds())
	} else {
		logger.Errorf("Memory worker handler not found for job type: %s", job.Type)
	}
}

func (w *MemoryWorker) Stats(_ context.Context) (Stats, error) {
	return Stats{
		Active:   w.sac.Load(),
		Enqueued: w.sen.Load(),
	}, nil
}
