# OpenTL - Open Tech Lead

An open source background coding agent for engineering teams. Send a task, get a PR.

```
opentl run "add rate limiting to /api/users" --repo myorg/myapp
# ...agent works in background...
# -> PR #142 opened: https://github.com/myorg/myapp/pull/142
```

## How It Works

1. You send a task via CLI (Slack and Web coming in Phase 2)
2. OpenTL spins up an isolated Docker sandbox with your repo
3. The [OpenCode](https://opencode.ai/) coding agent works on the task
4. Changes are committed, pushed, and a PR is opened
5. You review the PR

```
┌──────┐      ┌──────────────┐      ┌─────────────────┐      ┌────────┐
│ CLI  │─────▶│ OpenTL Server│─────▶│ Docker Sandbox   │─────▶│ GitHub │
└──────┘      └──────────────┘      │ (OpenCode Agent) │      └────────┘
                                    └─────────────────┘
```

## Quick Start

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/get-docker/)
- A GitHub personal access token (`GITHUB_TOKEN`)
- An LLM API key (`ANTHROPIC_API_KEY` or `OPENAI_API_KEY`)

### Install

```bash
go install github.com/jxucoder/opentl/cmd/opentl@latest
```

Or build from source:

```bash
git clone https://github.com/jxucoder/opentl.git
cd opentl
make build
```

### Setup

```bash
# Set required environment variables.
export GITHUB_TOKEN="ghp_..."
export ANTHROPIC_API_KEY="sk-ant-..."  # or OPENAI_API_KEY

# Build the sandbox Docker image.
make sandbox-image
```

### Run

```bash
# Start the server.
opentl serve

# In another terminal, run a task.
opentl run "fix the typo in README.md" --repo yourorg/yourrepo

# List sessions.
opentl list

# Check a session's status.
opentl status <session-id>

# Stream logs.
opentl logs <session-id> --follow
```

### Docker Compose

For a fully containerized setup:

```bash
# Set environment variables in a .env file or export them.
export GITHUB_TOKEN="ghp_..."
export ANTHROPIC_API_KEY="sk-ant-..."

# Start everything.
make docker-up

# Run tasks against the server.
opentl run "your task" --repo owner/repo --server http://localhost:7080
```

## Architecture

OpenTL has three components:

| Component | What it does |
|-----------|-------------|
| **Server** | Go HTTP server. Manages sessions, streams events, creates PRs. |
| **Sandbox** | Docker container per task. Clones repo, runs OpenCode agent, pushes branch. |
| **CLI** | Talks to the server. Creates sessions, streams logs. |

The server and CLI are the same binary (`opentl serve` vs `opentl run`).

### API

```
POST   /api/sessions            Create a session
GET    /api/sessions            List sessions
GET    /api/sessions/:id        Get session details
GET    /api/sessions/:id/events SSE stream of events
POST   /api/sessions/:id/stop   Stop a session
GET    /health                  Health check
```

## Configuration

All configuration is via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GITHUB_TOKEN` | Yes | - | GitHub personal access token |
| `ANTHROPIC_API_KEY` | One of these | - | Anthropic API key |
| `OPENAI_API_KEY` | One of these | - | OpenAI API key |
| `OPENTL_ADDR` | No | `:7080` | Server listen address |
| `OPENTL_DATA_DIR` | No | `~/.opentl` | Data directory for SQLite DB |
| `OPENTL_DOCKER_IMAGE` | No | `opentl-sandbox` | Sandbox Docker image |
| `OPENTL_DOCKER_NETWORK` | No | `opentl-net` | Docker network name |
| `OPENTL_SERVER` | No | `http://localhost:7080` | Server URL (for CLI) |

## Project Structure

```
OpenTL/
  cmd/opentl/          Main entry point (server + CLI in one binary)
  internal/
    config/            Configuration from environment
    server/            HTTP API, session orchestration, SSE streaming
    session/           Session model, SQLite store, event bus
    sandbox/           Docker container lifecycle
    github/            GitHub API (PR creation)
  docker/
    base.Dockerfile    Sandbox image (Ubuntu + Node + Python + Go + OpenCode)
    server.Dockerfile  Server image (minimal Alpine)
    compose.yml        Docker Compose for local dev
    entrypoint.sh      Sandbox entrypoint script
```

## Roadmap

### Phase 1: MVP (current)
- [x] Server with REST API and SSE streaming
- [x] Docker sandbox with OpenCode agent
- [x] CLI (run, list, status, logs)
- [x] GitHub PR creation

### Phase 2: Intelligence + Clients
- [ ] Plan-then-code-then-review prompt chain
- [ ] Web UI (React + Vite)
- [ ] Slack bot

### Phase 3: Scale
- [ ] Sandbox pre-warming and caching
- [ ] Modal/cloud sandbox provider
- [ ] Multiplayer sessions
- [ ] Kubernetes Helm chart

## License

Apache 2.0 - see [LICENSE](LICENSE).
