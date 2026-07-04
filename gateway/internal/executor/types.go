// Package executor proxies code-execution requests from the gateway to the
// exec-light / exec-heavy sandbox containers and implements the async job
// queue backed by SQLite.
package executor

// ExecRequest is the code-execution request accepted by the gateway. It mirrors
// the executor-server contract with additional resource-limit hints persisted
// for async jobs.
type ExecRequest struct {
	Code            string            `json:"code"`
	Language        string            `json:"language"`
	Timeout         int               `json:"timeout,omitempty"` // seconds
	Stdin           string            `json:"stdin,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	CPULimitPercent int               `json:"cpu_limit_percent,omitempty"`
	MemoryLimitMB   int               `json:"memory_limit_mb,omitempty"`
}

// ExecResponse is the synchronous execution result returned by the sandbox.
type ExecResponse struct {
	Status   string  `json:"status"` // success, timeout, error
	ExitCode int     `json:"exit_code"`
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Duration float64 `json:"duration_seconds"`
	Error    string  `json:"error,omitempty"`
}

// Tier identifies which sandbox image should handle a language.
type Tier string

const (
	TierLight Tier = "light"
	TierHeavy Tier = "heavy"
)

// JobStatus enumerates async job states.
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// Job is a queued async execution as surfaced to API callers.
type Job struct {
	JobID        string    `json:"job_id"`
	ExecutionID  string    `json:"execution_id"`
	Status       JobStatus `json:"status"`
	Position     int       `json:"position,omitempty"`
	Language     string    `json:"language"`
	ExitCode     *int      `json:"exit_code,omitempty"`
	Stdout       string    `json:"stdout,omitempty"`
	Stderr       string    `json:"stderr,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    string    `json:"created_at,omitempty"`
	StartedAt    string    `json:"started_at,omitempty"`
	CompletedAt  string    `json:"completed_at,omitempty"`
}

// defaultLangTiers maps each supported language to its sandbox tier.
var defaultLangTiers = map[string]Tier{
	"python":   TierLight,
	"node":     TierLight,
	"bash":     TierLight,
	"c":        TierLight,
	"cpp":      TierLight,
	"assembly": TierLight,
	"java":     TierHeavy,
	"rust":     TierHeavy,
	"csharp":   TierHeavy,
	"dotnet":   TierHeavy,
}

// TierFor returns the sandbox tier responsible for a language and whether the
// language is supported at all.
func TierFor(language string) (Tier, bool) {
	t, ok := defaultLangTiers[language]
	return t, ok
}

// SupportedLanguages returns the sorted-ish set of recognized languages.
func SupportedLanguages() []string {
	out := make([]string, 0, len(defaultLangTiers))
	for l := range defaultLangTiers {
		out = append(out, l)
	}
	return out
}
