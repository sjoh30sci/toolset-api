// Package queue implements the background worker pool that drains the async
// job queue, dispatching each pending job to the executor sandbox and
// recording its result.
package queue

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/yourusername/toolset-api/gateway/internal/executor"
)

// pollInterval is how often idle workers re-check the queue for new jobs.
const pollInterval = 500 * time.Millisecond

// Runner executes a stored request against a sandbox. *executor.Client
// satisfies this; tests supply a fake.
type Runner interface {
	Execute(ctx context.Context, req executor.ExecRequest) (executor.ExecResponse, error)
}

// dispatcher is the subset of *executor.Queue the worker needs. It is
// unexported because it relies on package-internal methods; the concrete
// *executor.Queue is passed in via NewWorker.
type dispatcher interface {
	ClaimNext(ctx context.Context) (jobID, execID string, req executor.ExecRequest, err error)
	Complete(ctx context.Context, jobID, execID string, res executor.ExecResponse, runErr error) error
}

// Worker drains the job queue using a fixed pool of goroutines.
type Worker struct {
	q        dispatcher
	runner   Runner
	logger   *slog.Logger
	workers  int
	interval time.Duration

	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// Config configures a Worker.
type Config struct {
	Queue    dispatcher
	Runner   Runner
	Logger   *slog.Logger
	Workers  int
	Interval time.Duration // optional; defaults to pollInterval
}

// NewWorker builds a Worker. Workers < 1 defaults to 1.
func NewWorker(cfg Config) *Worker {
	n := cfg.Workers
	if n < 1 {
		n = 1
	}
	iv := cfg.Interval
	if iv <= 0 {
		iv = pollInterval
	}
	lg := cfg.Logger
	if lg == nil {
		lg = slog.Default()
	}
	return &Worker{
		q:        cfg.Queue,
		runner:   cfg.Runner,
		logger:   lg,
		workers:  n,
		interval: iv,
	}
}

// Start launches the worker goroutines. It returns immediately; call Stop to
// shut them down.
func (w *Worker) Start(ctx context.Context) {
	ctx, w.cancel = context.WithCancel(ctx)
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.loop(ctx, i)
	}
	w.logger.Info("queue worker started", "workers", w.workers)
}

// Stop signals all workers to stop and waits for them to finish the in-flight
// job (if any).
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
	w.wg.Wait()
	w.logger.Info("queue worker stopped")
}

// loop is a single worker goroutine's run loop.
func (w *Worker) loop(ctx context.Context, id int) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Drain greedily: keep processing while jobs remain.
			for w.processOne(ctx, id) {
				if ctx.Err() != nil {
					return
				}
			}
		}
	}
}

// processOne claims and runs at most one job. It returns true if a job was
// processed (so the caller can keep draining), false if the queue was empty.
func (w *Worker) processOne(ctx context.Context, id int) bool {
	jobID, execID, req, err := w.q.ClaimNext(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && ctx.Err() == nil {
			w.logger.Warn("queue claim failed", "worker", id, "err", err)
		}
		return false
	}

	w.logger.Info("job started", "worker", id, "job_id", jobID, "language", req.Language)

	res, runErr := w.runner.Execute(ctx, req)
	if cerr := w.q.Complete(ctx, jobID, execID, res, runErr); cerr != nil {
		w.logger.Error("job completion write failed", "job_id", jobID, "err", cerr)
	}
	w.logger.Info("job finished", "worker", id, "job_id", jobID, "status", res.Status, "run_err", runErr)
	return true
}
