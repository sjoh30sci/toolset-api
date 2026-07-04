package executor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yourusername/toolset-api/gateway/internal/db"
)

// newTestDB opens a fresh migrated SQLite database for queue tests and seeds a
// tool row so foreign keys are satisfied.
func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "queue_test.db")
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

func TestQueueSubmitAndGet(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	ctx := context.Background()

	job, err := q.Submit(ctx, "tool-exec", ExecRequest{Code: "print(1)", Language: "python"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if job.JobID == "" || job.Status != JobPending {
		t.Fatalf("unexpected job: %+v", job)
	}
	if job.Position != 1 {
		t.Errorf("expected position 1, got %d", job.Position)
	}

	got, err := q.Get(ctx, job.JobID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.JobID != job.JobID || got.Language != "python" {
		t.Errorf("unexpected get result: %+v", got)
	}
}

func TestQueueSubmitUnsupported(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	if _, err := q.Submit(context.Background(), "tool-exec", ExecRequest{Code: "x", Language: "cobol"}); err == nil {
		t.Fatal("expected error for unsupported language")
	}
}

func TestQueueGetNotFound(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	if _, err := q.Get(context.Background(), "does-not-exist"); err != ErrJobNotFound {
		t.Fatalf("expected ErrJobNotFound, got %v", err)
	}
}

func TestQueueCancelPending(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	ctx := context.Background()

	job, err := q.Submit(ctx, "tool-exec", ExecRequest{Code: "print(1)", Language: "python"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	cancelled, err := q.Cancel(ctx, job.JobID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if cancelled.Status != JobCancelled {
		t.Errorf("expected cancelled, got %s", cancelled.Status)
	}
	// Cancelling again should fail.
	if _, err := q.Cancel(ctx, job.JobID); err != ErrJobNotCancellable {
		t.Fatalf("expected ErrJobNotCancellable, got %v", err)
	}
}

func TestQueueClaimAndComplete(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	ctx := context.Background()

	job, err := q.Submit(ctx, "tool-exec", ExecRequest{Code: "print(1)", Language: "python"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	jobID, execID, req, err := q.ClaimNext(ctx)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	if jobID != job.JobID || req.Language != "python" || req.Code != "print(1)" {
		t.Fatalf("unexpected claim: id=%s req=%+v", jobID, req)
	}

	// A second claim should find nothing.
	if _, _, _, err := q.ClaimNext(ctx); err == nil {
		t.Error("expected no rows on second claim")
	}

	res := ExecResponse{Status: "success", ExitCode: 0, Stdout: "1\n"}
	if err := q.Complete(ctx, jobID, execID, res, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := q.Get(ctx, jobID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != JobCompleted {
		t.Errorf("expected completed, got %s", got.Status)
	}
	if got.Stdout != "1\n" {
		t.Errorf("expected stdout preserved, got %q", got.Stdout)
	}
	if got.ExitCode == nil || *got.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %v", got.ExitCode)
	}
}

func TestQueueCompleteFailure(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	ctx := context.Background()

	_, err := q.Submit(ctx, "tool-exec", ExecRequest{Code: "boom", Language: "python"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	jobID, execID, _, err := q.ClaimNext(ctx)
	if err != nil {
		t.Fatalf("ClaimNext: %v", err)
	}
	res := ExecResponse{Status: "error", Error: "compile failed"}
	if err := q.Complete(ctx, jobID, execID, res, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	got, err := q.Get(ctx, jobID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != JobFailed {
		t.Errorf("expected failed, got %s", got.Status)
	}
}

func TestQueuePositions(t *testing.T) {
	d := newTestDB(t)
	q := NewQueue(d.DB)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := q.Submit(ctx, "tool-exec", ExecRequest{Code: "print(1)", Language: "python"}); err != nil {
			t.Fatalf("Submit %d: %v", i, err)
		}
	}
	// Three pending jobs should exist.
	var count int
	if err := d.QueryRow(`SELECT COUNT(*) FROM job_queue WHERE status='pending'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 pending jobs, got %d", count)
	}
}
