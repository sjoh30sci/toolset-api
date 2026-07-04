-- 003_browser_sessions.down.sql
-- Phase 4 rollback: drop the browser session tracking table and its indexes
-- (indexes are dropped implicitly with the table).

DROP TABLE browser_sessions;
