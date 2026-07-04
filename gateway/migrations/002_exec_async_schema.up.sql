-- 002_exec_async_schema.up.sql
-- Phase 3: enhance the executions table for code execution + add an async job queue.

-- Enhance executions table with code-execution fields.
ALTER TABLE executions ADD COLUMN job_id TEXT;
ALTER TABLE executions ADD COLUMN language TEXT;
ALTER TABLE executions ADD COLUMN code TEXT;
ALTER TABLE executions ADD COLUMN timeout_seconds INT DEFAULT 30;
ALTER TABLE executions ADD COLUMN cpu_limit_percent INT DEFAULT 100;
ALTER TABLE executions ADD COLUMN memory_limit_mb INT DEFAULT 512;
ALTER TABLE executions ADD COLUMN stdout TEXT;
ALTER TABLE executions ADD COLUMN stderr TEXT;
ALTER TABLE executions ADD COLUMN exit_code INT;

-- SQLite cannot add a UNIQUE column via ALTER TABLE, so enforce job_id
-- uniqueness with a partial unique index (NULLs are allowed to repeat).
CREATE UNIQUE INDEX idx_executions_job_id ON executions(job_id) WHERE job_id IS NOT NULL;

-- Job queue table for async submissions.
CREATE TABLE job_queue (
  id TEXT PRIMARY KEY,
  execution_id TEXT NOT NULL UNIQUE,
  tool_id TEXT NOT NULL,
  status TEXT DEFAULT 'pending', -- pending, running, completed, failed, cancelled
  position INT,                  -- queue position
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  started_at DATETIME,
  completed_at DATETIME,
  result_code INT,               -- exit code
  error_message TEXT,
  FOREIGN KEY(execution_id) REFERENCES executions(id),
  FOREIGN KEY(tool_id) REFERENCES tools(id)
);

CREATE INDEX idx_job_queue_status ON job_queue(status);
CREATE INDEX idx_job_queue_created ON job_queue(created_at);
