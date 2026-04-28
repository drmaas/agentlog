package agentlog

import "time"

type CommitLink struct {
	SessionID    string    `json:"session_id"`
	CommitSHA    string    `json:"commit_sha"`
	FilesChanged []string  `json:"files_changed,omitempty"`
	LinkedAt     time.Time `json:"linked_at"`
}
