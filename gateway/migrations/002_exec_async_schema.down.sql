-- 002_exec_async_schema.down.sql
-- Reverse Phase 3 schema changes, preserving the base executions table.

DROP INDEX IF EXISTS idx_job_queue_created;
DROP INDEX IF EXISTS idx_job_queue_status;
DROP TABLE IF EXISTS job_queue;

DROP INDEX IF EXISTS idx_executions_job_id;

-- SQLite (>= 3.35) supports DROP COLUMN. Remove the columns added in the up
-- migration to restore the original executions table structure.
ALTER TABLE executions DROP COLUMN exit_code;
ALTER TABLE executions DROP COLUMN stderr;
ALTER TABLE executions DROP COLUMN stdout;
ALTER TABLE executions DROP COLUMN memory_limit_mb;
ALTER TABLE executions DROP COLUMN cpu_limit_percent;
ALTER TABLE executions DROP COLUMN timeout_seconds;
ALTER TABLE executions DROP COLUMN code;
ALTER TABLE executions DROP COLUMN language;
ALTER TABLE executions DROP COLUMN job_id;
