---
description: Switch the active development mode. Use when the user runs /devmode or asks to change, set, pick, or view the development mode. Shows a quick-pick submenu of modes (og, tdd, vibe, poc, sdd, brainstorm, oneoff).
argument-hint: [set <og|tdd|vibe|poc|sdd|brainstorm|oneoff> | explain <og|tdd|vibe|poc|sdd|brainstorm|oneoff> | status | list]
allowed-tools:
  - Bash
  - ask_user
---

# Development Mode Switcher

The user invoked `/devmode $ARGUMENTS`.

Use the globally installed CLI:

```bash
devmode mode ...
```

## Instructions

### Step 1 - Parse arguments

If `$ARGUMENTS` is non-empty, handle directly:

- `set <mode>` -> run `devmode mode set <mode>` and confirm the switch. Done.
- `status` -> run `devmode mode status` and display the result. Done.
- `list` -> run `devmode mode list` and display the result. Done.
- `explain <mode>` -> run `devmode mode explain <mode>` and display the full mode flow and validation expectations. Done.

If `$ARGUMENTS` is empty, proceed to Step 2.

### Step 2 - Show the mode picker

1. Run `devmode mode --json status` to get the current mode.
2. Use the `ask_user` tool to present a single-select form:

```json
{
  "message": "Select development mode",
  "requestedSchema": {
    "properties": {
      "mode": {
        "type": "string",
        "title": "Development Mode",
        "description": "Choose the workflow for this session",
        "oneOf": [
          { "const": "og",   "title": "og - light plan -> implement -> verify -> review" },
          { "const": "tdd",  "title": "tdd - tests-first (red/green/refactor)" },
          { "const": "vibe", "title": "vibe - fast iteration with reduced ceremony" },
          { "const": "poc",  "title": "poc - exploratory spike, not production-ready" },
          { "const": "sdd",  "title": "sdd - spec-driven development" },
          { "const": "brainstorm", "title": "brainstorm - explore ideas without writing code" },
          { "const": "oneoff", "title": "oneoff - directly implement the user's request" }
        ],
        "default": "<current-mode-from-step-1>"
      }
    },
    "required": ["mode"]
  }
}
```

Set `default` to the current mode from `devmode mode --json status`. If no mode is set, omit `default`.

3. After the user selects, run `devmode mode set <selected-mode>`.
4. Confirm with a brief message: e.g., "Mode set to **sdd** (spec-driven development)."

### Step 3 - If user declines/cancels

Print the current mode and exit without making changes.

## Workflow After Mode Selection

Once a mode is set, the active coding harness automatically follows the selected workflow:

- `brainstorm` -> stay in discussion/ideation mode and do not write code
- `oneoff`, `og`, `tdd`, `vibe`, `poc`, `sdd` -> the harness routes implementation to `devmode-builder`
- when implementation is ready -> builder delegates to `devmode-reviewer`
- if reviewer requests changes -> builder iterates and resubmits

The user does not invoke builder or reviewer directly.
