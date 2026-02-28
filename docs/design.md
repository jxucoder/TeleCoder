# TeleCoder Design

> Send a task. An agent does the work in a sandbox. You get the result.

---

## The Problem

Coding agents are powerful and unreliable.

Claude Code, Codex, OpenCode — any of them can implement a feature, fix a bug,
or answer a question about a codebase. But running them in production requires
solving the same problems every team solves independently:

- Where does the agent run? (not on your laptop)
- How do you prevent it from leaking secrets?
- How do you ensure it runs tests before shipping?
- How does it know about your codebase conventions?
- How do you trigger it from Slack, GitHub, or a cron job?
- How do you track what it did?

Every team rebuilds this infrastructure from scratch. TeleCoder is that
infrastructure, packaged as a small open-source framework.

---

## What TeleCoder Is

TeleCoder is an orchestration layer for coding agents. It receives a task
from anywhere (CLI, Slack, Telegram, GitHub, Linear, Jira, HTTP API), spins
up a sandboxed environment, runs whatever coding agent you choose, verifies
the output, and delivers the result — a pull request if code changed, a text
answer if not.

The agent does the thinking. TeleCoder handles everything around it.

---

## How It Works

A task arrives. This is the full lifecycle:

**1. Receive.** A channel (Slack, CLI, GitHub webhook, etc.) normalizes the
incoming message into a task: a repo and a prompt.

**2. Enrich.** The framework gathers context before the agent sees the task.
It retrieves relevant code snippets from the codebase memory index, loads
knowledge notes about the repo, and finds past session summaries that relate
to the current prompt. This context is prepended to the prompt so the agent
starts with understanding, not from zero.

**3. Sandbox.** A Docker container starts with the repo cloned and credentials
injected. The agent cannot touch anything outside the container. A pre-warm
pool keeps startup fast — containers are ready before tasks arrive.

**4. Execute.** The coding agent (Claude Code, OpenCode, Codex, or auto-
selected based on available API keys) runs inside the sandbox with the
enriched prompt. It reads code, edits files, runs commands — whatever it
decides is needed. TeleCoder streams its output as events in real time.

**5. Verify.** After the agent finishes, the framework auto-detects the
project's test and lint commands and runs them. If they fail, the failures
are fed back to the agent for another attempt (bounded to 1-2 rounds to
avoid wasting tokens). This happens automatically — no configuration needed.

**6. Deliver.** If code changed, the framework commits, pushes, and opens a
pull request. If no code changed (the task was a question), the text answer
is returned directly. The output format is determined by what happened, not
by what was configured upfront.

---

## Core Concepts

### Blueprints

A blueprint is a markdown file that tells the agent what kind of work to do
and how to approach it. It lives at **.telecoder/blueprint.md** in the repo.

A blueprint is English. Not code. Not YAML. A product manager can write one.
A security team can write one. An intern can write one.

The framework prepends the blueprint to the agent's prompt before execution.
If no blueprint exists, a sensible default is used. Different blueprints can
exist for different task types — one for code review, one for security audits,
one for data analysis.

The key insight: the blueprint describes intent, not steps. The coding agent
is sophisticated enough to plan and execute on its own. The blueprint shapes
what it focuses on.

### Guardrails

Guardrails are deterministic checks that the framework runs before and after
agent execution. They are not optional. They are not part of the blueprint.
They are the framework's responsibility.

**Before the agent runs:** context enrichment (inject codebase memory and
scoped rules), secret scanning on inputs.

**After the agent runs:** secret scanning on outputs, lint check, test run,
change size limits, scope check. If any post-guardrail fails, the failure is
fed back to the agent for a bounded retry.

The critical distinction: an agent can ignore an English instruction in a
blueprint. It cannot bypass a guardrail. Blueprints are flexible. Guardrails
are rigid. This is the hybrid orchestration pattern — agentic where flexibility
helps, deterministic where correctness matters.

Custom guardrails are defined in **.telecoder/guardrails.yaml** — each one is
a name and a shell command that returns pass or fail.

### Scoped Rules

Every codebase has conventions — documented in CLAUDE.md, AGENTS.md,
.cursorrules, .github/copilot-instructions.md, or similar files. TeleCoder
auto-discovers these during the pre-guardrail phase and injects them into the
agent's prompt alongside the blueprint.

What's good for human developers is good for agents. If your repo already has
agent rules, TeleCoder uses them automatically.

### Sandbox

Every task runs in an isolated Docker container. The agent cannot access the
host filesystem, network services it shouldn't reach, or other sessions. Docker
is the default runtime, with a pluggable interface for E2B, Modal, Fly, or any
container platform.

A pre-warm pool keeps containers ready before tasks arrive. The first session
should feel instant.

### Codebase Memory

TeleCoder maintains a semantic index of each repository it works with. Code is
chunked by language-aware parsers (AST-based for Go, regex-based for Python,
JavaScript, TypeScript, Rust, Java, Ruby), embedded into vectors, and stored
in SQLite with sqlite-vec for similarity search.

Three kinds of memory feed into prompt enrichment:

- **Code context** — relevant code snippets retrieved by hybrid search
  (keyword + vector similarity, merged with reciprocal rank fusion)
- **Knowledge notes** — durable key-value facts about a repo (architecture
  decisions, conventions, known issues) that persist across sessions
- **Session history** — summaries of past sessions for the same repo, so
  the agent doesn't repeat mistakes or miss context from prior work

### Multi-Repo

Tasks that span multiple repositories work without configuration. The sandbox
has git credentials injected. The agent clones whatever repos it needs. After
execution, the framework walks the filesystem, discovers all repos with changes,
runs guardrails per-repo using each repo's own configuration, and creates a PR
for each changed repo — cross-linked in their descriptions.

No repos field in the task request. The agent decides at runtime.

### Channels

Channels are thin adapters that receive tasks from external systems and deliver
results back. Each channel normalizes messages into tasks and routes results
to the appropriate destination.

| Channel | Trigger | Output |
|---------|---------|--------|
| **CLI** | Command line | Terminal output, PR link |
| **HTTP API** | REST endpoint | JSON response, SSE event stream |
| **Slack** | Message or mention | Thread reply with PR link or answer |
| **Telegram** | Bot message | Reply with PR link or answer |
| **GitHub** | Issue, PR comment, webhook | PR, issue comment |
| **Linear** | Issue label or assignment | Issue comment with result |
| **Jira** | Issue transition or label | Issue comment with result |

### Dispatch

An LLM-powered dispatcher examines incoming events and decides the appropriate
action: create a task session, ignore the message, or route to a specific agent.
This replaces keyword matching with intelligent classification.

### Agent Chains

After a session completes, the dispatcher can evaluate the result and spawn a
follow-up session — for example, running a security audit after a feature PR is
created. Chains have a depth limit (default 3) to prevent loops, and a chain ID
links related sessions.

### Scheduling

Recurring tasks are defined in **.telecoder/jobs/** as YAML files with cron
expressions. The scheduler triggers sessions automatically — useful for nightly
security scans, weekly dependency updates, or daily test runs.

---

## What Exists Today

The v2 architecture is complete and working. Here is what the codebase contains:

| Component | What it does |
|-----------|-------------|
| **Docker sandbox** | Container lifecycle, persistent containers for chat mode, pre-warm pool |
| **Engine** | Session management (task mode and chat mode), event streaming, prompt enrichment, multi-step execution with checkpoints, verify-and-retry loop, PR comment sessions, idle session reaper |
| **Coding agents** | Four implementations — OpenCode, Claude Code, Codex, Pi — each providing command generation and output parsing behind a common interface |
| **Dispatcher** | LLM-powered event routing that classifies incoming messages and decides actions |
| **Agent chains** | Follow-up session spawning with depth limiting and chain linking |
| **Codebase memory** | Code indexing with language-aware chunking, vector + keyword hybrid search, knowledge notes, session summaries |
| **Verify** | Auto-detection of test and lint commands based on project files (go.mod, package.json, Cargo.toml, etc.) |
| **Channels** | Slack, Telegram, GitHub webhooks, Linear, Jira |
| **Scheduler** | Cron-based recurring task execution |
| **HTTP API** | REST endpoints for session management, SSE for real-time event streaming |
| **Web dashboard** | TypeScript frontend for monitoring sessions |
| **CLI** | Commands for serve, run, list, status, logs, config |
| **Builder pattern** | Pluggable composition — swap any component (store, sandbox, git provider, bus, channels) |
| **SQLite store** | Session persistence, event storage, message history |
| **Event bus** | In-memory pub/sub for real-time updates |

The core is approximately 6800 lines of Go. Total with channels, tests, CLI,
and web UI is larger, but the framework a contributor needs to understand is
small enough to read in a single sitting.

---

## What to Build Next

### Blueprints and Guardrails

The current verify-and-retry loop is hardcoded in the engine. Extract it into
an explicit guardrails framework. Add blueprint loading from
.telecoder/blueprint.md. Add custom guardrails from .telecoder/guardrails.yaml.
Add scoped rules discovery (auto-find CLAUDE.md, .cursorrules, etc.).

This is the differentiator. It makes TeleCoder's value proposition visible
and configurable rather than hidden in engine internals.

### Memory Security

Code memory stores proprietary source code. Before companies trust it, the
system needs: a secret scanner that detects API keys and tokens before indexing,
content sanitization when injecting context into prompts, repo-scoped isolation
to prevent cross-repo data leaks, and audit events on every memory operation.

### Developer Experience

A one-command quickstart that works. A tutorial that gets someone from zero to
a working task in five minutes. Example blueprints for common workflows. A
README that tells the blueprint story front and center.

### Ecosystem

MCP server to expose memory as tools for external editors. MCP client to consume
external memory providers. Ollama embedder for zero-API-key local operation.
Webhook-driven reindexing so the code index stays fresh.

---

## What Not to Build Yet

The v3 design proposed a full conversational agent with heartbeat monitoring,
a skill system, and bidirectional gateway channels. That's the right long-term
direction, but it's too much surface area for adoption.

Projects get popular by doing one thing brilliantly, then expanding. Docker
started with containers. FastAPI started with a web framework. Next.js started
with server-side rendering. Each expanded after becoming the default.

TeleCoder's one thing: send a task, agent works in a sandbox, guardrails enforce
quality, you get the result. Once that's the obvious default for running coding
agents in production, expand to conversations, heartbeat, and proactive
monitoring.

---

## Design Principles

**Small auditable core.** The entire engine fits in a few thousand lines of Go.
Any developer — or AI agent — can read and understand the whole thing. This is
the meta-insight: the framework is maximally forkable because it's small enough
for AI to operate on.

**English is configuration.** Blueprints are markdown files. Guardrails config
is a short YAML file. Scoped rules are the same files human developers already
write for their editors. No DSL. No custom language. No learning curve.

**Sandboxes by default.** Every task runs isolated. This is non-negotiable. The
default is Docker. The interface supports any container platform.

**Separate agentic from deterministic.** Blueprints (agentic, flexible) describe
what the agent should do. Guardrails (deterministic, rigid) enforce what must be
true regardless. The agent handles ambiguity. The framework handles correctness.

**Shift feedback left.** Run tests and lint locally in the sandbox before
creating a PR. Don't waste CI cycles on failures the framework can catch.

**Bounded retries.** One or two revision rounds, no more. If the agent can't
pass tests in two tries, the problem is the prompt or the blueprint, not
insufficient attempts.

**Agent-agnostic.** Support every coding agent. Don't pick winners. Let the
user choose. TeleCoder orchestrates whichever agent they prefer.

**Progressive disclosure.** Zero config works (sensible defaults, auto-detected
agent, auto-detected test commands). Custom blueprints for teams that want
control. Provider interfaces for companies that need to swap internals. Each
level is opt-in.

**Forkability over extensibility.** A developer who forks TeleCoder and modifies
the source should have an easier time than one who writes plugins. Keep the core
small enough to modify directly.

**Pre-warm everything.** Sandbox pool, code index, rules cache. The first task
should feel instant.

---

## Comparison

| | TeleCoder | Claude Code | Codex CLI | SWE-agent |
|---|-----------|-------------|-----------|-----------|
| Async (fire-and-forget) | Yes | No | Yes | No |
| Blueprints + guardrails | Yes | No | No | No |
| Agent-agnostic | 4 agents | Claude only | OpenAI only | Any LLM |
| Sandbox by default | Pluggable | Optional | Yes | Docker |
| Multi-repo | Yes | No | No | No |
| Codebase memory | Yes | Yes | No | No |
| Multi-channel | 5 channels | No | No | No |
| Verify + retry | Yes | No | No | Yes |
| Scoped rules auto-discovery | Yes | CLAUDE.md | No | No |

TeleCoder is the orchestration layer that makes any coding agent work reliably
in production.
