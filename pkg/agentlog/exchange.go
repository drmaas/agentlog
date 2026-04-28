package agentlog

import "time"

type Exchange struct {
	ExchangeID      string            `json:"exchange_id"`
	Index           int               `json:"index"`
	Timestamp       time.Time         `json:"timestamp"`
	Request         string            `json:"request"`
	ResponseSummary string            `json:"response_summary"`
	FilesChanged    []string          `json:"files_changed,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}
