# AgentLog

A prompt audit log that links coding agent sessions to git commits. Git captures *what* changed — AgentLog captures *why*.

## Overview

When a coding agent (Claude, Copilot, Cursor, etc.) makes changes in your repository, AgentLog records the conversation: the user's request, the agent's response summary, and the files touched — then ties that record to the git commit that followed. The result is a human-readable, queryable history of *intent* alongside your code history.

```
.agentlog/
    index.md          ← session summary table
    sessions/
        2026-04-13T10-00-00Z_alice_abc1234_sess-12345.md
```

## Features

- **Zero dependencies** — pure Go, single static binary
- **Markdown storage** — session files are plain `.md`, readable without tooling
- **Git integration** — auto-links sessions to commits via a `prepare-commit-msg` hook
- **MCP server** — agents can call AgentLog directly via the Model Context Protocol
- **Extensible backends** — implement `StorageBackend` to store sessions anywhere

## Installation

The fastest way to install is the remote installer:

```bash
curl -fsSL https://raw.githubusercontent.com/drmaas/agentlog/main/scripts/install.sh | sh
```

It detects your platform and uses prebuilt release binaries when available, falling back to `go install` if needed.

Once installed, initialize AgentLog in your git repository:

```bash
agentlog install
```

This sets up a git hook, configuration, and a skill file for integration with coding agents.

## Usage

### CLI

```bash
# Start a new session
agentlog start [--user alice] [--tags feature,bugfix]

# Log an exchange (request + agent summary)
agentlog log \
  --request "Add retry logic to the HTTP client" \
  --summary "Added exponential backoff with jitter in pkg/http/client.go" \
  --files pkg/http/client.go

# End the session (links any commits made during the session)
agentlog end

# Check if a session is active
agentlog status

# Query sessions
agentlog query --file pkg/http/client.go
agentlog query --text "retry"
agentlog query --sha abc1234
agentlog query --user alice --since 2026-01-01T00:00:00Z --limit 20

# Show a full session
agentlog show sess-1712345678000000000
```

### MCP Server

Run AgentLog as an MCP server so agents can call it directly:

```bash
agentlog mcp-serve
```

Add to your MCP client config (e.g. Claude Desktop `mcp_config.json`):

```json
{
  "mcpServers": {
    "agentlog": {
      "command": "agentlog",
      "args": ["mcp-serve"]
    }
  }
}
```

Available MCP tools: `agentlog_start`, `agentlog_log`, `agentlog_end`, `agentlog_status`, `agentlog_query`, `agentlog_show`.

## Configuration

`.agentlog/config.yaml` is created automatically on `install`. Default:

```yaml
version: 1
user_id: alice          # defaults to $USER
backends:
  - type: markdown
    path: .agentlog/sessions
```

`path` is resolved relative to the repository root. The first backend that initializes successfully is used.

## How It Works

1. **`agentlog start`** — creates a session record, snaps the current `HEAD`
2. Agent makes changes and the user commits — the git hook appends `AgentLog-Session: sess-xxx` to the commit message
3. **`agentlog log`** — records each request/response exchange inside the session
4. **`agentlog end`** — closes the session, snaps `HEAD` again, and links any commits made between start and end
5. **`agentlog query`** — scans session files and returns matches by commit SHA, file path, text, user, branch, or time range

## Custom Backends

Implement `agentlog.StorageBackend` from `pkg/agentlog/backend.go` and register it:

```go
import "github.com/drmaas/agentlog/pkg/agentlog"

func init() {
    agentlog.RegisterBackend("mybackend", func() agentlog.StorageBackend {
        return &MyBackend{}
    })
}
```

Then add it to `config.yaml`:

```yaml
backends:
  - type: mybackend
```

## Development

### Building from Source

Clone the repository and build locally:

```bash
git clone https://github.com/drmaas/agentlog
cd agentlog
go build ./cmd/agentlog/...
go test ./...
```

Run the locally-built binary:

```bash
./agentlog install
```

### Code Organization

- `cmd/agentlog/` — CLI entrypoint
- `pkg/agentlog/` — Core library interfaces and types
- `internal/backends/` — Storage backend implementations
- `internal/mcp/` — MCP server over stdin/stdout
- `hooks/` — Git hook embedded by `agentlog install`

### Custom Backends

Implement the `StorageBackend` interface and register with `RegisterBackend()`:

```go
type StorageBackend interface {
    StartSession(...) error
    LogExchange(...) error
    LinkCommit(...) error
    // ... other methods
}

func init() {
    agentlog.RegisterBackend("mybackend", func() agentlog.StorageBackend {
        return &Backend{}
    })
}
```

## Release Process

Tagged releases automatically publish cross-platform binaries via GitHub Actions.

### For Maintainers

Create and push a semantic version tag to trigger the release workflow:

```bash
git tag v0.0.1
git push origin v0.0.1
```

The workflow builds binaries for:
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Published assets are named `agentlog_{os}_{arch}.tar.gz` and are automatically available to the remote installer.

## License

See [LICENSE](LICENSE).
