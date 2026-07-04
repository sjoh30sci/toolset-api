// Package registry manages tool registration and discovery.
package registry

import "time"

// ToolStatus represents the lifecycle state of a tool.
type ToolStatus string

const (
	StatusPending ToolStatus = "pending"
	StatusReady   ToolStatus = "ready"
	StatusError   ToolStatus = "error"
)

// Tool describes a registered downstream tool service.
type Tool struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Description    string     `json:"description"`
	Category       string     `json:"category"` // search, files, exec, browser
	Status         ToolStatus `json:"status"`
	HealthCheckURL string     `json:"health_check_url,omitempty"`
	ContainerName  string     `json:"container_name,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
