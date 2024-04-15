package jobs

import "context"

// Dummy worker is useful for unit tests - it immediately processes the job, there is no queue
// or background goroutines.
type DummyWorker struct {
	hs map[JobType]JobHandler
}

func NewDummyWorker() *DummyWorker {
	return &DummyWorker{
		hs: make(map[JobType]JobHandler),
	}
}

func (w *DummyWorker) RegisterHandlers(jtype JobType, handler, _ JobHandler) {
	w.hs[jtype] = handler
}

func (w *DummyWorker) Enqueue(ctx context.Context, job *Job) error {
	if h, ok := w.hs[job.Type]; ok {
		h(ctx, job)
	}

	return nil
}

func (w *DummyWorker) Start(_ context.Context) {
}

func (w *DummyWorker) Stop(_ context.Context) {
}

func (w *DummyWorker) Stats(_ context.Context) (Stats, error) {
	return Stats{}, nil
}
