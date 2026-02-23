# TeleCoder v3 Design

> Your personal AI coding assistant. Any channel. Any repo. The lobster's coding cousin. 🦀

## What It Is

TeleCoder is an **OpenClaw-style personal AI assistant built for coding**. It lives
on your messaging channels (WhatsApp, Telegram, Slack, Discord), monitors your
repos, and does real coding work — proactively and on demand.

Unlike v2 (a task-runner that produces PRs), v3 is a **coding companion** that:
- Messages you when CI breaks, a PR needs review, or deps are outdated
- Fixes things autonomously when you tell it to
- Answers code questions in natural conversation
- Runs anywhere: your laptop, a VPS, or your team's server

```
You (Telegram): "hey, tests are failing on main — can you look?"
TeleCoder:      "Looking... it's a nil pointer in user_service.go:47.
                 The recent merge removed the null check. Want me to fix it?"
You:            "yeah go for it"
TeleCoder:      "Fixed and pushed → PR #203. Tests pass now."
```

```
TeleCoder (proactive, 9am Monday):
  "3 Dependabot alerts on myorg/api — 1 critical (lodash prototype pollution).
   Want me to update and run the test suite?"
```

---

## Architecture: Gateway → Agent Loop → Heartbeat

Inspired by OpenClaw's three-pillar architecture, adapted for coding:

```
┌─────────────────────────────────────────────────────────┐
│                       TeleCoder                          │
│                                                          │
│  ┌──────────┐    ┌──────────────┐    ┌──────────────┐   │
│  │ Gateway   │───▶│  Agent Loop  │◀───│  Heartbeat   │   │
│  │           │    │              │    │              │   │
│  │ WhatsApp  │    │ LLM brain    │    │ CI monitor   │   │
│  │ Telegram  │    │ Skill router │    │ PR watcher   │   │
│  │ Slack     │    │ Conversation │    │ Dep auditor  │   │
│  │ Discord   │    │ Memory       │    │ Repo health  │   │
│  │ GitHub    │    │ Tool use     │    │ Cron jobs    │   │
│  │ WebChat   │    │              │    │ Alert feeds  │   │
│  └──────────┘    └──────┬───────┘    └──────────────┘   │
│                         │                                │
│              ┌──────────▼───────────┐                    │
│              │  Skill Executor      │                    │
│              │                      │                    │
│              │  Sandbox (Docker)    │                    │
│              │  Git operations      │                    │
│              │  CI/CD APIs          │                    │
│              │  Code analysis       │                    │
│              │  File system         │                    │
│              └──────────────────────┘                    │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │  Store (SQLite)                                   │   │
│  │  Conversations · Memory · Repos · Heartbeat state │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

### Gateway

The Gateway handles all inbound/outbound messaging. Every channel is a
bidirectional transport — TeleCoder both receives and sends messages.

**Channels (messaging-first):**
- WhatsApp (via WhatsApp Business API or Baileys)
- Telegram
- Slack
- Discord
- GitHub (issues, PR comments, discussions)
- WebChat (built-in web UI)
- Linear, Jira (webhook → outbound via API)

**Key difference from v2:** Channels are the *primary* interface, not a bolt-on.
The Gateway maintains per-user conversation state, not per-session. You talk to
TeleCoder like a person, not like a job queue.

### Agent Loop

The brain. An LLM-powered agent that:
1. Understands your message in context (conversation history + repo knowledge)
2. Decides which Skills to invoke (or just responds conversationally)
3. Executes Skills and reports results
4. Maintains persistent memory across conversations

**Not a dispatcher** — the Agent Loop is a full reasoning agent, not a router.
It can plan multi-step workflows, ask clarifying questions, and hold context
across hours or days of conversation.

```go
type AgentLoop interface {
    // HandleMessage processes an inbound message and returns responses.
    // The agent decides what skills to invoke, if any.
    HandleMessage(ctx context.Context, conv *Conversation, msg *Message) ([]*Message, error)

    // HandleHeartbeatEvent processes a proactive event from the Heartbeat.
    // Returns messages to send to the user, or nil to stay silent.
    HandleHeartbeatEvent(ctx context.Context, event *HeartbeatEvent) ([]*Message, error)
}
```

**Model-agnostic:** Works with Claude, GPT, DeepSeek, Gemini, local models.
The LLM is a pluggable interface, not hardcoded.

### Heartbeat

The proactive engine. Runs on a schedule, monitors external systems, and
feeds events into the Agent Loop for decision-making.

**Built-in monitors:**
- **CI/CD** — Poll GitHub Actions, GitLab CI, Buildkite for failures
- **PR watcher** — New PRs, review requests, stale PRs, merge conflicts
- **Dependency audit** — Dependabot/Renovate alerts, outdated packages
- **Repo health** — Flaky tests, code coverage drops, TODO accumulation
- **Alert feeds** — Sentry, PagerDuty, Datadog (via webhooks)
- **Cron jobs** — User-defined scheduled tasks (same YAML as v2)

The Heartbeat doesn't act directly — it sends events to the Agent Loop, which
decides whether to message the user, act autonomously, or stay silent.

```go
type Monitor interface {
    Name() string
    // Check runs the monitor and returns events (if any).
    Check(ctx context.Context, repos []RepoConfig) ([]HeartbeatEvent, error)
    // Interval returns how often this monitor should run.
    Interval() time.Duration
}
```

---

## Skills (replacing monolithic coding agents)

v2 had 4 monolithic coding agents (Pi, Claude Code, OpenCode, Codex) that
did everything. v3 breaks capabilities into **focused Skills** that the
Agent Loop composes as needed.

### Core Skills (built-in)

| Skill | What it does |
|-------|-------------|
| `code.edit` | Edit files in a repo (direct LLM-powered edits) |
| `code.sandbox` | Run a coding agent in a Docker sandbox (Pi, Claude Code, etc.) |
| `code.search` | Search codebases (grep, AST, semantic) |
| `code.review` | Review a PR or diff for issues |
| `code.explain` | Explain code, architecture, or patterns |
| `git.commit` | Stage, commit, push changes |
| `git.pr` | Create, update, or merge pull requests |
| `git.branch` | Create, switch, delete branches |
| `git.diff` | Show diffs, compare branches |
| `test.run` | Run test suites, report results |
| `test.write` | Generate tests for given code |
| `lint.run` | Run linters, format code |
| `deps.audit` | Check for outdated/vulnerable dependencies |
| `deps.update` | Update dependencies and verify |
| `ci.status` | Check CI/CD pipeline status |
| `ci.logs` | Fetch and analyze CI failure logs |
| `shell.exec` | Run arbitrary shell commands in sandbox |
| `web.fetch` | Fetch and summarize web pages/docs |
| `repo.setup` | Clone, configure, and analyze a new repo |
| `memory.recall` | Search past conversations and sessions |

### Skill Interface

```go
type Skill interface {
    // Name returns the skill identifier (e.g., "code.edit").
    Name() string

    // Description returns a human-readable description for the LLM to decide when to use it.
    Description() string

    // Parameters returns the JSON schema for this skill's input.
    Parameters() json.RawMessage

    // Execute runs the skill and returns a result.
    Execute(ctx context.Context, params json.RawMessage) (*SkillResult, error)
}

type SkillResult struct {
    Output   string   // Text output to include in conversation
    Files    []string // Files created or modified
    Artifacts []Artifact // PRs, commits, etc.
    Error    string   // Error message if failed
}
```

### Community Skills

Like OpenClaw's AgentSkills, users can add custom skills:

```yaml
# ~/.telecoder/skills/deploy-staging.yaml
name: deploy.staging
description: "Deploy the current branch to the staging environment"
command: |
  cd /workspace/repo
  make deploy-staging
  echo "Deployed to https://staging.example.com"
requires: [sandbox]
```

Or as Go plugins:

```go
type DeploySkill struct{}
func (s *DeploySkill) Name() string { return "deploy.staging" }
// ...
```

The `code.sandbox` skill wraps the existing v2 coding agents (Pi, Claude Code,
OpenCode, Codex) as a single skill. For complex tasks, the Agent Loop invokes
`code.sandbox` which spins up a full Docker sandbox just like v2 — but now it's
one tool among many, not the only way to interact.

---

## Conversation Model (replacing Sessions)

v2's model: **Session** = one task, one sandbox, one result.

v3's model: **Conversation** = ongoing relationship with a user about their repos.

```go
type Conversation struct {
    ID        string
    UserID    string       // The human
    Channel   string       // Where this conversation lives
    Repos     []string     // Repos in context
    Messages  []Message    // Full history
    Memory    []MemoryRef  // Relevant past context (injected)
    State     ConvState    // active, paused, archived
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

A conversation can span days. The Agent Loop has context of what you discussed
yesterday. When you say "that PR from earlier", it knows which one.

**Sessions still exist** internally — when the Agent Loop invokes `code.sandbox`,
it creates a sandbox session. But users never see "sessions". They see a
conversation thread on Telegram.

---

## Deployment Modes

### Local (personal)

```bash
telecoder start
# → Connects to Telegram, starts Heartbeat, ready to chat
```

Runs on your laptop or a Raspberry Pi. One user. Docker for sandboxes.
Config in `~/.telecoder/config.yaml`.

### Server (team)

```bash
telecoder serve --multi-user
# → HTTP API + all channels, multi-user, shared repos
```

Runs on a VPS. Multiple users. Each user has their own conversations but
shares repo access. Admin dashboard.

### Config file

```yaml
# ~/.telecoder/config.yaml
llm:
  provider: anthropic          # or openai, deepseek, ollama, etc.
  model: claude-sonnet-4-6
  api_key: ${ANTHROPIC_API_KEY}

channels:
  telegram:
    bot_token: ${TELEGRAM_BOT_TOKEN}
  slack:
    bot_token: ${SLACK_BOT_TOKEN}
    app_token: ${SLACK_APP_TOKEN}
  # whatsapp:
  #   ...

github:
  token: ${GITHUB_TOKEN}

repos:
  - name: myorg/api
    branch: main
    monitors: [ci, deps, prs]
  - name: myorg/frontend
    branch: main
    monitors: [ci, prs]

heartbeat:
  enabled: true
  ci_poll_interval: 5m
  pr_poll_interval: 10m
  dep_audit_interval: 24h
  quiet_hours: "22:00-08:00"   # Don't message during these hours

sandbox:
  image: telecoder-sandbox
  docker_network: telecoder-net

memory:
  enabled: true
  embedder: openai              # or local (sentence-transformers)
```

---

## What We Keep from v2

| Component | Status |
|-----------|--------|
| `pkg/sandbox/` (Docker runtime) | **Keep** — core infrastructure |
| `pkg/store/sqlite/` | **Keep + extend** — add conversations, memory tables |
| `pkg/eventbus/` | **Keep** — still useful for real-time events |
| `pkg/gitprovider/` | **Keep** — GitHub PR/webhook integration |
| `pkg/model/` | **Refactor** — new types (Conversation, Skill, HeartbeatEvent) |
| `pkg/memory/` | **Keep + extend** — conversation-level memory |
| `pkg/agent/` | **Refactor** → becomes `code.sandbox` skill |
| `pkg/dispatcher/` | **Delete** — replaced by Agent Loop |
| `pkg/scheduler/` | **Refactor** → becomes part of Heartbeat |
| `pkg/channel/` | **Refactor** — bidirectional Gateway channels |
| `internal/engine/` | **Refactor** → Agent Loop + Skill Executor |
| `internal/httpapi/` | **Keep + extend** — add conversation endpoints |
| `cmd/telecoder/` | **Refactor** — `start` (local), `serve` (server) |
| `web/` | **Refactor** — conversation UI, not session list |
| `docker/entrypoint.sh` | **Keep** — sandbox still needs it |

---

## New Packages

```
pkg/gateway/          Bidirectional channel abstraction
pkg/gateway/telegram/  Telegram (send + receive)
pkg/gateway/slack/     Slack (send + receive)
pkg/gateway/whatsapp/  WhatsApp
pkg/gateway/discord/   Discord
pkg/gateway/webchat/   Built-in WebChat
pkg/agentloop/        LLM-powered reasoning agent
pkg/skill/            Skill interface + registry
pkg/skill/code/       code.edit, code.search, code.review, code.explain
pkg/skill/git/        git.commit, git.pr, git.branch, git.diff
pkg/skill/test/       test.run, test.write
pkg/skill/sandbox/    code.sandbox (wraps v2 coding agents)
pkg/skill/deps/       deps.audit, deps.update
pkg/skill/ci/         ci.status, ci.logs
pkg/skill/shell/      shell.exec
pkg/skill/web/        web.fetch
pkg/heartbeat/        Proactive monitoring engine
pkg/heartbeat/ci/     CI/CD monitor
pkg/heartbeat/pr/     PR watcher
pkg/heartbeat/deps/   Dependency auditor
pkg/heartbeat/cron/   Cron job runner (from v2 scheduler)
pkg/conversation/     Conversation state management
pkg/llm/              LLM provider interface (Anthropic, OpenAI, etc.)
```

---

## Implementation Plan

### Phase 1 — Core Agent Loop (replace engine with conversational agent)

1. **LLM provider interface** — `pkg/llm/` with Anthropic + OpenAI implementations
2. **Skill interface + registry** — `pkg/skill/` with core skill definitions
3. **Agent Loop** — `pkg/agentloop/` — LLM-powered reasoning with tool use
4. **Conversation model** — `pkg/conversation/` + store schema changes
5. **Port `code.sandbox` skill** — wrap existing v2 agent execution as a skill

### Phase 2 — Gateway (messaging-first)

6. **Gateway interface** — `pkg/gateway/` bidirectional channel abstraction
7. **Telegram gateway** — full send + receive with conversation threading
8. **Slack gateway** — Socket Mode, thread support
9. **WebChat gateway** — built-in web UI with conversation view
10. **GitHub gateway** — issues, PR comments, discussions as conversations

### Phase 3 — Heartbeat (proactive)

11. **Heartbeat engine** — `pkg/heartbeat/` monitor scheduler
12. **CI monitor** — poll GitHub Actions for failures
13. **PR watcher** — new PRs, review requests, stale PRs
14. **Dependency auditor** — security alerts, outdated packages
15. **Integration** — Heartbeat → Agent Loop → Gateway (proactive messages)

### Phase 4 — Skills Library

16. **code.edit** — direct LLM file editing (no sandbox needed for small changes)
17. **code.search + code.review + code.explain** — read-only code skills
18. **git.* skills** — full git workflow
19. **test.run + lint.run** — verification skills
20. **Community skill loader** — YAML-defined custom skills

### Phase 5 — Polish

21. **Local mode** — `telecoder start` for personal use
22. **Multi-user** — user management for server mode
23. **Quiet hours + notification preferences**
24. **WhatsApp + Discord gateways**
25. **Memory improvements** — conversation-aware retrieval

---

## Open Questions

1. **LLM tool-use format**: Use native function calling (Claude/OpenAI) or our own tool-use protocol?
2. **WhatsApp**: Baileys (unofficial, free) vs Business API (official, paid)?
3. **Conversation context window**: How much history to send per LLM call? Summarize old messages?
4. **Skill permissions**: Should some skills require user confirmation? (e.g., `git.pr`, `shell.exec`)
5. **Multi-repo conversations**: Can one conversation span multiple repos?
6. **Offline mode**: Should the Agent Loop work with local models (Ollama)?
