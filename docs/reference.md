# TeleCoder Reference

This page collects technical details that were intentionally kept out of the main `README.md`.

## Builder Usage

TeleCoder is designed as a **pluggable Go framework**. Import it as a library, swap any component via interfaces, and compose a custom application in ~15 lines.

### Minimal Usage

```go
package main

import (
    "context"
    telecoder "github.com/jxucoder/TeleCoder"
)

func main() {
    app, err := telecoder.NewBuilder().Build()
    if err != nil {
        panic(err)
    }
    app.Start(context.Background())
}
```

### Custom Usage

Swap out any component: store, sandbox, git provider, LLM, pipeline stages, channels.

```go
app, err := telecoder.NewBuilder().
    WithConfig(telecoder.Config{ServerAddr: ":8080", MaxRevisions: 2}).
    WithStore(myPostgresStore).
    WithGitProvider(myGitLabProvider).
    WithSandbox(myK8sRuntime).
    WithLLM(myLLMClient).
    WithChannel(myDiscordBot).
    Build()
```

## Core Interfaces

| Interface | Purpose | Built-in |
|:----------|:--------|:---------|
| `llm.Client` | LLM provider | Anthropic, OpenAI |
| `store.SessionStore` | Persistence | SQLite |
| `sandbox.Runtime` | Sandbox lifecycle | Docker, SSH (remote VPS) |
| `gitprovider.Provider` | Git hosting | GitHub |
| `eventbus.Bus` | Real-time event pub/sub | In-memory |
| `pipeline.Stage` | Orchestration stages | Plan, Review, Decompose, Verify |
| `channel.Channel` | Input/output transport | Slack, Telegram |

## Architecture

TeleCoder is a single binary: `telecoderserve` runs the server, and `telecoderrun` talks to it. Every component is swappable via interfaces.

| Component | Package | Description |
|:----------|:--------|:------------|
| **Builder** | `telecoder` | Entry point. Composes all components via `NewBuilder().Build()`. |
| **Engine** | `engine/` | Session orchestration: creates sessions, manages sandbox lifecycle, runs review/revision loops. |
| **Pipeline** | `pipeline/` | Plan -> code -> review pipeline with optional task decomposition and configurable stages. |
| **HTTP API** | `httpapi/` | Chi router. REST API + SSE streaming. Delegates logic to engine. |
| **Store** | `store/sqlite/` | SQLite persistence (WAL mode) for sessions, messages, and events. |
| **Sandbox** | `sandbox/docker/` | One Docker container per task (or persistent container for chat mode). |
| **Sandbox (SSH)** | `sandbox/ssh/` | Remote sandbox via SSH — run Docker on any VPS or cloud host. |
| **Sandbox Pool** | `sandbox/` | Pre-warming pool that wraps any Runtime for near-instant startup. |
| **Git Provider** | `gitprovider/github/` | GitHub API: PR creation, repo indexing, webhook handling. |
| **Event Bus** | `eventbus/` | In-memory pub/sub for real-time SSE events. |
| **Channels** | `channel/slack/`, `channel/telegram/` | Bot integrations: send tasks from chat, get PR links back. |
| **CLI** | `cmd/telecoder/` | Reference implementation. Creates sessions, streams logs, checks status. |
| **Web UI** | `web/` | React + Vite dashboard for monitoring sessions. |

## Multi-Agent Pipeline

TeleCoder supports assigning different coding agents to different pipeline stages. This lets you compose workflows like "OpenCode for research, Claude Code for coding, Codex for review."

### Agent Selection

Each stage can use a different agent:

| Stage | Config Field | Env Var | Behavior |
|:------|:-------------|:--------|:---------|
| **Research** | `ResearchAgent` | `TELECODER_RESEARCH_AGENT` | Spins up a sandbox agent to explore the codebase before planning. Falls back to `IndexRepo` file-tree scraping if unset. |
| **Code** | `CodeAgent` / `Agent` | `TELECODER_CODE_AGENT` / `TELECODER_AGENT` | The main coding agent. Defaults to `auto` (Anthropic key → OpenCode, OpenAI key → Codex). |
| **Review** | `ReviewAgent` | `TELECODER_REVIEW_AGENT` | Spins up a sandbox agent with the diff as context. Falls back to LLM-only review if unset. |

### Per-Session Override

The `POST /api/sessions` endpoint accepts an optional `agent` field that overrides the default coding agent for that session only:

```json
{"repo": "owner/repo", "prompt": "fix the bug", "agent": "claude-code"}
```

The CLI exposes this via the `--agent` / `-a` flag:

```bash
telecoder run "fix the bug" --repo myorg/myapp --agent claude-code
```

### Builder API

```go
app, _ := telecoder.NewBuilder().
    WithConfig(telecoder.Config{
        Agent:         "claude-code",
        ResearchAgent: &telecoder.AgentConfig{Name: "opencode"},
        CodeAgent:     &telecoder.AgentConfig{Name: "claude-code", Model: "claude-sonnet-4-20250514"},
        ReviewAgent:   &telecoder.AgentConfig{Name: "codex"},
    }).
    Build()
```

Each `AgentConfig` supports `Name`, `Image` (override Docker image), and `Model` (override LLM model).

### Available Agents

| Agent | Name | Requires | Description |
|:------|:-----|:---------|:------------|
| [OpenCode](https://opencode.ai/) | `opencode` | `ANTHROPIC_API_KEY` | AI coding agent (npm package) |
| [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) | `claude-code` | `ANTHROPIC_API_KEY` | Anthropic's CLI coding agent |
| [Codex CLI](https://openai.com/index/codex/) | `codex` | `OPENAI_API_KEY` | OpenAI's coding agent |

## API Endpoints

| Method | Endpoint | Description |
|:-------|:---------|:------------|
| `POST` | `/api/sessions` | Create a session (`mode=task` or `mode=chat`) |
| `GET` | `/api/sessions` | List sessions |
| `GET` | `/api/sessions/:id` | Get session details |
| `GET` | `/api/sessions/:id/events` | SSE stream of events |
| `GET` | `/api/sessions/:id/messages` | List chat messages |
| `POST` | `/api/sessions/:id/messages` | Send chat message |
| `POST` | `/api/sessions/:id/pr` | Create PR from chat session |
| `POST` | `/api/sessions/:id/stop` | Stop a session |
| `GET` | `/health` | Health check |

## Configuration

All configuration is via environment variables.

| Variable | Required | Default | Description |
|:---------|:---------|:--------|:------------|
| `GITHUB_TOKEN` | Yes | — | GitHub personal access token |
| `ANTHROPIC_API_KEY` | One of these | — | Anthropic API key |
| `OPENAI_API_KEY` | One of these | — | OpenAI API key |
| `TELECODER_ADDR` | No | `:7080` | Server listen address |
| `TELECODER_DATA_DIR` | No | `~/.telecoder` | Data directory for SQLite DB |
| `TELECODER_DOCKER_IMAGE` | No | `telecoder-sandbox` | Sandbox Docker image |
| `TELECODER_DOCKER_NETWORK` | No | `telecoder-net` | Docker network name |
| `TELECODER_MAX_REVISIONS` | No | `1` | Max review/revision rounds per sub-task |
| `TELECODER_CHAT_IDLE_TIMEOUT` | No | `30m` | Idle timeout for persistent chat sandboxes |
| `TELECODER_CHAT_MAX_MESSAGES` | No | `50` | Max user messages per chat session |
| `TELECODER_PLANNER_MODEL` | No | — | Override LLM model for pipeline stages |
| `TELECODER_AGENT` | No | `auto` | Default coding agent: `opencode`, `claude-code`, `codex`, or `auto` |
| `TELECODER_AGENT_MODEL` | No | — | Override the LLM model used by the in-sandbox coding agent |
| `TELECODER_RESEARCH_AGENT` | No | — | Agent for codebase research before planning (e.g. `opencode`) |
| `TELECODER_CODE_AGENT` | No | — | Override the coding-stage agent (e.g. `claude-code`) |
| `TELECODER_REVIEW_AGENT` | No | — | Agent for code review instead of LLM-only review (e.g. `codex`) |
| `TELECODER_SERVER` | No | `http://localhost:7080` | Server URL (for CLI) |

## Project Structure

```text
TeleCoder/
|-- telecoder.go              Builder, App, Config - top-level framework entry point
|-- defaults.go               Default wiring logic for Build()
|-- model/                    Foundation types (Session, Message, Event)
|-- llm/                      LLM Client interface
|   |-- anthropic/            Anthropic implementation
|   `-- openai/               OpenAI implementation
|-- store/                    SessionStore interface
|   `-- sqlite/               SQLite implementation
|-- sandbox/                  Runtime interface + pre-warming Pool
|   |-- docker/               Docker implementation
|   `-- ssh/                  SSH remote implementation
|-- gitprovider/              Provider interface
|   `-- github/               GitHub implementation (client, indexer, webhook)
|-- eventbus/                 Bus interface + InMemoryBus
|-- pipeline/                 Stage/Pipeline interfaces + built-in stages
|-- engine/                   Session orchestration logic
|-- httpapi/                  HTTP API handler (chi router, SSE)
|-- channel/                  Channel interface
|   |-- slack/                Slack bot (Socket Mode)
|   `-- telegram/             Telegram bot (long polling)
|-- cmd/telecoder/            Reference CLI implementation
|-- web/                      React + Vite dashboard
|-- docker/
|   |-- base.Dockerfile       Sandbox image (Ubuntu + Node + Python + Go + agents)
|   |-- server.Dockerfile     Server image (minimal Alpine)
|   |-- compose.yml           Docker Compose for local dev
|   |-- entrypoint.sh         Sandbox entrypoint script
|   `-- setup.sh              Sandbox setup script
|-- _examples/minimal/        Minimal framework usage example
`-- docs/                     Deployment and setup guides
```

## Roadmap

### Phase 1 - MVP

- [x] Server with REST API and SSE streaming
- [x] Docker sandbox with pluggable agents (OpenCode, Codex)
- [x] CLI (`run`, `list`, `status`, `logs`)
- [x] GitHub PR creation
- [x] Slack bot integration
- [x] Telegram bot integration
- [x] Web UI (React + Vite)

### Phase 2 - Intelligence

- [x] Plan-then-code-then-review prompt chain
- [x] Repo-aware context indexing
- [x] Multi-step task decomposition
- [x] Review-revision loop with configurable max rounds

### Phase 3 - Extensible Framework

- [x] Interface-based architecture with 7 pluggable components
- [x] Builder API for composing custom applications
- [x] Extracted engine, pipeline, HTTP API into independent packages
- [x] Framework importable as a Go library

### Phase 4 - Quality and Speed

- [x] Built-in test/lint pipeline stage — verify agent output before creating PR
- [x] Sandbox pre-warming pool for near-instant startup
- [x] Remote sandbox provider — run sandboxes on a VPS or cloud Docker host via SSH

### Phase 5 - Multi-Agent Pipeline

- [x] Per-stage agent selection (research, code, review)
- [x] Claude Code support in sandbox image
- [x] `TELECODER_AGENT` explicit agent selection in entrypoint
- [x] Agent-based research stage (replaces IndexRepo scraping)
- [x] Agent-based review stage (replaces LLM-only review)
- [x] Per-session agent override via API and CLI
