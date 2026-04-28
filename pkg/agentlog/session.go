package agentlog

import "time"

type Session struct {
	SessionID    string            `json:"session_id"`
	UserID       string            `json:"user_id"`
	StartedAt    time.Time         `json:"started_at"`
	EndedAt      *time.Time        `json:"ended_at,omitempty"`
	GitSHABefore string            `json:"git_sha_before"`
	GitSHAAfter  string            `json:"git_sha_after,omitempty"`
	Repository   string            `json:"repository"`
	Branch       string            `json:"branch"`
	Tags         []string          `json:"tags,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

type SessionFilter struct {
	UserID string
	Branch string
	Since  *time.Time
	Until  *time.Time
	Tags   []string
	Limit  int
	Offset int
}
