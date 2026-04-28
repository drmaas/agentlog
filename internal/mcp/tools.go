package mcp

import "encoding/json"

type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func Tools() []Tool {
	return []Tool{
		{
			Name:        "agentlog_start",
			Description: "Start an AgentLog session for the current repository and return session_id.",
			InputSchema: rawSchema(`{"type":"object","properties":{"user_id":{"type":"string"},"tags":{"type":"array","items":{"type":"string"}}}}`),
		},
		{
			Name:        "agentlog_log",
			Description: "Record a completed exchange in the active session after finishing a user request.",
			InputSchema: rawSchema(`{"type":"object","required":["request","response_summary"],"properties":{"session_id":{"type":"string"},"request":{"type":"string"},"response_summary":{"type":"string"},"files_changed":{"type":"array","items":{"type":"string"}}}}`),
		},
		{
			Name:        "agentlog_end",
			Description: "End the active AgentLog session and finalize commit links.",
			InputSchema: rawSchema(`{"type":"object","properties":{"session_id":{"type":"string"}}}`),
		},
		{
			Name:        "agentlog_query",
			Description: "Query sessions by commit SHA, file path, text, or user/branch/time filters.",
			InputSchema: rawSchema(`{"type":"object","properties":{"sha":{"type":"string"},"file":{"type":"string"},"text":{"type":"string"},"user":{"type":"string"},"branch":{"type":"string"},"since":{"type":"string"},"until":{"type":"string"}}}`),
		},
		{
			Name:        "agentlog_show",
			Description: "Show full detail for a specific session id.",
			InputSchema: rawSchema(`{"type":"object","required":["session_id"],"properties":{"session_id":{"type":"string"}}}`),
		},
		{
			Name:        "agentlog_status",
			Description: "Check whether an AgentLog session is currently active.",
			InputSchema: rawSchema(`{"type":"object","properties":{}}`),
		},
	}
}

func rawSchema(v string) json.RawMessage { return json.RawMessage(v) }
