package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/drmaas/agentlog/pkg/agentlog"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Serve(ctx context.Context, cwd string) error {
	manager, err := agentlog.NewManager(ctx, cwd)
	if err != nil {
		return err
	}
	defer manager.Close()

	scanner := bufio.NewScanner(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)

	for scanner.Scan() {
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = encoder.Encode(response{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		var id interface{}
		_ = json.Unmarshal(req.ID, &id)
		res, rpcErr := handle(ctx, manager, req)
		if rpcErr != nil {
			_ = encoder.Encode(response{JSONRPC: "2.0", ID: id, Error: rpcErr})
		} else {
			_ = encoder.Encode(response{JSONRPC: "2.0", ID: id, Result: res})
		}
	}
	return scanner.Err()
}

func handle(ctx context.Context, manager *agentlog.Manager, req request) (interface{}, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "agentlog",
				"version": "0.1.0",
			},
		}, nil
	case "tools/list":
		return map[string]interface{}{"tools": Tools()}, nil
	case "tools/call":
		var p struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid params"}
		}
		out, err := callTool(ctx, manager, p.Name, p.Arguments)
		if err != nil {
			return map[string]interface{}{
				"isError": true,
				"content": []map[string]string{{"type": "text", "text": err.Error()}},
			}, nil
		}
		return map[string]interface{}{
			"isError": false,
			"content": []map[string]string{{"type": "text", "text": out}},
		}, nil
	case "ping":
		return map[string]string{"ok": "true"}, nil
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func callTool(ctx context.Context, manager *agentlog.Manager, name string, args map[string]interface{}) (string, error) {
	switch name {
	case "agentlog_start":
		user, _ := args["user_id"].(string)
		session, err := manager.Start(ctx, agentlog.StartOpts{UserID: user})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("AgentLog session started: %s", session.SessionID), nil
	case "agentlog_log":
		sessionID, _ := args["session_id"].(string)
		request, _ := args["request"].(string)
		summary, _ := args["response_summary"].(string)
		files := toStringSlice(args["files_changed"])
		ex, err := manager.Log(ctx, agentlog.LogOpts{
			SessionID:       sessionID,
			Request:         request,
			ResponseSummary: summary,
			FilesChanged:    files,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Exchange logged: %s", ex.ExchangeID), nil
	case "agentlog_end":
		sessionID, _ := args["session_id"].(string)
		s, links, err := manager.End(ctx, sessionID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("AgentLog session ended: %s (%d commits linked)", s.SessionID, len(links)), nil
	case "agentlog_status":
		id, s, err := manager.Status(ctx)
		if err != nil {
			return "", err
		}
		if id == "" || s == nil {
			return "No active session", nil
		}
		return fmt.Sprintf("Active session: %s (user=%s branch=%s)", id, s.UserID, s.Branch), nil
	case "agentlog_query":
		var filter agentlog.SessionFilter
		if v, ok := args["user"].(string); ok {
			filter.UserID = v
		}
		if v, ok := args["branch"].(string); ok {
			filter.Branch = v
		}
		if v, ok := args["since"].(string); ok && v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err == nil {
				filter.Since = &t
			}
		}
		if v, ok := args["until"].(string); ok && v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err == nil {
				filter.Until = &t
			}
		}
		sha, _ := args["sha"].(string)
		file, _ := args["file"].(string)
		text, _ := args["text"].(string)
		sessions, exchanges, err := manager.Query(ctx, filter, sha, file, text)
		if err != nil {
			return "", err
		}
		if exchanges != nil {
			return fmt.Sprintf("Found %d exchanges", len(exchanges)), nil
		}
		return fmt.Sprintf("Found %d sessions", len(sessions)), nil
	case "agentlog_show":
		sessionID, _ := args["session_id"].(string)
		s, ex, links, err := manager.Show(ctx, sessionID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Session %s: %d exchanges, %d commits", s.SessionID, len(ex), len(links)), nil
	default:
		return "", fmt.Errorf("unknown tool %q", name)
	}
}

func toStringSlice(v interface{}) []string {
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
