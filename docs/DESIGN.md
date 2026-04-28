# AgentLog — System Design

## Problem Statement

When coding agents modify a codebase, the **intent** behind changes is lost. Git captures *what* changed but not *why* — the human utterances, agent reasoning, and decision context that drove those changes. This makes it hard to:

- Audit who prompted what and when
- Understand *why* a function, file, or module looks the way it does
- Reconstruct the decision trail for a given commit or series of commits
- Onboard new contributors to the rationale behind architectural choices

AgentLog solves this by creating a **prompt audit log** — a change-data-capture (CDC) system that links user utterances and agent sessions to git commits, forming a bidirectional trail from intent to code and back.

## Goals

1. **Auditability**: Every coding agent session is recorded with full utterance history, tied to the git state before and after.
2. **Codebase Understanding**: Given any commit, file, or function, answer "what utterances led to this?"
3. **Pluggable Storage**: Default to Markdown files in the repo. Support arbitrary backends (JSON, SQLite, Postgres, MongoDB, ChromaDB, etc.) via a plugin interface.
4. **Minimal Friction**: Capture should be automatic or near-automatic — not a burden on the developer.

## Non-Goals (v1)

- Real-time streaming / event bus (batch capture is fine)
- UI / dashboard (CLI and file-based queries only)
- Multi-repo correlation (single repo at a time)
- Agent-side integration SDK (agents log via the CLI or library; we don't modify agents)

---

## Data Model

### Session

A session represents one interaction between a user and a coding agent.

| Field            | Type      | Description                                      |
|------------------|-----------|--------------------------------------------------|
| `session_id`     | `string`  | Unique identifier (UUID v4)                      |
| `user_id`        | `string`  | Identifier for the user who initiated the session |
| `started_at`     | `RFC3339` | Timestamp when the session began                 |
| `ended_at`       | `RFC3339` | Timestamp when the session ended (nullable)      |
| `git_sha_before` | `string`  | HEAD commit SHA at session start                 |
| `git_sha_after`  | `string`  | HEAD commit SHA at session end (nullable)        |
| `repository`     | `string`  | Repository identifier (e.g., `owner/repo`)       |
| `branch`         | `string`  | Git branch name at session start                 |
| `tags`           | `[]string`| Optional user-defined tags                       |
| `metadata`       | `map`     | Extensible key-value metadata                    |

### Exchange

An exchange is a logical request→response pair: what the user asked and what the agent did about it. This is **not** a full transcript — it captures the audit-relevant record of intent and outcome.

A session contains one or more exchanges. Each exchange pairs a user request with the agent's summary of actions taken. This keeps the log concise and audit-focused rather than replicating the full conversation.

| Field          | Type      | Description                                        |
|----------------|-----------|----------------------------------------------------|
| `exchange_id`  | `string`  | Unique identifier (`session_id` + sequential index) |
| `index`        | `int`     | 0-based sequential index within the session        |
| `timestamp`    | `RFC3339` | When the exchange started                          |
| `request`      | `string`  | The user's utterance / prompt (verbatim)           |
| `response_summary` | `string` | Brief agent-authored summary of what it did and why. Not the full response — just the key decisions, actions taken, and rationale. |
| `files_changed`| `[]string`| Files created, modified, or deleted in this exchange |
| `trace_ref`    | `TraceRef`| Optional link to observability trace (see below)   |
| `metadata`     | `map`     | Extensible key-value metadata                      |

### TraceRef (Observability Correlation)

AgentLog is a **logical audit record**, not an observability tool. But when observability tools like [Arize Phoenix](https://github.com/Arize-AI/phoenix), Langfuse, LangSmith, Datadog LLM Obs, or others are also running, each exchange can store a reference that lets you jump from the audit log into the full trace.

| Field          | Type     | Description                                        |
|----------------|----------|----------------------------------------------------|
| `trace_id`     | `string` | OpenTelemetry trace ID (128-bit hex). The primary correlation key. |
| `span_id`      | `string` | OpenTelemetry span ID for the specific exchange (optional — trace_id alone is usually sufficient). |
| `provider`     | `string` | Which observability tool holds the full trace: `phoenix`, `langfuse`, `langsmith`, `datadog`, `custom`, etc. |
| `url`          | `string` | Direct deep-link URL to the trace in the observability UI (optional, best-effort). |

#### How this works in practice

```
┌──────────────────────────────────────────────────────────────┐
│  AgentLog (audit layer)                                      │
│                                                              │
│  Exchange [0]                                                │
│    Request: "Add JWT auth to the API"                        │
│    Summary: "Added RS256 middleware in src/auth.go"           │
│    Files:   src/auth.go, src/auth_test.go                    │
│    TraceRef:                                                 │
│      trace_id: 4bf92f3577b34da6a3ce929d0e0e4736              │
│      provider: phoenix                                       │
│      url:      http://localhost:6006/traces/4bf92f...         │
│                         │                                    │
└─────────────────────────┼────────────────────────────────────┘
                          │  "show me the full trace"
                          ▼
┌──────────────────────────────────────────────────────────────┐
│  Phoenix / Langfuse / etc. (observability layer)             │
│                                                              │
│  Trace: 4bf92f3577b34da6a3ce929d0e0e4736                     │
│  ├─ LLM call: GPT-4 (2.3s, 4200 tokens)                     │
│  ├─ Tool: file_edit src/auth.go                              │
│  ├─ Tool: bash "go test ./..."                               │
│  ├─ LLM call: GPT-4 (1.1s, 1800 tokens)                     │
│  └─ Tool: file_edit src/auth_test.go                         │
│                                                              │
│  Full token counts, latencies, tool I/O, error traces...     │
└──────────────────────────────────────────────────────────────┘
```

AgentLog answers **"what was requested and what changed?"** — the observability tool answers **"how did the agent accomplish it?"** The `trace_id` is the bridge between the two.

#### Why trace_id is the right correlation key

Most agent observability tools converge on OpenTelemetry:
- **Phoenix** uses OpenInference (OTel-based semantic conventions)
- **Langfuse** supports OTel trace ingestion and native trace IDs
- **LangSmith** has its own `run_id` but supports OTel export
- **Datadog LLM Obs** is built on OTel

By storing `trace_id`, AgentLog stays tool-agnostic. The `provider` field tells you *where* to look, and the optional `url` gives a direct deep-link.

For tools that use non-OTel IDs (e.g., LangSmith `run_id`), store those in `metadata`:

```yaml
trace_ref:
  trace_id: ""           # may be empty if tool doesn't use OTel
  provider: langsmith
  url: https://smith.langchain.com/runs/abc-123
metadata:
  langsmith_run_id: abc-123
```

#### TraceRef discovery (zero-config)

TraceRef is **always optional**. Most users won't have an observability tool running, and AgentLog must work perfectly without one. TraceRef is populated through a priority chain — the first match wins:

1. **Explicit** — caller passes `trace_id` via library API or CLI flag (`--trace-id`, `--trace-provider`). Always takes precedence.
2. **Environment sniffing** — if not explicitly provided, AgentLog checks well-known environment variables that observability tools set automatically when active:

| Variable              | Provider   | Notes                                    |
|-----------------------|------------|------------------------------------------|
| `TRACEPARENT`         | (any OTel) | W3C standard; parse trace-id from it     |
| `OTEL_TRACE_ID`       | (any OTel) | Some instrumentations export this        |
| `PHOENIX_TRACE_ID`    | Phoenix    | Set by Phoenix auto-instrumentation      |
| `LANGFUSE_TRACE_ID`   | Langfuse   | Set by Langfuse SDK                      |
| `LANGSMITH_RUN_ID`    | LangSmith  | Set by LangSmith tracing                 |

The `provider` field is inferred from which variable matched. If `TRACEPARENT` is found, `provider` defaults to `"otel"` unless a more specific variable (e.g., `PHOENIX_TRACE_ID`) is also present.

3. **Omitted** — if no trace context is found, `trace_ref` is simply absent from the exchange. No empty fields, no error, no config needed.

```
User has Phoenix running?
  → TRACEPARENT / PHOENIX_TRACE_ID are in the environment
  → AgentLog auto-populates trace_ref
  → Markdown renders: **Trace:** `4bf92f...` (phoenix) — [view](...)

User has no observability tool?
  → No env vars found
  → trace_ref is omitted
  → Markdown renders nothing for that field
```

This means:
- **Zero config for the common case** (no observability tool)
- **Auto-enrichment** when an observability tool is active alongside the agent
- **Explicit override** when users want control

#### Design rationale

- **Request** is always captured verbatim — this is the primary audit artifact ("who asked for what").
- **Response summary** is a concise description, not a full transcript. Think of it as a commit message for the conversation turn: _"Added JWT auth middleware in src/auth.go using RS256. Created tests. Added golang-jwt/jwt/v5 dependency."_
- **Files changed** ties the exchange to specific code artifacts, enabling file→exchange→session queries.
- **TraceRef** is the bridge to observability — follow the `trace_id` to see the full LLM calls, tool invocations, token usage, and latencies in your observability tool of choice.
- **Full transcripts are out of scope.** AgentLog intentionally sits at the "logical record" layer. The observability tool handles the "how" in full detail.

#### What about multi-turn follow-ups?

A single user request might span multiple back-and-forth turns with the agent (clarifications, retries). These collapse into **one exchange** — the original request plus the final summary of what was accomplished. The exchange represents the logical unit of work, not individual messages.

If the user makes a new, distinct request in the same session, that's a new exchange.

### CommitLink

Maps sessions to the commits they produced. A session can produce multiple commits; a commit can be linked to multiple sessions (e.g., merge commits).

| Field        | Type     | Description                           |
|--------------|----------|---------------------------------------|
| `session_id` | `string` | Session that produced the commit      |
| `commit_sha` | `string` | The resulting commit SHA              |
| `files_changed` | `[]string` | Files modified in this commit    |
| `linked_at`  | `RFC3339`| When the link was established         |

---

## Architecture

AgentLog has three layers. The top layer is how agents and users interact with it; the bottom two are the engine.

```
┌─────────────────────────────────────────────────────────────┐
│                   Interaction Layer                          │
│                                                             │
│  ┌──────────────────────┐  ┌─────────────────────────────┐ │
│  │   MCP Server          │  │   Standalone CLI            │ │
│  │   (agent sessions)    │  │   (scripting, CI, import)   │ │
│  │                       │  │                             │ │
│  │  Tools:               │  │  agentlog install             │ │
│  │   agentlog_start      │  │  agentlog start / log / end   │ │
│  │   agentlog_log        │  │  agentlog query / show        │ │
│  │   agentlog_end        │  │  agentlog import              │ │
│  │   agentlog_query      │  │  agentlog mcp-serve           │ │
│  │   agentlog_query      │  │                             │ │
│  │   agentlog_show       │  │                             │ │
│  │   agentlog_status     │  │                             │ │
│  └──────────┬───────────┘  └──────────────┬──────────────┘ │
│             └──────────────┬──────────────┘                 │
│                            │                                │
├────────────────────────────┼────────────────────────────────┤
│                      Core Library                           │
│                                                             │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────────┐      │
│  │  Session  │  │  Exchange    │  │   CommitLink     │      │
│  │  Manager  │  │  Recorder    │  │   Tracker        │      │
│  └─────┬────┘  └──────┬───────┘  └────────┬─────────┘      │
│        └───────────────┼───────────────────┘                │
│                        │                                    │
│                ┌───────▼────────┐                           │
│                │ StorageBackend │ (interface)                │
│                │   interface    │                            │
│                └───────┬────────┘                           │
│                        │                                    │
├────────────────────────┼────────────────────────────────────┤
│        Backend Plugins │                                    │
│                        │                                    │
│  ┌──────────┐  ┌──────┴──────┐  ┌──────────────────┐      │
│  │ Markdown │  │    JSON     │  │  SQLite/Postgres  │      │
│  │ (default)│  │  (built-in) │  │  ChromaDB / ...   │      │
│  └──────────┘  └─────────────┘  └──────────────────┘      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Layer responsibilities

| Layer | Purpose | Who uses it |
|-------|---------|-------------|
| **MCP Server** | Primary integration for agent coding sessions. Exposes AgentLog as MCP tools that any compatible agent calls natively. | All MCP-compatible coding agents |
| **Standalone CLI** | Scripting, CI pipelines, post-session import, manual queries. Also starts the MCP server (`agentlog mcp-serve`). | Developers, CI/CD, automation scripts |
| **Core Library** | The engine. Session/exchange/commit management, git integration, TraceRef discovery, backend fan-out. | MCP server and CLI both import this. |
| **Backend Plugins** | Pluggable storage. Markdown ships as default. | Configured by user, called by core. |

---

## Agent Integration

AgentLog integrates with coding agents through two open standards — no agent-specific code required:

1. **[MCP](https://modelcontextprotocol.io/)** (Model Context Protocol) — makes AgentLog's tools callable by any MCP-compatible agent
2. **[Agent Skills](https://agentskills.io/)** — tells agents _when_ and _how_ to call those tools

Together these provide a complete integration: MCP = "what tools exist", Agent Skills = "when to use them".

### How agents discover and call AgentLog

```
Coding Agent
  1. Discovers SKILL.md at startup (loads description for task routing)
  2. Activates skill when relevant task detected (loads full instructions)
  3. Calls MCP tools per the instructions (agentlog_start / agentlog_log / ...)
         |
         | MCP (stdio)
         v
AgentLog MCP Server
  agentlog_start   agentlog_log   agentlog_end
  agentlog_query   agentlog_show  agentlog_status
```

### AgentLog as an Agent Skill

[Agent Skills](https://agentskills.io) is an open standard for giving agents procedural knowledge. A skill is a directory with a `SKILL.md` containing metadata frontmatter + markdown instructions. Agents discover skills at startup, load the description for routing, and activate the full instructions on demand.

#### Skill structure

```
agentlog/
├── SKILL.md                    # Metadata + instructions
├── references/
│   └── EXCHANGE-FORMAT.md      # Detailed exchange field reference
└── scripts/
    └── check-session.sh        # Helper: verify MCP server is running
```

#### `SKILL.md`

```markdown
---
name: agentlog
description: >
  Audit trail for coding sessions. Use this skill whenever starting a coding
  session, completing a unit of work, or ending a session in a git repository.
  Also use when asked about the history of changes to files, commits, or
  functions — AgentLog can query past sessions to explain what was requested
  and what changed. Activates automatically at session boundaries and after
  completing user requests.
compatibility: Requires agentlog MCP server to be registered. Works with any MCP-compatible agent.
metadata:
  version: "0.1.0"
  repository: "https://github.com/drmaas/agentlog"
allowed-tools: agentlog_start agentlog_log agentlog_end agentlog_query agentlog_show agentlog_status
---

# AgentLog — Session Audit Trail

AgentLog maintains a causal link between user intent and code changes.

## When to activate

- **Session start**: At the beginning of any coding session in a git repository
- **After each unit of work**: When you complete a user request or distinct sub-task
- **Session end**: When the session is ending or the user says goodbye
- **History queries**: When asked "what changed?", "what led to this commit?",
  "why was this file modified?", or similar questions about codebase history

## Workflow

### 1. Start a session

Call `agentlog_start` at the beginning of a coding session.
This captures the current git HEAD, branch, and user identity.

### 2. Log each exchange

After completing a user request (or a distinct sub-task), call `agentlog_log`:

- `request`: the user's original prompt (verbatim — do not paraphrase)
- `response_summary`: a brief summary of what you did and why (1-3 sentences)
- `files_changed`: list of files you created, modified, or deleted

If the user's request spans multiple sub-tasks, log one exchange per logical
unit of work (e.g., "add auth middleware" and "add /me endpoint" are separate
exchanges even if requested together).

### 3. End the session

Call `agentlog_end` when:
- The user explicitly ends the session
- The user says goodbye, thanks you, or indicates they're done
- You detect the session is concluding

This finalizes the log, links commits, and writes the session file.

### 4. Answer history queries

When asked about the history of a file, function, commit, or branch:

1. Call `agentlog_query` with the relevant filter (SHA, file path, text, time range)
2. Summarize which sessions and exchanges are relevant
3. Call `agentlog_show` for details on specific sessions if needed

## Edge cases

- If `agentlog_status` shows no active session, call `agentlog_start` first
- If the MCP server is unavailable, inform the user and continue without logging
- Multi-turn follow-ups on the same request → one exchange (don't log partial work)
```

#### Why Agent Skills

| Concern | How Agent Skills addresses it |
|---------|-------------------------------|
| Portability | Open standard — any compliant agent discovers and uses it |
| Progressive disclosure | Description (~100 tokens) loaded at startup; full body loaded on activation |
| No per-agent code | One SKILL.md works everywhere |
| Discovery | Agents find skills in known locations (project dir, user dir, registries) |
| Versioning | Lives in the repo, version-controlled alongside the code |
| Rich instructions | Markdown body supports step-by-step workflows, examples, edge cases |
| Tool pre-approval | `allowed-tools` field declares which MCP tools the skill needs |

### `agentlog install` — zero-friction setup

```bash
# Install the Agent Skill + git hook into the current project
agentlog install

# Install globally (user-level skill)
agentlog install --global

# Only install the skill (skip git hook)
agentlog install --skill-only

# Only install the git hook (skip skill)
agentlog install --hook-only
```

**What `agentlog install` does:**

1. **Installs the Agent Skill** — writes `SKILL.md` and supporting files to `.agentlog/skill/` (project) or `~/.agentlog/skill/` (global)

2. **Prints MCP registration instructions** — outputs the JSON snippet to add to the agent's MCP config. Since MCP config locations vary by agent, AgentLog prints the snippet rather than writing to an agent-specific path:

   ```
   Add this to your agent's MCP configuration:

   {
     "mcpServers": {
       "agentlog": {
         "command": "agentlog",
         "args": ["mcp-serve"]
       }
     }
   }
   ```

3. **Installs the git hook** (`prepare-commit-msg`) for commit trailers

4. **Creates `.agentlog/config.yaml`** with defaults if it doesn't exist

5. **Adds `.agentlog/sessions/` to `.gitignore`** — session logs are local by default; the skill directory itself is committed so all contributors get it

### MCP Server

AgentLog runs as an MCP server (stdio transport). Any MCP-compatible agent connects to it and sees AgentLog's tools alongside its other tools (file editing, bash, etc.).

**MCP Tools exposed:**

| Tool | Description | When called |
|------|-------------|-------------|
| `agentlog_start` | Start a session. Auto-captures HEAD, branch, repo. Returns session_id. | At the beginning of a coding session |
| `agentlog_log` | Record an exchange (request + response summary + files changed). | After each logical unit of work |
| `agentlog_end` | End the session. Captures final HEAD, computes commit links, writes trailers. | When the session is done |
| `agentlog_query` | Query sessions by SHA, file, text, time range. | When the agent (or user) asks "what led to this?" |
| `agentlog_show` | Show full session detail. | When exploring a specific session |
| `agentlog_status` | Check if a session is active, get current session_id. | For the agent to know if it should be logging |

#### Tool descriptions as implicit instructions

MCP tool descriptions are written so agents understand _when_ to call each tool, even without the Agent Skill installed:

```go
{
    Name:        "agentlog_log",
    Description: "Record a completed exchange in the active AgentLog session. " +
        "Call this after completing a user request or distinct unit of work. " +
        "Provide the user's original request verbatim, a brief summary of " +
        "what you did, and the list of files changed.",
    InputSchema: { /* ... */ },
}
```

The Agent Skill provides richer context (workflow, edge cases, examples), but well-written tool descriptions are the baseline that works everywhere.

### Example session

```
User: Start tracking this session with agentlog.
Agent: [calls agentlog_start]
       ✓ AgentLog session started: sess-a1b2c3d4
         Branch: main | HEAD: abc1234 | User: daniel

User: Add JWT authentication to the API server. Use RS256 signing.
Agent: [... does the work, edits files, runs tests ...]
       [calls agentlog_log with request, summary, files]
       ✓ Exchange logged: request captured, 3 files changed.

User: Also add a /me endpoint.
Agent: [... does the work ...]
       [calls agentlog_log]
       ✓ Exchange logged: request captured, 2 files changed.

User: We're done, end the session.
Agent: [calls agentlog_end]
       ✓ AgentLog session ended: sess-a1b2c3d4
         2 exchanges recorded | 2 commits linked
         Log: .agentlog/sessions/2026-04-12T16-00-00Z_daniel_abc1234_sess-a1b2c3d4.md
```

With the Agent Skill installed, the agent calls `agentlog_start` automatically at the beginning of a session and `agentlog_log` after each request — no user intervention needed.


## Storage Backend Interface

```go
// StorageBackend defines the contract for all storage plugins.
type StorageBackend interface {
    // Session lifecycle
    WriteSession(ctx context.Context, session *Session) error
    UpdateSession(ctx context.Context, session *Session) error
    GetSession(ctx context.Context, sessionID string) (*Session, error)
    ListSessions(ctx context.Context, filter SessionFilter) ([]*Session, error)

    // Exchange recording
    WriteExchange(ctx context.Context, sessionID string, exchange *Exchange) error
    GetExchanges(ctx context.Context, sessionID string) ([]*Exchange, error)

    // Commit linking
    WriteCommitLink(ctx context.Context, link *CommitLink) error
    GetCommitLinks(ctx context.Context, sessionID string) ([]*CommitLink, error)

    // Query capabilities
    QueryByCommitSHA(ctx context.Context, sha string) ([]*Session, error)
    QueryByFilePath(ctx context.Context, path string) ([]*Session, error)
    QueryByTimeRange(ctx context.Context, start, end time.Time) ([]*Session, error)
    QueryByText(ctx context.Context, query string) ([]*Exchange, error) // full-text search

    // Lifecycle
    Init(ctx context.Context, config map[string]string) error
    Close() error
}
```

### SessionFilter

```go
type SessionFilter struct {
    UserID    string
    Branch    string
    Since     *time.Time
    Until     *time.Time
    Tags      []string
    Limit     int
    Offset    int
}
```

---

## Markdown Backend (Default)

### Directory Structure

```
.agentlog/
├── config.yaml                    # Backend config, user defaults
├── sessions/
│   ├── 2026-04-12T16-00-00Z_user123_abc123_sess-uuid.md
│   ├── 2026-04-12T18-30-00Z_user123_def456_sess-uuid.md
│   └── ...
└── index.md                       # Auto-generated session index
```

### Session File Format

Each session is a single Markdown file. The filename encodes the sort key:
`{started_at}_{user_id}_{git_sha_short}_{session_id}.md`

```markdown
# Session: sess-uuid-1234

| Field          | Value                                    |
|----------------|------------------------------------------|
| Session ID     | `sess-uuid-1234`                         |
| User           | `daniel`                                 |
| Started        | `2026-04-12T16:00:00Z`                   |
| Ended          | `2026-04-12T16:45:00Z`                   |
| Git SHA Before | `abc1234`                                |
| Git SHA After  | `def5678`                                |
| Repository     | `drmaas/agentlog`                        |
| Branch         | `main`                                   |
| Tags           | `feature`, `auth`                        |

## Commits

- `def5678` — `src/auth.go`, `src/auth_test.go`
- `ccc9999` — `README.md`

## Exchanges

### [0] — 2026-04-12T16:00:05Z

**Request:**
Add JWT authentication to the API server. Use RS256 signing.

**What changed:**
Added JWT auth middleware using RS256 with `golang-jwt/jwt/v5`. Created middleware in `src/auth.go` that validates tokens on protected routes. Added unit tests covering valid tokens, expired tokens, and malformed headers.

**Files:** `src/auth.go`, `src/auth_test.go`, `go.mod`
**Trace:** `4bf92f3577b34da6a3ce929d0e0e4736` (phoenix) — [view](http://localhost:6006/traces/4bf92f...)

### [1] — 2026-04-12T16:32:00Z

**Request:**
Also add a `/me` endpoint that returns the current user from the token.

**What changed:**
Added `GET /me` route in `src/routes.go` that extracts the user claim from the JWT and returns it as JSON. Reused the auth middleware from exchange [0].

**Files:** `src/routes.go`, `src/auth.go`
**Trace:** `8af43b2288c14da7b5de929d0e0e9812` (phoenix) — [view](http://localhost:6006/traces/8af43b...)
```

### Index File

Auto-generated for quick lookup:

```markdown
# AgentLog Session Index

| Started | User | Branch | Before SHA | After SHA | Session ID | File |
|---------|------|--------|------------|-----------|------------|------|
| 2026-04-12T16:00Z | daniel | main | abc1234 | def5678 | sess-uuid-1234 | [link](sessions/...) |
```

---

## Git Integration

### Auto-Detect (Session Start/End)

When `agentlog start` is run:
1. Capture `git rev-parse HEAD` → `git_sha_before`
2. Capture `git branch --show-current` → `branch`
3. Capture `git remote get-url origin` → derive `repository`

When `agentlog end` is run:
1. Capture `git rev-parse HEAD` → `git_sha_after`
2. Run `git log --format='%H' {before}..{after}` to find all commits in the session
3. For each commit, run `git diff-tree --no-commit-id --name-only -r {sha}` to get changed files
4. Write CommitLinks

### Commit Trailers

When commits are made during a session, embed a trailer:

```
feat: add JWT authentication

Implemented RS256-based JWT auth middleware.

AgentLog-Session: sess-uuid-1234
Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>
```

This enables reverse lookup: `git log --all --grep="AgentLog-Session: sess-uuid-1234"`.

**Implementation**: Provide a git hook (`prepare-commit-msg`) that auto-appends the trailer when an active session exists.

---

## Capture Mechanisms

### 1. MCP Server (primary — in-session, agent-driven)

The MCP server is the **primary capture path** for agent coding sessions. The agent calls AgentLog tools as part of its natural workflow. See [Agent Integration Layer](#agent-integration-layer) above for tool definitions and UX.

No manual intervention needed — the agent handles logging as part of doing its work.

### 2. Core Library API (for embedding in custom tools)

```go
import "github.com/drmaas/agentlog/pkg/agentlog"

session, _ := agentlog.StartSession(agentlog.SessionOpts{
    UserID: "daniel",
})
defer session.End()

session.LogExchange(agentlog.Exchange{
    Request:         "Add JWT auth to the API",
    ResponseSummary: "Added RS256 JWT middleware in src/auth.go with tests.",
    FilesChanged:    []string{"src/auth.go", "src/auth_test.go", "go.mod"},
})
```

### 3. Setup via `agentlog install`

```bash
# Install skill + git hook into the current project
agentlog install

# Install globally (user-level)
agentlog install --global
```

### 4. Standalone CLI (scripting, CI, manual use)

```bash
# Start a session
SESSION_ID=$(agentlog start --user daniel)

# Log an exchange (request + summary of what changed)
agentlog log --session $SESSION_ID \
  --request "Add JWT auth to the API" \
  --summary "Added RS256 JWT middleware in src/auth.go with tests." \
  --files src/auth.go,src/auth_test.go,go.mod

# End session
agentlog end --session $SESSION_ID
```

### 5. Post-Session Import (retroactive capture)

```bash
# Import from a generic JSON format
agentlog import json --file session-export.json

# Import from an agent's session store (agent-specific importers are pluggable)
agentlog import --format generic --file conversation.json
```

Importers are also pluggable — implement the `Importer` interface:

```go
type Importer interface {
    Import(ctx context.Context, source string, backend StorageBackend) error
}
```

---

## Query System

### CLI Queries

```bash
# What sessions touched this commit?
agentlog query --sha def5678

# What exchanges relate to this file?
agentlog query --file src/auth.go

# What happened on this branch this week?
agentlog query --branch main --since 2026-04-06

# Free-text search across exchanges
agentlog query --text "JWT authentication"

# Show full session detail
agentlog show sess-uuid-1234
```

### Programmatic Queries (for agent integration)

An agent can call the library to answer "what led to the state of function X?":

```go
// Find sessions that modified a file
sessions, _ := backend.QueryByFilePath("src/auth.go")

// Find which exchanges mention a concept
exchanges, _ := backend.QueryByText("JWT middleware")

// Trace a commit back to its session
sessions, _ := backend.QueryByCommitSHA("def5678")
```

For **semantic search** (ChromaDB, vector backends), `QueryByText` performs embedding-based similarity search rather than keyword matching.

---

## Plugin System

### Registration

Backends register via Go plugin interface or compile-time registration:

```go
func init() {
    agentlog.RegisterBackend("markdown", NewMarkdownBackend)
    agentlog.RegisterBackend("json", NewJSONBackend)
    agentlog.RegisterBackend("sqlite", NewSQLiteBackend)
}
```

### Configuration (`.agentlog/config.yaml`)

```yaml
version: 1
user_id: daniel
backends:
  - type: markdown     # Always active as default
    path: .agentlog/sessions
  - type: sqlite       # Optional secondary backend
    dsn: .agentlog/agentlog.db
  - type: chromadb     # Optional for semantic search
    url: http://localhost:8000
    collection: agentlog
```

Multiple backends can be active simultaneously (fan-out writes).

---

## Project Structure

```
agentlog/
├── cmd/
│   └── agentlog/
│       ├── main.go              # CLI entrypoint
│       ├── cli.go               # CLI command definitions
│       ├── install.go           # agentlog install (skill + MCP registration + fallback instructions)
│       └── mcp.go               # MCP server command (agentlog mcp-serve)
├── pkg/
│   └── agentlog/
│       ├── session.go           # Session manager
│       ├── exchange.go          # Exchange recorder
│       ├── commitlink.go        # Commit link tracker
│       ├── traceref.go          # TraceRef discovery (env sniffing, explicit)
│       ├── backend.go           # StorageBackend interface
│       ├── registry.go          # Backend registry
│       ├── config.go            # Configuration loading
│       └── git.go               # Git integration helpers
├── internal/
│   ├── mcp/
│   │   ├── server.go            # MCP server implementation (stdio transport)
│   │   └── tools.go             # MCP tool definitions (start, log, end, query, show, status)
│   ├── install/
│   │   ├── install.go           # Install orchestrator (write skill, git hook, config)
│   │   └── templates/           # Embedded templates (Go embed)
│   │       ├── skill.md         # SKILL.md template (the Agent Skill itself)
│   │       └── exchange-format.md  # references/EXCHANGE-FORMAT.md
│   ├── backends/
│   │   └── markdown/
│   │       ├── markdown.go      # Markdown backend implementation
│   │       ├── parser.go        # Markdown file parser (for queries)
│   │       ├── renderer.go      # Markdown file renderer
│   │       └── index.go         # Index file management
│   └── importers/
│       ├── importer.go          # Importer interface
│       └── json.go              # Generic JSON importer
├── hooks/
│   └── prepare-commit-msg       # Git hook for commit trailers
├── design.md                    # This file
├── go.mod
└── go.sum
```

---

## Future Considerations (Post-v1)

- **Agent Skill registries**: Publish the AgentLog skill to skill registries so users can discover and install it
- **Git Blame Integration**: `agentlog blame src/auth.go:42` → shows the exchange that led to that line
- **Diff Summarization**: Auto-summarize what changed per session using the LLM
- **Multi-Backend Sync**: Replicate from Markdown to a central database
- **Web UI / TUI**: Browse sessions, search exchanges, visualize commit→session graphs
- **CI Integration**: Auto-import agent sessions from CI pipelines
- **Session Replay**: Re-run a session's exchanges against a fresh agent to verify reproducibility
