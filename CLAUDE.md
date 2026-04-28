# AgentLog — Agent Guide

## Project Overview

AgentLog is a prompt audit log that links coding agent sessions (user requests + agent summaries) to git commits. It answers: *what was requested and what changed?* Git captures *what* changed; AgentLog captures *why*.

## Prerequisites

- **Go 1.21+** (check with `go version`)
- **Git** (for commit history and hook installation)
- **Bash** (for install and hook scripts)

## Quick Start

First-time contributors:

```bash
# Clone and navigate
git clone https://github.com/drmaas/agentlog.git
cd agentlog

# Build the binary
go build ./cmd/agentlog/...

# Run tests to verify setup
go test ./...

# Install locally to current project
./agentlog install
```

For a global installation (affects all projects):

```bash
./agentlog install --global
```

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

## Testing the MCP Server Locally

To test the MCP server without a full agent setup:

```bash
# Build the binary
go build ./cmd/agentlog/...

# Start the MCP server (runs on stdin/stdout)
./agentlog mcp

# In another terminal, send a JSON-RPC 2.0 request
echo '{"jsonrpc":"2.0","method":"initialize","params":{},"id":1}' | ./agentlog mcp
```

For manual testing with tools:

```bash
# Start server
./agentlog mcp

# Send tool call (e.g., GetSessions)
echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"GetSessions","arguments":{}},"id":2}' | ./agentlog mcp
```

Verify the MCP server in integration tests with `internal/mcp/*_test.go`.

## Testing Install & Global Setup

**⚠️ CRITICAL**: When testing `agentlog install --global`, NEVER delete `~/.agentlog`, `~/.claude`, or any other global user directories. These are user-level directories shared across all projects and all tools. Deleting them will break the user's entire setup and any other tools that depend on them.

**Safe testing approach for `--global` installs:**

1. **Use a temporary home directory** (preferred):
   ```bash
   # Create an isolated test environment
   TEST_HOME=$(mktemp -d)
   export HOME="$TEST_HOME"
   # Run install and tests
   ./agentlog install --global
   # Clean up after
   rm -rf "$TEST_HOME"
   ```

2. **Use environment variable overrides**:
   - Override `AGENTLOG_HOME` or `XDG_DATA_HOME` to point to a test directory instead of `~/.agentlog`
   - This allows testing installation logic without touching actual user directories

3. **Mock the directories in tests**:
   - Use Go's `os.TempDir()` and test fixtures
   - Mock the home directory lookup to return a test path
   - Verify files are created/linked in the right places without actually using user directories

4. **Use container/VM isolation**:
   - Run tests in Docker or a virtual machine where it's safe to modify a test user's home
   - Ensures complete isolation

5. **Verify with `--dry-run` or introspection** (if implemented):
   - Check what would be installed without actually installing
   - Inspect file paths and permissions without modifying the system

**For local/project-level installs** (the default, without `--global`):
- It is safe to delete `.agentlog/` and `.claude/` within test repos since they are project-specific and not shared

**Key principle**: Global state affects all users and all projects. Test it in isolation, never by modifying actual user directories.

## Runtime File Layout

```
.agentlog/
    config.yaml              # backend config
    active_session.json      # current session ID (if active)
    index.md                 # session summary table (committable)
    sessions/                # one .md per session (gitignored)
    skill/                   # SKILL.md + references/ (committable)
```
