CREATE TABLE tools (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  description TEXT,
  category TEXT, -- search, files, exec, browser
  status TEXT DEFAULT 'pending', -- pending, ready, error
  health_check_url TEXT,
  container_name TEXT,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE api_keys (
  id TEXT PRIMARY KEY,
  token TEXT UNIQUE NOT NULL,
  tool_id TEXT,
  rate_limit INT DEFAULT 100,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  expires_at DATETIME,
  FOREIGN KEY(tool_id) REFERENCES tools(id)
);

CREATE TABLE executions (
  id TEXT PRIMARY KEY,
  tool_id TEXT NOT NULL,
  tool_name TEXT,
  status TEXT, -- pending, running, success, error
  started_at DATETIME,
  ended_at DATETIME,
  error_message TEXT,
  FOREIGN KEY(tool_id) REFERENCES tools(id)
);

CREATE TABLE rate_limits (
  id TEXT PRIMARY KEY,
  token_or_ip TEXT NOT NULL,
  requests_count INT DEFAULT 0,
  window_start DATETIME DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(token_or_ip, window_start)
);

CREATE INDEX idx_tools_status ON tools(status);
CREATE INDEX idx_api_keys_token ON api_keys(token);
CREATE INDEX idx_executions_tool ON executions(tool_id);
