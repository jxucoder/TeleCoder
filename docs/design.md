# TeleCoder — Design for Maximum Adoption

> The open framework for async coding agents.
> Any model. Any codebase. Any scale.

---

## Thesis

TeleCoder should be to **async coding agents** what Docker is to containers
or FastAPI is to Python APIs: the obvious default that everyone reaches for.

The insight from Stripe Minions: the coding agent (Claude Code, Codex, etc.)
is ~20% of the value. The other 80% is the **orchestration around it**:

- Pre-computing relevant context (rules, code, past sessions)
- English blueprints that describe what the agent should do
- Deterministic guardrails that enforce quality and security (non-negotiable)
- Sandbox isolation with pre-warming
- Smart output routing (PR, text reply, report, PR comments — whatever fits)
- Multi-repo: agent clones what it needs, framework guardrails each repo

TeleCoder owns that 80%. It doesn't replace Claude Code — it makes Claude Code
(and Codex, and OpenCode, and Aider, and your custom agent) work reliably in
production.

### What makes projects maximally popular

1. **Clear identity** — one sentence explains what it is
2. **Instant gratification** — working in 30 seconds
3. **Small core** — fits in your head (and in an AI's context window)
4. **Low floor, high ceiling** — easy to start, scales to Stripe
5. **Extensible without complexity** — plugins, not config sprawl

TeleCoder's identity: **"Send a task in English. An agent does the work in a
sandbox. Guardrails enforce quality. You get the result — PR, text answer,
report, whatever the task requires."**

---

## Architecture

Four components, each independently useful:

**Channels** receive tasks from the outside world — CLI, HTTP API, Slack,
Telegram, GitHub, Linear, Jira. Each channel is a thin adapter that normalizes
incoming messages into a task.

**Engine** manages sessions, events, memory, and dispatch. It decides which
agent handles the task, tracks state, and coordinates the lifecycle.

**Blueprints and Guardrails** define what the agent should do (English) and
what the framework enforces (deterministic checks). The blueprint instructs;
the guardrails constrain.

**Sandbox** provides isolated execution — Docker by default, with pluggable
support for E2B, Modal, Fly, or any container platform. A pre-warm pool keeps
startup fast.

Underneath everything sits a SQLite store for persistence, a codebase memory
system (code index + notes + past sessions), and an event bus for real-time
streaming via SSE.

Total core: **~4500 lines of Go**. The rest is channels, tests, CLI, and web UI.

---

## The Two Core Ideas

### 1. Blueprints — English-First Workflow Descriptions

A blueprint is a **natural language description** of what the agent should do.
Not Go. Not YAML. Not shell scripts. **English.**

A typical blueprint lives at **.telecoder/blueprint.md** in the repo and reads
like this: "You are a coding agent working on this repository. When given a
task, understand the codebase context and coding conventions, implement the
requested changes, make sure all tests pass, fix failures if needed (up to 2
attempts), and write a clear PR description explaining what changed and why.
If the task is a question, just answer it directly."

That's it. Drop this file in your repo and TeleCoder uses it to instruct the
agent. Anyone can write a blueprint. Anyone can customize the workflow. No
programming language required.

The framework prepends the blueprint + gathered context to the agent's prompt,
then dispatches to whatever agent is configured (Claude Code, Codex,
OpenCode, etc.). The agent handles the multi-step execution internally — these
are powerful coding agents, they know how to plan and execute.

**Why English?** Because English is the most universal "programming language."
A PM can write a blueprint. A security team can write a blueprint. An intern
can write a blueprint. English blueprints scale to any team, any skill level.

**Different blueprints for different tasks.** You can have separate blueprints
for different workflows:

- **review.md** — instructs the agent to act as a code reviewer, analyze PR
  diffs, list issues by severity, and respect the project's linter config
- **security-audit.md** — instructs the agent to scan for vulnerabilities,
  run security tools if available, and produce a severity-grouped report
- **data-analysis.md** — instructs the agent to run analysis scripts and
  produce a summary with key findings and recommended actions

The agent does the agentic work. The blueprint tells it what to focus on.

### 2. Guardrails — Deterministic, Non-Negotiable, Always-On

Here's what Stripe learned: you don't ask the agent to run tests. You don't
ask the agent to scan for secrets. You don't write a blueprint step for it.
**The framework does it automatically, every time, non-negotiably.**

Guardrails are **deterministic checks** that run before and after agent
execution. They are not part of the blueprint. They are not opt-in. They are
the framework's job.

The flow is: **Blueprint** feeds into **Pre-Guardrails** (context enrichment,
rules injection, secret scan on inputs), then the **Agent** works in a sandbox,
then **Post-Guardrails** run (secret scan on output, lint, tests, size limits,
scope check). If post-guardrails fail, failures are fed back to the agent for
a bounded number of retries. Once everything passes, the **Output** is
delivered — PR, text reply, comments, report — whatever fits.

**Built-in guardrails:**

| Guardrail | When | What |
|-----------|------|------|
| **Context enrichment** | Pre | Inject codebase memory, scoped rules, past sessions |
| **Secret scan** | Pre + Post | Detect API keys, tokens, passwords in inputs and outputs |
| **Lint check** | Post | Auto-detect linter, run it, feed failures back to agent |
| **Test run** | Post | Auto-detect test framework, run it, feed failures back |
| **Size limit** | Post | Reject changes that touch too many files (configurable) |
| **Scope check** | Post | Warn if agent modified files outside expected scope |
| **Retry budget** | Post | Bounded retries (default 1-2) — don't burn tokens |

**Custom guardrails** are any executable that returns pass/fail, defined in
**.telecoder/guardrails.yaml**. Examples: run the TypeScript compiler with
no-emit to type-check, run semgrep for security scanning, or grep for banned
patterns like console.log in production code. Each custom guardrail has a name
and a shell command — nothing more.

**Why this split matters:**

- The **blueprint** (English) is for the team — it describes what you want.
- The **guardrails** (deterministic) are for the framework — they enforce
  quality and security regardless of what the blueprint says.
- An agent can ignore an English instruction. It **cannot** bypass a guardrail.

This is the Stripe lesson distilled: hybrid orchestration where the agentic
parts are flexible (English) and the deterministic parts are rigid (guardrails).

### How They Work Together

Consider the task "add rate limiting to /api/users":

1. The **blueprint** is loaded — it says to implement changes, ensure tests
   pass, and write a clear PR description.
2. **Pre-guardrails** run — gather related code context, load CLAUDE.md rules
   and notes, secret-scan the inputs. Everything is clean. This context is
   prepended to the agent's prompt.
3. The **agent** executes in a sandbox with the enriched prompt. Claude Code
   edits files, writes tests, does its thing.
4. **Post-guardrails** run — secret scan the output (clean), run the linter
   (passes), run the tests (2 failures detected). Failures are fed back to
   the agent. The agent fixes the issues. Tests pass on round 2. Size check:
   4 files changed, within limits.
5. **Output** — files changed, so the framework commits, pushes, and creates
   a pull request.

### Programmatic Escape Hatch

For companies building products on TeleCoder that need precise orchestration
control, blueprints can also be Go functions. This gives full control over
multi-agent orchestration, custom verification logic, and external API calls.
But it's the power-user path, not the default.

Even programmatic blueprints get guardrails applied automatically. You can't
accidentally skip the secret scan.

### The Run Object — Layered Primitives

The Run has two layers of capabilities:

**Generic primitives** work for any agent task — sandbox operations (execute
commands, read files, write files), agent invocation (send a prompt, get a
response, optionally with a specific model), context gathering (enrich the
prompt with memory and rules), output (reply with text, finalize with smart
defaults, emit SSE events), and state queries (check if files changed).

**Coding helpers** are convenience methods built entirely on the generic
primitives — verify (auto-detect and run lint + test commands), lint fix, test
only, git push, amend push, create PR, comment on PR, wait for CI. These are
why TeleCoder is great for coding agents specifically, but they're just
wrappers around the generic layer.

A framework builder who doesn't care about PRs or linting never touches the
coding helpers. They use the generic primitives and build whatever workflow
they need.

---

## Multi-Repo Tasks

Real-world tasks often span multiple repositories. Update an API and its
client. Change a shared schema and all its consumers. Upgrade a library
across dependent repos.

### The wrong approach: pre-define repos upfront

Listing repos in the task request is limiting. You're deciding upfront which
repos the agent needs. But the agent might discover mid-task that it needs
another repo. Or realize the client library is fine and only the server needs
changes. Pre-defining repos is the same mistake as pre-defining steps — it
constrains the agent to what you anticipated.

### The right approach: give the agent git access, let it work

The sandbox has git credentials. The agent can clone any repo it has access to.
It decides at runtime what it needs. The framework **watches and reacts**.

For a task like "add rate limiting to the API and update the Python client":

1. The agent clones the API server repo and the Python client repo
2. It edits both (and maybe clones a third repo just to read how another
   service handles rate limiting)
3. The framework scans the sandbox afterward — discovers 2 repos with changes,
   notes the third was cloned but unchanged (skip it)
4. Post-guardrails run **per repo** — each repo gets its own secret scan, lint,
   and test run, using that repo's own configuration
5. Output: one PR per changed repo, cross-linked in their descriptions

### What the framework does (not the user)

1. **Before**: Inject git credentials into the sandbox so the agent can clone
2. **After**: Walk the filesystem, find all git repos with uncommitted changes
3. **Per changed repo**: Load its .telecoder/ config, discover its lint/test
   commands, run guardrails
4. **Output**: Create a PR for each changed repo, cross-link them

The user's task request is just a prompt — "Add rate limiting to the API and
update the Python client." No repos field. No upfront configuration. The agent
figures out which repos it needs, the framework handles everything after.

### Why this is better

- Agent can discover dependencies dynamically (clone a repo to read it, not change it)
- Agent can decide mid-task that a repo doesn't need changes
- Agent can clone repos you didn't anticipate
- Zero configuration for multi-repo — it just works
- Single-repo tasks work identically (one repo detected, one PR created)

---

## Scoped Rules (Stripe Pattern)

Stripe discovered that **what's good for human developers is good for agents**.
Every codebase has conventions documented in .cursorrules, CLAUDE.md, AGENTS.md,
.github/copilot-instructions.md, and similar files. TeleCoder auto-discovers
and injects these.

During the pre-guardrail phase, the framework searches each repo for rules
files — CLAUDE.md, AGENTS.md, .cursorrules, .github/copilot-instructions.md,
.telecoder/rules.md, and any files under .telecoder/rules/. It finds the most
relevant rules for the current task and prepends them to the agent's prompt
alongside the blueprint.

This is zero-config for repos that already have agent rules — which is most
active repos in 2026.

---

## What TeleCoder Already Has (v2, Complete)

| Component | Status | Lines |
|-----------|--------|-------|
| Docker sandbox with pre-warm pool | Done | ~500 |
| Engine (sessions, events, memory) | Done | ~1250 |
| 4 coding agents (OpenCode, Claude Code, Codex, Pi) | Done | ~200 |
| LLM-powered dispatcher | Done | ~140 |
| Agent chains (depth-limited) | Done | ~70 |
| Codebase memory (code index + notes + sessions) | Done | ~1000 |
| Verify (auto-detect test/lint commands) | Done | ~40 |
| 5 channels (Slack, Telegram, GitHub, Linear, Jira) | Done | ~1400 |
| Cron scheduler | Done | ~130 |
| HTTP API + SSE | Done | ~375 |
| Web dashboard | Done | ~400 (TS) |
| CLI (serve, run, list, status, config) | Done | ~650 |
| Builder pattern (plug anything) | Done | ~200 |
| SQLite store | Done | ~375 |
| Event bus | Done | ~70 |
| **Total** | | **~6800** |

The foundation is solid. What's needed for maximum adoption is **not more
features** — it's refinement, positioning, and the blueprint pattern.

---

## What to Build Next

### Phase 1: Blueprints + Guardrails (the differentiator)

**Goal**: English-first blueprints with deterministic guardrails. Extract the
current hardcoded task flow into the guardrails framework. Add blueprint
loading from .telecoder/blueprint.md.

1. Define the Blueprint type — English loader plus Go function escape hatch — and the Run struct
2. Blueprint discovery — load .telecoder/blueprint.md and prepend to prompt
3. Guardrails framework — pre-guard, post-guard, retry loop
4. Built-in guardrails — secret scan, lint, test, size limit
5. Custom guardrails from .telecoder/guardrails.yaml
6. Wire into engine via the Builder pattern
7. Scoped rules discovery in pre-guard (CLAUDE.md, .cursorrules, etc.)

**Eval**: All tests pass. Default behavior identical but now with guardrails
enforced. English blueprints work via .telecoder/blueprint.md.

### Phase 2: Memory Security (trust)

Companies won't use a framework that leaks secrets into agent prompts.

1. Secret scanner — regex patterns for AWS keys, GitHub tokens, JWTs, etc.
2. Integrate scanner into the chunk insertion pipeline
3. Skip sensitive files (.env, .pem, etc.) during indexing
4. Content sanitization when enriching prompts
5. Audit events on memory operations

**Eval**: No .env or .pem files indexed. No API keys in retrieved chunks.

### Phase 3: Developer Experience (adoption)

1. One-command quickstart via docker compose
2. "Build your first async agent in 5 minutes" tutorial
3. Example blueprints gallery (security, CI-aware, multi-agent)
4. README rewrite focused on the blueprint story
5. GitHub Actions template for "TeleCoder as CI bot"

### Phase 4: Ecosystem (network effects)

1. MCP server — expose memory as MCP tools
2. MCP client — consume external tools as memory providers
3. Ollama embedder — zero-API-key local memory
4. Webhook-driven reindexing (reindex on git push)
5. Provider interfaces for memory (Qdrant, Mem0, etc.)

---

## What NOT to Build (Yet)

The v3 design proposed a full conversational agent (Agent Loop, Heartbeat,
Skills, Gateway). That's the right long-term vision, but it's **too much
surface area for adoption**.

Projects get popular by doing **one thing brilliantly**, then expanding:
- Docker started as containers, then added Compose, Swarm, Hub
- FastAPI started as a framework, then added background tasks, WebSockets
- Next.js started as SSR React, then added API routes, middleware

TeleCoder's "one thing": **send a task in English, agent works freely in a
sandbox, guardrails enforce quality, you get the result**. Once that's the
default, expand to conversations, heartbeat, and proactive monitoring.

### Defer to Phase 2 (after adoption)

- Full conversational agent (Agent Loop replaces Engine)
- Heartbeat (proactive CI/PR/dep monitoring)
- Bidirectional gateway channels
- Skill system (code.edit, code.review, etc.)
- WhatsApp/Discord channels
- Multi-user/team mode

These are valuable but not necessary for the core value proposition.

---

## Design Principles (Lessons Learned)

### From NanoClaw

1. **Small auditable core** — The entire engine + blueprint + sandbox fits in
   ~4500 lines. Any AI agent can read, understand, and modify the whole thing.
   This is the meta-insight: the framework is maximally forkable because it's
   small enough for AI to operate on.

2. **English IS configuration** — Blueprints are markdown files anyone can
   write. Guardrails are the framework's job, not the user's. Go functions
   exist as an escape hatch for power users, not the default path.

3. **Sandboxes by default** — Every task runs in an isolated container. Docker
   is the default runtime, but the sandbox interface supports E2B, Modal, Fly,
   Firecracker, or any container platform.

### From Stripe Minions

4. **Separate agentic from deterministic** — English blueprints dispatch
   agents. Deterministic guardrails enforce quality and security. The agent
   can't bypass the guardrails. This is the hybrid orchestration pattern.

5. **Shift feedback left** — Lint locally before pushing. Test locally before
   creating the PR. Don't waste CI cycles on obvious failures.

6. **Bounded retries** — One or two revision rounds, no more. Additional
   retries don't help — they waste tokens. Fix the prompt or the blueprint
   instead.

7. **Scoped rules** — What's good for human developers is good for agents.
   Discover and inject CLAUDE.md, .cursorrules, etc. automatically.

8. **Pre-warm everything** — Sandbox pool, code index, rules cache. The first
   session should feel instant.

### For Maximum Adoption

9. **30-second quickstart** — One command to start, one command to send a task.
   No setup wizard, no account creation, no cloud dependency. Just works.

10. **Agent-agnostic** — Support every coding agent. Don't pick winners. The
    user chooses. TeleCoder orchestrates.

11. **Progressive disclosure** — Default blueprint works with zero config.
    Custom blueprints for power users. Provider interfaces for companies.
    Each level is opt-in.

12. **Forkability over extensibility** — A developer who forks TeleCoder and
    modifies the source should have an easier time than one who writes plugins.
    Keep the core small enough to modify directly.

---

## Comparison

| Feature | TeleCoder | Claude Code | Codex CLI | SWE-agent |
|---------|-----------|-------------|-----------|-----------|
| Async (fire-and-forget) | Yes | No | Yes | No |
| English blueprints + guardrails | Yes | No | No | No |
| Agent-agnostic | Yes (4 agents) | Claude only | OpenAI only | Any LLM |
| Sandbox by default | Yes (pluggable) | Optional | Yes | Docker |
| Multi-repo tasks | Yes | No | No | No |
| Pre-warm pool | Yes | No | No | No |
| Codebase memory | Yes | Yes | No | No |
| Multi-channel (Slack, etc.) | Yes | No | No | No |
| Verify + retry | Yes | No | No | Yes |
| Scoped rules | Yes | CLAUDE.md | No | No |
| Lines of core code | ~4500 | ~50K+ | ~10K+ | ~5K |

TeleCoder's unique position: it's the **orchestration layer** that makes any
of these agents work better — and it doesn't force every task into a PR-shaped
box.

---

## Success Metrics

TeleCoder is maximally popular when:

1. **"telecoder" is the first thing you google** when you want to run a coding
   agent in CI, on a cron, or from Slack.

2. **Companies fork it** as the foundation for their internal coding agent
   infra (like Stripe did with their own system).

3. **The English blueprint + guardrails pattern becomes a standard** — other
   tools adopt the same concept of English-described workflows with
   deterministic quality enforcement.

4. **Contributors add agents, channels, and blueprints** without touching the
   core engine — the extension points are that clean.

5. **The README example works in 30 seconds** for any developer with Docker
   installed.
