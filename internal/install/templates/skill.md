---
name: agentlog
description: >
  Audit trail for coding sessions. Use this skill whenever starting a coding
  session, completing a unit of work, or ending a session in a git repository.
compatibility: Requires agentlog MCP server to be registered. Works with any MCP-compatible agent.
metadata:
  version: "0.1.0"
  repository: "https://github.com/drmaas/agentlog"
allowed-tools: agentlog_start agentlog_log agentlog_end agentlog_query agentlog_show agentlog_status
---

# AgentLog — Session Audit Trail

Use `agentlog_start` at session start, `agentlog_log` after each completed unit of work, and `agentlog_end` when done.

When asked what led to a commit/file state, use `agentlog_query` and `agentlog_show`.
