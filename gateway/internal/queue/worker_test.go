package queue

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yourusername/toolset-api/gateway/internal/db"
	"github.com/yourusername/toolset-api/gateway/internal/executor"
)

// fakeRunner records invocations and returns a canned response.
type fakeRunner struct {
	mu    sync.Mutex
	calls int32
	resp  executor.ExecResponse
	err   error
	seen  []string // languages seen
}

func (f *fakeRunner) Execute(_ context.Context, req executor.ExecRequest) (executor.ExecResponse, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	f.seen = append(f.seen, req.Language)
	f.mu.Unlock()
	return f.resp, f.err
}

func newWorkerTestDB(t *testing.T) *db.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "worker_test.db")
	migrationsDir := filepath.Join("..", "..", "migrations")
	d, err := db.Open(db.Config{Path: dbPath, MaxConnections: 4, MigrationsDir: migrationsDir})
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if _, err := d.Exec(
		`INSERT INTO tools (id, name, category, status) VALUES ('tool-exec','exec-light','exec','ready');`,
	); err != nil {
		t.Fatalf("seed tool: %v", err)
	}
	return d
}

// waitForStatus polls until the job reaches the wanted status or times out.
func waitForStatus(t *testing.T, q *executor.Queue, jobID string, want executor.JobStatus) executor.Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := q.Get(context.Background(), jobID)
		if err == nil && job.Status == want {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach status %s in time", jobID, want)
	return executor.Job{}
}

func TestWorkerProcessesJob(t *testing.T) {
	d := newWorkerTestDB(t)
	q := executor.NewQueue(d.DB)
	runner := &fakeRunner{resp: executor.ExecResponse{Status: "success", ExitCode: 0, Stdout: "done"}}

	w := NewWorker(Config{
		Queue:    q,
		Runner:   runner,
		Workers:  1,
		Interval: 10 * time.Millisecond,
	})
	w.Start(context.Background())
	defer w.Stop()

	job, err := q.Submit(context.Background(), "tool-exec", executor.ExecRequest{Code: "print(1)", Language: "python"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got := waitForStatus(t, q, job.JobID, executor.JobCompleted)
	if got.Stdout != "done" {
		t.Errorf("expected stdout 'done', got %q", got.Stdout)
	}
	if atomic.LoadInt32(&runner.calls) != 1 {
		t.Errorf("expected 1 runner call, got %d", runner.calls)
	}
}

func TestWorkerConcurrentSubmissions(t *testing.T) {
	d := newWorkerTestDB(t)
	q := executor.NewQueue(d.DB)
	runner := &fakeRunner{resp: executor.ExecResponse{Status: "success", ExitCode: 0}}

	w := NewWorker(Config{
		Queue:    q,
		Runner:   runner,
		Workers:  3,
		Interval: 10 * time.Millisecond,
	})
	w.Start(context.Background())
	defer w.Stop()

	const n = 6
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		job, err := q.Submit(context.Background(), "tool-exec", executor.ExecRequest{Code: "print(1)", Language: "python"})
		if err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
		ids = append(ids, job.JobID)
	}

	for _, id := range ids {
		waitForStatus(t, q, id, executor.JobCompleted)
	}
	if got := atomic.LoadInt32(&runner.calls); got != n {
		t.Errorf("expected %d runner calls, got %d", n, got)
	}
}

func TestWorkerStopIdempotent(t *testing.T) {
	d := newWorkerTestDB(t)
	q := executor.NewQueue(d.DB)
	w := NewWorker(Config{
		Queue:    q,
		Runner:   &fakeRunner{},
		Workers:  2,
		Interval: 10 * time.Millisecond,
	})
	w.Start(context.Background())
	w.Stop()
	// Second stop should not panic or block.
	w.Stop()
}
