package registry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// healthProbeTimeout bounds a single tool health check.
const healthProbeTimeout = 3 * time.Second

// Registry provides in-memory + persistent tool registration and discovery.
// The DB is the source of truth; the in-memory cache accelerates reads.
type Registry struct {
	db    *sql.DB
	mu    sync.RWMutex
	cache map[string]Tool
}

// New creates a Registry backed by the provided database handle.
func New(db *sql.DB) *Registry {
	return &Registry{
		db:    db,
		cache: make(map[string]Tool),
	}
}

// Register inserts or updates a tool by name, persisting it and updating cache.
// If the tool has no ID it is assigned a new UUID.
func (r *Registry) Register(ctx context.Context, t Tool) (Tool, error) {
	if t.Name == "" {
		return Tool{}, errors.New("registry: tool name is required")
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.Status == "" {
		t.Status = StatusPending
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = now

	const q = `
INSERT INTO tools (id, name, description, category, status, health_check_url, container_name, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  name=excluded.name,
  description=excluded.description,
  category=excluded.category,
  status=excluded.status,
  health_check_url=excluded.health_check_url,
  container_name=excluded.container_name,
  updated_at=excluded.updated_at;`

	if r.db != nil {
		if _, err := r.db.ExecContext(ctx, q,
			t.ID, t.Name, t.Description, t.Category, string(t.Status),
			t.HealthCheckURL, t.ContainerName, t.CreatedAt, t.UpdatedAt,
		); err != nil {
			return Tool{}, fmt.Errorf("registry: persist tool: %w", err)
		}
	}

	r.mu.Lock()
	r.cache[t.ID] = t
	r.mu.Unlock()
	return t, nil
}

// List returns all registered tools from the database (or cache if no DB).
func (r *Registry) List(ctx context.Context) ([]Tool, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		out := make([]Tool, 0, len(r.cache))
		for _, t := range r.cache {
			out = append(out, t)
		}
		return out, nil
	}

	const q = `
SELECT id, name, COALESCE(description,''), COALESCE(category,''), COALESCE(status,'pending'),
       COALESCE(health_check_url,''), COALESCE(container_name,''), created_at, updated_at
FROM tools ORDER BY name;`

	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("registry: list: %w", err)
	}
	defer rows.Close()

	var out []Tool
	for rows.Next() {
		var t Tool
		var status string
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Category, &status,
			&t.HealthCheckURL, &t.ContainerName, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("registry: scan: %w", err)
		}
		t.Status = ToolStatus(status)
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetByName returns a single tool by its logical name.
func (r *Registry) GetByName(ctx context.Context, name string) (Tool, bool, error) {
	r.mu.RLock()
	for _, t := range r.cache {
		if t.Name == name {
			r.mu.RUnlock()
			return t, true, nil
		}
	}
	r.mu.RUnlock()

	if r.db == nil {
		return Tool{}, false, nil
	}

	const q = `
SELECT id, name, COALESCE(description,''), COALESCE(category,''), COALESCE(status,'pending'),
       COALESCE(health_check_url,''), COALESCE(container_name,''), created_at, updated_at
FROM tools WHERE name=?;`

	var t Tool
	var status string
	err := r.db.QueryRowContext(ctx, q, name).Scan(&t.ID, &t.Name, &t.Description,
		&t.Category, &status, &t.HealthCheckURL, &t.ContainerName, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Tool{}, false, nil
	}
	if err != nil {
		return Tool{}, false, fmt.Errorf("registry: get by name: %w", err)
	}
	t.Status = ToolStatus(status)
	return t, true, nil
}

// DefaultTools describes the tool services shipped with the gateway. Health
// check URLs point at the internal service endpoints.
func DefaultTools() []Tool {
	return []Tool{
		{
			Name:           "search",
			Description:    "Web/meta search via SearXNG aggregator",
			Category:       "search",
			HealthCheckURL: "http://search:8888/",
			ContainerName:  "toolset-search",
		},
		{
			Name:           "files",
			Description:    "Sandboxed file operations (read/write/list/delete/move)",
			Category:       "files",
			HealthCheckURL: "http://files-server:8765/health",
			ContainerName:  "toolset-files-server",
		},
		{
			Name:           "exec-light",
			Description:    "Sandboxed code execution: python, node, bash, c, cpp, assembly",
			Category:       "exec",
			HealthCheckURL: "http://exec-light:8765/health",
			ContainerName:  "toolset-exec-light",
		},
		{
			Name:           "exec-heavy",
			Description:    "Sandboxed code execution: dotnet/csharp, java, rust",
			Category:       "exec",
			HealthCheckURL: "http://exec-heavy:8765/health",
			ContainerName:  "toolset-exec-heavy",
		},
	}
}

// RegisterDefaults registers the built-in tool services, probing each one's
// health endpoint to set an initial status. Probe failures are non-fatal: the
// tool is registered with StatusError so it appears in the registry.
func (r *Registry) RegisterDefaults(ctx context.Context) error {
	for _, t := range DefaultTools() {
		if existing, ok, err := r.GetByName(ctx, t.Name); err == nil && ok {
			t.ID = existing.ID
			t.CreatedAt = existing.CreatedAt
		}
		t.Status = StatusReady
		if t.HealthCheckURL != "" && !r.probeURL(ctx, t.HealthCheckURL) {
			t.Status = StatusError
		}
		if _, err := r.Register(ctx, t); err != nil {
			return fmt.Errorf("registry: register default %q: %w", t.Name, err)
		}
	}
	return nil
}

// ProbeToolHealth hits the health endpoint of the named tool and updates its
// status in the registry. It returns the resulting status.
func (r *Registry) ProbeToolHealth(ctx context.Context, name string) (ToolStatus, error) {
	t, ok, err := r.GetByName(ctx, name)
	if err != nil {
		return StatusError, err
	}
	if !ok {
		return StatusError, fmt.Errorf("registry: tool %q not found", name)
	}
	status := StatusReady
	if t.HealthCheckURL == "" || !r.probeURL(ctx, t.HealthCheckURL) {
		status = StatusError
	}
	if status != t.Status {
		t.Status = status
		if _, err := r.Register(ctx, t); err != nil {
			return status, err
		}
	}
	return status, nil
}

// probeURL performs a best-effort GET and reports whether it returns 2xx.
func (r *Registry) probeURL(ctx context.Context, rawURL string) bool {
	ctx, cancel := context.WithTimeout(ctx, healthProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// Get returns a single tool by ID.
func (r *Registry) Get(ctx context.Context, id string) (Tool, bool, error) {
	r.mu.RLock()
	if t, ok := r.cache[id]; ok {
		r.mu.RUnlock()
		return t, true, nil
	}
	r.mu.RUnlock()

	if r.db == nil {
		return Tool{}, false, nil
	}

	const q = `
SELECT id, name, COALESCE(description,''), COALESCE(category,''), COALESCE(status,'pending'),
       COALESCE(health_check_url,''), COALESCE(container_name,''), created_at, updated_at
FROM tools WHERE id=?;`

	var t Tool
	var status string
	err := r.db.QueryRowContext(ctx, q, id).Scan(&t.ID, &t.Name, &t.Description,
		&t.Category, &status, &t.HealthCheckURL, &t.ContainerName, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Tool{}, false, nil
	}
	if err != nil {
		return Tool{}, false, fmt.Errorf("registry: get: %w", err)
	}
	t.Status = ToolStatus(status)
	return t, true, nil
}
