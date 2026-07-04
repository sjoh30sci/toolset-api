-- 003_browser_sessions.up.sql
-- Phase 4: browser automation. Tracks Playwright session lifecycle so the
-- gateway can surface session metadata and reconcile orphaned sessions.

CREATE TABLE browser_sessions (
  id TEXT PRIMARY KEY,
  tool_id TEXT NOT NULL,
  browser_type TEXT DEFAULT 'chromium', -- chromium, firefox, webkit
  status TEXT DEFAULT 'active',         -- active, idle, closed
  url TEXT,
  title TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_activity_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  closed_at DATETIME,
  FOREIGN KEY(tool_id) REFERENCES tools(id)
);

CREATE INDEX idx_browser_sessions_status ON browser_sessions(status);
CREATE INDEX idx_browser_sessions_created ON browser_sessions(created_at);
