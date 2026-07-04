package executor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrJobNotFound is returned when a job id has no matching queue row.
var ErrJobNotFound = errors.New("executor: job not found")

// ErrJobNotCancellable is returned when a cancel targets a terminal job.
var ErrJobNotCancellable = errors.New("executor: job not cancellable")

// Queue provides persistent async job storage over the executions + job_queue
// tables. It is safe for concurrent use (SQLite serializes writes).
type Queue struct {
	db *sql.DB
}

// NewQueue builds a Queue over the given database handle.
func NewQueue(db *sql.DB) *Queue { return &Queue{db: db} }

// Submit persists a new execution + queue entry in a single transaction and
// returns the created Job (status pending) with its queue position.
func (q *Queue) Submit(ctx context.Context, toolID string, req ExecRequest) (Job, error) {
	if q.db == nil {
		return Job{}, errors.New("executor: queue has no database")
	}
	if _, ok := TierFor(req.Language); !ok {
		return Job{}, fmt.Errorf("%w: %s", ErrUnsupportedLanguage, req.Language)
	}

	execID := uuid.NewString()
	jobID := uuid.NewString()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	cpu := req.CPULimitPercent
	if cpu <= 0 {
		cpu = 100
	}
	mem := req.MemoryLimitMB
	if mem <= 0 {
		mem = 512
	}
	now := time.Now().UTC()

	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, fmt.Errorf("executor: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insExec = `
INSERT INTO executions
  (id, tool_id, tool_name, status, job_id, language, code, timeout_seconds,
   cpu_limit_percent, memory_limit_mb, started_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`
	if _, err := tx.ExecContext(ctx, insExec,
		execID, toolID, "exec", string(JobPending), jobID, req.Language, req.Code,
		timeout, cpu, mem, nil,
	); err != nil {
		return Job{}, fmt.Errorf("executor: insert execution: %w", err)
	}

	// Position = number of jobs not yet terminal (including this one).
	const insJob = `
INSERT INTO job_queue (id, execution_id, tool_id, status, position, created_at)
VALUES (?, ?, ?, ?,
  (SELECT COUNT(*) + 1 FROM job_queue WHERE status IN ('pending','running')),
  ?);`
	if _, err := tx.ExecContext(ctx, insJob,
		jobID, execID, toolID, string(JobPending), now,
	); err != nil {
		return Job{}, fmt.Errorf("executor: insert job: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return Job{}, fmt.Errorf("executor: commit: %w", err)
	}

	pos, _ := q.positionOf(ctx, jobID)
	return Job{
		JobID:       jobID,
		ExecutionID: execID,
		Status:      JobPending,
		Position:    pos,
		Language:    req.Language,
		CreatedAt:   now.Format(time.RFC3339),
	}, nil
}

// Get returns the full state of a job, joining queue metadata with execution
// results.
func (q *Queue) Get(ctx context.Context, jobID string) (Job, error) {
	if q.db == nil {
		return Job{}, errors.New("executor: queue has no database")
	}
	const query = `
SELECT j.id, j.execution_id, j.status, COALESCE(j.position, 0),
       COALESCE(e.language, ''), e.exit_code, COALESCE(e.stdout, ''),
       COALESCE(e.stderr, ''), COALESCE(j.error_message, ''),
       COALESCE(j.created_at, ''), COALESCE(j.started_at, ''),
       COALESCE(j.completed_at, '')
FROM job_queue j
JOIN executions e ON e.id = j.execution_id
WHERE j.id = ?;`

	var (
		job      Job
		status   string
		exitCode sql.NullInt64
	)
	err := q.db.QueryRowContext(ctx, query, jobID).Scan(
		&job.JobID, &job.ExecutionID, &status, &job.Position, &job.Language,
		&exitCode, &job.Stdout, &job.Stderr, &job.ErrorMessage,
		&job.CreatedAt, &job.StartedAt, &job.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("executor: get job: %w", err)
	}
	job.Status = JobStatus(status)
	if exitCode.Valid {
		v := int(exitCode.Int64)
		job.ExitCode = &v
	}
	return job, nil
}

// Cancel marks a pending or running job (and its execution) as cancelled. It
// returns ErrJobNotCancellable for jobs already in a terminal state.
func (q *Queue) Cancel(ctx context.Context, jobID string) (Job, error) {
	if q.db == nil {
		return Job{}, errors.New("executor: queue has no database")
	}
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, fmt.Errorf("executor: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var status, execID string
	err = tx.QueryRowContext(ctx,
		`SELECT status, execution_id FROM job_queue WHERE id = ?;`, jobID,
	).Scan(&status, &execID)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	if err != nil {
		return Job{}, fmt.Errorf("executor: lookup job: %w", err)
	}

	switch JobStatus(status) {
	case JobCompleted, JobFailed, JobCancelled:
		return Job{}, ErrJobNotCancellable
	}

	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx,
		`UPDATE job_queue SET status = ?, completed_at = ?, error_message = ? WHERE id = ?;`,
		string(JobCancelled), now, "cancelled by request", jobID,
	); err != nil {
		return Job{}, fmt.Errorf("executor: cancel job: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE executions SET status = ?, ended_at = ? WHERE id = ?;`,
		string(JobCancelled), now, execID,
	); err != nil {
		return Job{}, fmt.Errorf("executor: cancel execution: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Job{}, fmt.Errorf("executor: commit: %w", err)
	}

	return Job{
		JobID:       jobID,
		ExecutionID: execID,
		Status:      JobCancelled,
		CompletedAt: now.Format(time.RFC3339),
	}, nil
}

// ClaimNext atomically transitions the oldest pending job to running and
// returns its identifiers plus the stored request. It returns sql.ErrNoRows
// when the queue is empty.
func (q *Queue) ClaimNext(ctx context.Context) (jobID, execID string, req ExecRequest, err error) {
	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", ExecRequest{}, fmt.Errorf("executor: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const sel = `
SELECT j.id, j.execution_id, COALESCE(e.language,''), COALESCE(e.code,''),
       COALESCE(e.timeout_seconds,30), COALESCE(e.cpu_limit_percent,100),
       COALESCE(e.memory_limit_mb,512)
FROM job_queue j
JOIN executions e ON e.id = j.execution_id
WHERE j.status = 'pending'
ORDER BY j.created_at ASC
LIMIT 1;`
	err = tx.QueryRowContext(ctx, sel).Scan(
		&jobID, &execID, &req.Language, &req.Code,
		&req.Timeout, &req.CPULimitPercent, &req.MemoryLimitMB,
	)
	if err != nil {
		return "", "", ExecRequest{}, err
	}

	now := time.Now().UTC()
	if _, err = tx.ExecContext(ctx,
		`UPDATE job_queue SET status = 'running', started_at = ? WHERE id = ? AND status = 'pending';`,
		now, jobID,
	); err != nil {
		return "", "", ExecRequest{}, fmt.Errorf("executor: claim job: %w", err)
	}
	if _, err = tx.ExecContext(ctx,
		`UPDATE executions SET status = 'running', started_at = ? WHERE id = ?;`,
		now, execID,
	); err != nil {
		return "", "", ExecRequest{}, fmt.Errorf("executor: mark execution running: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", "", ExecRequest{}, fmt.Errorf("executor: commit claim: %w", err)
	}
	return jobID, execID, req, nil
}

// Complete records the terminal result of a job and its execution.
func (q *Queue) Complete(ctx context.Context, jobID, execID string, res ExecResponse, runErr error) error {
	now := time.Now().UTC()
	status := JobCompleted
	errMsg := ""
	exitCode := res.ExitCode
	if runErr != nil {
		status = JobFailed
		errMsg = runErr.Error()
	} else if res.Status == "error" || res.Status == "timeout" {
		status = JobFailed
		errMsg = res.Error
	}

	tx, err := q.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("executor: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`UPDATE job_queue SET status = ?, completed_at = ?, result_code = ?, error_message = ? WHERE id = ?;`,
		string(status), now, exitCode, nullIfEmpty(errMsg), jobID,
	); err != nil {
		return fmt.Errorf("executor: update job: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE executions SET status = ?, ended_at = ?, stdout = ?, stderr = ?, exit_code = ?, error_message = ? WHERE id = ?;`,
		string(status), now, res.Stdout, res.Stderr, exitCode, nullIfEmpty(errMsg), execID,
	); err != nil {
		return fmt.Errorf("executor: update execution: %w", err)
	}
	return tx.Commit()
}

// positionOf returns the 1-based queue position of a pending job.
func (q *Queue) positionOf(ctx context.Context, jobID string) (int, error) {
	const query = `
SELECT COUNT(*) FROM job_queue
WHERE status = 'pending'
  AND created_at <= (SELECT created_at FROM job_queue WHERE id = ?);`
	var pos int
	err := q.db.QueryRowContext(ctx, query, jobID).Scan(&pos)
	return pos, err
}

// nullIfEmpty maps an empty string to a SQL NULL.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
