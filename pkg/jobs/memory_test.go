package jobs

import (
	"context"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var defaultConfig = Config{
	QueueSize: 1,
	Workers:   2,
	Timeout:   30 * time.Second,
}

func TestMemoryWorker_Enqueue(t *testing.T) {
	ctx := context.Background()
	worker := NewMemoryClientWithConfig(defaultConfig)
	defer worker.Stop(ctx)
	var success atomic.Bool

	worker.RegisterHandlers("test", func(context.Context, *Job) {
		// called to process job
		success.Store(true)
	}, func(context.Context, *Job) {
		// called when context is cancelled, expires or after unhandled panic
		t.Error("Failure handler should not be called")
	})
	worker.Start(ctx)

	job := &Job{
		Type: "test",
	}

	err := worker.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("Enqueue call failed: %v", err)
	}

	waitUntilTrue(t, success.Load, "Timeout: job was not processed successfully")
}

func TestMemoryWorker_Stats(t *testing.T) {
	ctx := context.Background()
	worker := NewMemoryClientWithConfig(defaultConfig)
	defer worker.Stop(ctx)
	var success atomic.Bool

	worker.RegisterHandlers("test", func(context.Context, *Job) {
		// called to process job
		success.Store(true)
	}, func(context.Context, *Job) {
		// called when context is cancelled, expires or after unhandled panic
		t.Error("Failure handler should not be called")
	})

	job := &Job{
		Type: "test",
	}

	err := worker.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("Enqueue call failed: %v", err)
	}

	s, err := worker.Stats(ctx)
	if err != nil {
		t.Errorf("Stats call failed: %v", err)
	}
	if s.Enqueued != 1 {
		t.Errorf("Expected 1 enqueued job, got %d", s.Enqueued)
	}

	worker.Start(ctx)
	waitUntilTrue(t, success.Load, "Timeout: job was not processed successfully")
}

func TestMemoryWorker_Timeout(t *testing.T) {
	ctx := context.Background()
	config := defaultConfig
	config.Timeout = 1 * time.Microsecond
	worker := NewMemoryClientWithConfig(config)
	defer worker.Stop(ctx)
	var success atomic.Bool

	worker.RegisterHandlers("test", func(context.Context, *Job) {
		// called to process job
	}, func(context.Context, *Job) {
		// called when context is cancelled, expires or after unhandled panic
		success.Store(true)
	})

	job := &Job{
		Type: "test",
	}

	err := worker.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("Enqueue call failed: %v", err)
	}

	time.Sleep(2 * time.Microsecond)
	worker.Start(ctx)
	waitUntilTrue(t, success.Load, "Timeout: failure handler was not called for timeout test")
}

func TestMemoryWorker_Cancel(t *testing.T) {
	ctx := context.Background()
	worker := NewMemoryClientWithConfig(defaultConfig)
	var success atomic.Bool

	worker.RegisterHandlers("test", func(context.Context, *Job) {
		// called to process job
		go worker.Stop(ctx)
		// graceful shutdown initiated - wait little bit more to trigger context cancel
		time.Sleep(200 * time.Millisecond)
	}, func(context.Context, *Job) {
		// called when context is cancelled, expires or after unhandled panic
		success.Store(true)
	})

	job := &Job{
		Type: "test",
	}

	err := worker.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("Enqueue call failed: %v", err)
	}

	worker.Start(ctx)
	waitUntilTrue(t, success.Load, "Timeout: failure handler was not called for cancel test")
}

func TestMemoryWorker_Panic(t *testing.T) {
	ctx := context.Background()
	worker := NewMemoryClientWithConfig(defaultConfig)
	defer worker.Stop(ctx)
	var success atomic.Bool

	worker.RegisterHandlers("test", func(context.Context, *Job) {
		// called to process job
		panic("panic calls failure handler")
	}, func(context.Context, *Job) {
		// called when context is cancelled, expires or after unhandled panic
		success.Store(true)
	})

	job := &Job{
		Type: "test",
	}

	err := worker.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("Enqueue call failed: %v", err)
	}

	worker.Start(ctx)
	waitUntilTrue(t, success.Load, "Timeout: failure handler was not called for panic test")
}

func TestMemoryWorker_Interrupt(t *testing.T) {
	ctx := context.Background()
	config := defaultConfig
	config.IntSignal = os.Signal(syscall.SIGUSR1)
	worker := NewMemoryClientWithConfig(config)
	var success atomic.Bool

	worker.RegisterHandlers("test", func(context.Context, *Job) {
		// called to process job
		p, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Errorf("Cannot find the process: %v", err)
		}
		if err := p.Signal(os.Signal(syscall.SIGUSR1)); err != nil {
			t.Errorf("Cannot send the signal: %v", err)
		}

		// graceful shutdown initiated - wait little bit more to trigger context cancel
		time.Sleep(200 * time.Millisecond)
	}, func(context.Context, *Job) {
		// called when context is cancelled, expires or after unhandled panic
		success.Store(true)
	})

	job := &Job{
		Type: "test",
	}

	err := worker.Enqueue(ctx, job)
	if err != nil {
		t.Errorf("Enqueue call failed: %v", err)
	}
	defer worker.Stop(ctx)

	worker.Start(ctx)
	waitUntilTrue(t, success.Load, "Timeout: failure handler was not called for interrupt test")
}

func waitUntilTrue(t *testing.T, check func() bool, msg string) {
	timeout := time.After(5 * time.Second)
	for !check() {
		select {
		case <-timeout:
			t.Error(msg)
			return
		default:
		}
		time.Sleep(50 * time.Millisecond)
	}
}
