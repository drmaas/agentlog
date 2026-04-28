# AgentLog — Agent Guide

## Project Overview

AgentLog is a prompt audit log that links coding agent sessions (user requests + agent summaries) to git commits. It answers: *what was requested and what changed?* Git captures *what* changed; AgentLog captures *why*.

## Build & Test

```bash
go build ./cmd/agentlog/...   # build binary
go test ./...                  # run all tests
go vet ./...                   # lint
```

Zero external dependencies — pure Go standard library only.

## Architecture

```
cmd/agentlog/        CLI entrypoint; main.go registers markdown backend via side-effect import
pkg/agentlog/        Public core library: interfaces, data structs, manager, registry, git helpers
internal/backends/   Storage backend implementations (currently: markdown)
internal/install/    Install logic (hook, skill, config, gitignore) and embedded templates
internal/mcp/        JSON-RPC 2.0 MCP server over stdin/stdout
hooks/               Git hook (prepare-commit-msg) embedded and installed by `agentlog install`
```

## Key Interfaces & Patterns

### StorageBackend

All storage is behind the `StorageBackend` interface in `pkg/agentlog/backend.go`. Backends register themselves via `init()`:

```go
func init() {
    agentlog.RegisterBackend("markdown", func() agentlog.StorageBackend {
        return &Backend{}
    })
}
```

Add new backend types by implementing `StorageBackend` and registering with `RegisterBackend`. The active backend is selected from `config.yaml`; the first entry that initializes successfully wins.

### Data Model

- **Session** — one complete agent interaction; identified by `sess-{unixnano}`
- **Exchange** — one request→response pair within a session; identified by `{session_id}-{index:03d}`
- **CommitLink** — maps a session to a git commit SHA
- **TraceRef** — optional link to an observability backend (Phoenix, LangFuse, LangSmith, Datadog, OTEL)

### Manager

`pkg/agentlog/manager.go` is the central orchestrator. All CLI commands and MCP tools go through `Manager`. It holds the active `StorageBackend`, git helpers, and config. The active session ID is persisted in `.agentlog/active_session.json`.

### MCP Server

`internal/mcp/` implements the MCP stdio transport (JSON-RPC 2.0, protocol version `2024-11-05`). Tools are defined in `tools.go`; dispatch is in `server.go`. Each tool maps 1:1 to a `Manager` method.

## Conventions

- **No external dependencies.** Keep the module dependency-free.
- **Relative paths** in `config.yaml` are resolved relative to the repo root (discovered via git).
- **Session files are gitignored** by design (`.agentlog/sessions/`). `index.md` and skill files are not.
- **File naming** for session files: `{timestamp}_{user}_{sha7}_{session_id}.md` — sortable chronologically.
- **TraceRef auto-discovery** checks env vars in priority order before falling back to zero value; see `pkg/agentlog/traceref.go`.

## Adding a New CLI Command

1. Add a handler function in `cmd/agentlog/cli.go`
2. Add a new `case` in the `switch` in `run()` (also `cli.go`)
3. Wire any new `Manager` method needed in `pkg/agentlog/manager.go`

## Adding a New MCP Tool

1. Define the tool schema in `internal/mcp/tools.go`
2. Add dispatch in `internal/mcp/server.go` → `callTool()`
3. The underlying logic should live in `Manager`

## Runtime File Layout

```
.agentlog/
    config.yaml              # backend config
    active_session.json      # current session ID (if active)
    index.md                 # session summary table (committable)
    sessions/                # one .md per session (gitignored)
    skill/                   # SKILL.md + references/ (committable)
```
