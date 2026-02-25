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
- Blueprint-based workflow (deterministic verification between agentic steps)
- Sandbox isolation with pre-warming
- Feedback loops (lint → fix → test → fix)
- Smart output routing (PR, text reply, report, PR comments — whatever fits)
- CI integration and bounded retries

TeleCoder owns that 80%. It doesn't replace Claude Code — it makes Claude Code
(and Codex, and OpenCode, and Aider, and your custom agent) work reliably in
production.

### What makes projects maximally popular

1. **Clear identity** — one sentence explains what it is
2. **Instant gratification** — working in 30 seconds
3. **Small core** — fits in your head (and in an AI's context window)
4. **Low floor, high ceiling** — easy to start, scales to Stripe
5. **Extensible without complexity** — plugins, not config sprawl

TeleCoder's identity: **"Run any coding agent in a sandbox with blueprint
orchestration. Send a task, get a result — PR, text answer, report, whatever
the task requires."**

---

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                       TeleCoder                           │
│                                                          │
│  ┌──────────┐   ┌──────────┐   ┌───────────┐            │
│  │ Channels  │──▶│  Engine   │──▶│ Blueprint │            │
│  │           │   │          │   │           │            │
│  │ CLI       │   │ Sessions │   │ context   │            │
│  │ HTTP API  │   │ Events   │   │ implement │            │
│  │ Slack     │   │ Memory   │   │ lint+fix  │──▶ Sandbox │
│  │ Telegram  │   │ Dispatch │   │ test      │   (Docker) │
│  │ GitHub    │   │          │   │ fix       │            │
│  │ Linear    │   │          │   │ finalize  │──▶ Output  │
│  │ Jira      │   │          │   │           │   (PR,text,│
│  │           │   │          │   │           │    report) │
│  └──────────┘   └──────────┘   └───────────┘            │
│                                                          │
│  ┌────────────────────────────────────────────────────┐  │
│  │  Store (SQLite) · Memory (code+notes+sessions)    │  │
│  │  Pre-warm Pool · Event Bus (SSE)                  │  │
│  └────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────┘
```

Four layers, each independently useful:

| Layer | What | Lines | You need it when... |
|-------|------|-------|---------------------|
| **Sandbox** | Container lifecycle, pre-warm pool (Docker default; E2B, Modal, Fly pluggable) | ~500 | You want isolated execution |
| **Engine** | Session management, events, memory, dispatch | ~1500 | You want to orchestrate agents |
| **Blueprint + Guardrails** | English workflow descriptions + deterministic quality/security checks | ~500 | You want reliable multi-step flows |
| **Channels** | Slack, Telegram, GitHub, CLI, HTTP API | ~200 each | You want integrations |

Total core: **~4500 lines of Go**. The rest is channels, tests, CLI, and web UI.

---

## The Two Core Ideas

### 1. Blueprints — English-First Workflow Descriptions

A blueprint is a **natural language description** of what the agent should do.
Not Go. Not YAML. Not shell scripts. **English.**

```markdown
# .telecoder/blueprint.md

You are a coding agent working on this repository.

When given a task:
1. Understand the codebase context and coding conventions
2. Implement the requested changes
3. Make sure all tests pass
4. If tests fail, fix your changes (up to 2 attempts)
5. Write a clear PR description explaining what changed and why

If the task is a question (not a code change), just answer it directly.
```

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

**Different blueprints for different tasks:**

```markdown
# .telecoder/blueprints/review.md
You are a code reviewer. Analyze the PR diff carefully.
List issues by severity (critical, warning, nit).
Focus on: correctness, security, performance, readability.
Never suggest style changes that contradict the project's linter config.
```

```markdown
# .telecoder/blueprints/security-audit.md
You are a security auditor. Scan the codebase for vulnerabilities.
Run semgrep and trivy if available. Synthesize findings into a
readable report grouped by severity. Include remediation advice.
```

```markdown
# .telecoder/blueprints/data-analysis.md
You are a data analyst. Run the analysis scripts in scripts/,
interpret the output, and produce a summary report with key findings
and recommended actions.
```

The agent does the agentic work. The blueprint tells it what to focus on.

### 2. Guardrails — Deterministic, Non-Negotiable, Always-On

Here's what Stripe learned: you don't ask the agent to run tests. You don't
ask the agent to scan for secrets. You don't write a blueprint step for it.
**The framework does it automatically, every time, non-negotiably.**

Guardrails are **deterministic checks** that run before and after agent
execution. They are not part of the blueprint. They are not opt-in. They are
the framework's job.

```
     Blueprint (English)
            │
     ┌──────▼──────┐
     │  PRE-GUARD   │  Context enrichment, rules injection,
     │              │  secret scan on inputs
     └──────┬──────┘
            │
     ┌──────▼──────┐
     │    AGENT     │  The LLM agent does the work
     │  (sandboxed) │  (Claude Code, Codex, OpenCode, etc.)
     └──────┬──────┘
            │
     ┌──────▼──────┐
     │  POST-GUARD  │  Secret scan on output, lint check,
     │              │  test run, size limits, scope check
     └──────┬──────┘
            │
        Pass? ──No──► Feed failures back to agent (bounded)
            │
           Yes
            │
     ┌──────▼──────┐
     │   OUTPUT     │  Auto-detect: PR, text reply, comments,
     │              │  report — whatever fits
     └─────────────┘
```

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

**Custom guardrails** — any executable that returns pass/fail:

```yaml
# .telecoder/guardrails.yaml
post:
  - name: type-check
    run: npx tsc --noEmit
  - name: security-scan
    run: semgrep --config=auto --error .
  - name: no-console-log
    run: "! grep -r 'console.log' src/"
```

**Why this split matters:**

- The **blueprint** (English) is for the team — it describes what you want.
- The **guardrails** (deterministic) are for the framework — they enforce
  quality and security regardless of what the blueprint says.
- An agent can ignore an English instruction. It **cannot** bypass a guardrail.

This is the Stripe lesson distilled: hybrid orchestration where the agentic
parts are flexible (English) and the deterministic parts are rigid (guardrails).

### How They Work Together

```
User: "add rate limiting to /api/users"
         │
         ▼
┌─ Blueprint (.telecoder/blueprint.md) ───────────────────┐
│ "Implement the requested changes. Make sure all tests   │
│  pass. Write a clear PR description."                   │
└─────────────────────────────────────────────────────────┘
         │
         ▼ (prepended to prompt)
┌─ Pre-Guardrails ────────────────────────────────────────┐
│ • Gather context: related code, CLAUDE.md rules, notes  │
│ • Secret scan inputs: clean                             │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─ Agent Execution (sandboxed) ───────────────────────────┐
│ Claude Code runs with enriched prompt                    │
│ Edits files, writes tests, etc.                         │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─ Post-Guardrails ───────────────────────────────────────┐
│ • Secret scan output: clean                             │
│ • Lint: ✓ passed                                        │
│ • Tests: ✗ 2 failures → feed back to agent (round 1/2) │
│ • [agent fixes] → Tests: ✓ passed                       │
│ • Size check: 4 files changed (ok)                      │
└─────────────────────────────────────────────────────────┘
         │
         ▼
┌─ Output ────────────────────────────────────────────────┐
│ Files changed → commit, push, create PR #203            │
└─────────────────────────────────────────────────────────┘
```

### Go Escape Hatch (for framework builders)

For companies building products on TeleCoder that need precise orchestration
control, blueprints can still be Go functions:

```go
type Blueprint func(ctx context.Context, run *Run) error
```

The Go path gives you full control over multi-agent orchestration, custom
verification logic, external API calls, etc. But it's the power-user path,
not the default.

```go
// Multi-agent: plan with a fast model, implement with a strong one
func PlanAndImplement(ctx context.Context, run *Run) error {
    run.GatherContext()
    plan, _ := run.AgentWith("haiku", "Create a detailed plan for: "+run.Prompt)
    run.Agent("Implement this plan:\n" + plan)
    return run.Finalize()  // guardrails still run automatically
}
```

Even Go blueprints get guardrails applied automatically. You can't accidentally
skip the secret scan.

### The Run Object — Layered Primitives

The Run has two layers: **generic primitives** that work for any agent task,
and **coding helpers** that build on them for software development workflows.

```go
type Run struct {
    Session      *model.Session
    Prompt       string          // the enriched prompt
    ContainerID  string          // active sandbox container
    MaxRevisions int
    engine       *Engine
}

// ========= Layer 1: Generic primitives (work for ANY agent task) =========

// Sandbox
func (r *Run) Exec(cmd ...string) *ExecResult           // run any command
func (r *Run) ReadFile(path string) string               // read a file from sandbox
func (r *Run) WriteFile(path, content string)            // write a file in sandbox

// Agent
func (r *Run) Agent(prompt string) (string, error)       // invoke agent, return output
func (r *Run) AgentWith(model, prompt string) (string, error)

// Context
func (r *Run) GatherContext()                            // enrich prompt with memory + rules

// Output
func (r *Run) Reply(text string)                         // return a text answer
func (r *Run) Finalize() error                           // smart default based on what happened
func (r *Run) Emit(eventType, data string)               // emit SSE event

// State
func (r *Run) HasChanges() bool                          // did files change in the sandbox?
func (r *Run) SetPrompt(prompt string)

// ========= Layer 2: Coding helpers (convenience for software dev) =========

func (r *Run) Verify() *VerifyResult                     // auto-detect lint + test commands, run them
func (r *Run) LintFix()                                  // run linter with --fix
func (r *Run) Test() *VerifyResult                       // run test suite only
func (r *Run) Push()                                     // git commit + push
func (r *Run) PushAmend()                                // amend + force push
func (r *Run) CreatePR() *PRResult                       // create a pull request
func (r *Run) CommentOnPR(prNumber int, body string)     // post comments on existing PR
func (r *Run) WaitForCI(pr *PRResult, timeout time.Duration) *CIResult
```

**Layer 1 is the framework.** It works for any async agent task: data pipelines,
infra automation, research, analysis, anything that runs in a sandbox.

**Layer 2 is the opinionated default.** It's why TeleCoder is great for coding
agents specifically. But Layer 2 is built entirely on Layer 1 — `Verify()` is
just `Exec()` with auto-detection, `Push()` is just `Exec("git", ...)`,
`CreatePR()` is just a GitHub API call.

A framework builder who doesn't care about PRs or linting never touches
Layer 2. They use `Exec()`, `Agent()`, `Reply()` and build whatever workflow
they need.

---

## Multi-Repo Tasks

Real-world tasks often span multiple repositories. Update an API and its
client. Change a shared schema and all its consumers. Upgrade a library
across dependent repos.

### The wrong approach: pre-define repos programmatically

```json
{
  "repos": [
    {"url": "github.com/acme/api-server"},
    {"url": "github.com/acme/python-client"}
  ]
}
```

This is limiting. You're deciding upfront which repos the agent needs. But
the agent might discover mid-task that it needs another repo. Or realize the
client library is fine and only the server needs changes. Pre-defining repos
is the same mistake as pre-defining steps — it constrains the agent to what
you anticipated.

### The right approach: give the agent git access, let it work

The sandbox has git credentials. The agent can clone any repo it has access to.
It decides at runtime what it needs. The framework **watches and reacts**.

```
Task: "Add rate limiting to the API and update the Python client"
         │
         ▼
┌─ Sandbox (git credentials injected) ─────┐
│                                           │
│  Agent decides what to do:                │
│    $ git clone .../api-server             │
│    $ git clone .../python-client          │
│    (edits both)                           │
│    (maybe clones a third repo to check    │
│     how another service handles limits)   │
│                                           │
└───────────────────────────────────────────┘
         │
         ▼
┌─ Framework scans for changes ─────────────┐
│                                           │
│  Discovers: 2 git repos with changes      │
│  /home/user/api-server     → 4 files      │
│  /home/user/python-client  → 2 files      │
│                                           │
│  (third repo cloned but unchanged — skip) │
└───────────────────────────────────────────┘
         │
         ▼
┌─ Post-Guardrails (per-repo, automatic) ───┐
│                                           │
│  api-server:                              │
│    ✓ secret scan · ✓ lint · ✓ tests       │
│                                           │
│  python-client:                           │
│    ✓ secret scan · ✓ lint                 │
│    ✗ pytest failed → agent fixes          │
│    ✓ pytest passed (round 2)              │
│                                           │
└───────────────────────────────────────────┘
         │
         ▼
┌─ Output ──────────────────────────────────┐
│  PR #204 on api-server                    │
│  PR #87 on python-client                  │
│  (cross-linked in descriptions)           │
└───────────────────────────────────────────┘
```

### What the framework does (not the user)

1. **Before**: Inject git credentials into the sandbox so the agent can clone
2. **After**: Walk the filesystem, find all git repos with uncommitted changes
3. **Per changed repo**: Load its `.telecoder/` config, discover its lint/test
   commands, run guardrails
4. **Output**: Create a PR for each changed repo, cross-link them

The user's task request stays simple:

```json
{
  "prompt": "Add rate limiting to the API and update the Python client"
}
```

No `repos` field. No upfront configuration. The agent figures out which repos
it needs, the framework handles everything after.

### Why this is better

- Agent can discover dependencies dynamically ("I need to check how service X
  does this" → clones it, reads it, doesn't change it)
- Agent can decide mid-task that a repo doesn't need changes
- Agent can clone repos you didn't anticipate
- Zero configuration for multi-repo — it just works
- Single-repo tasks work identically (one repo detected, one PR created)

---

## Scoped Rules (Stripe Pattern)

Stripe discovered that **what's good for human developers is good for agents**.
Every codebase has conventions documented in `.cursorrules`, `CLAUDE.md`,
`AGENTS.md`, `.github/copilot-instructions.md`, etc. TeleCoder auto-discovers
and injects these.

```go
// During GatherContext(), the blueprint:
// 1. Searches for rules files in the repo
// 2. Finds the most relevant rules for the current task
// 3. Prepends them to the prompt

// Auto-discovered files (checked in order):
var rulesFiles = []string{
    "CLAUDE.md",
    "AGENTS.md",
    ".cursorrules",
    ".github/copilot-instructions.md",
    ".telecoder/rules.md",
    ".telecoder/rules/*.md",
}
```

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
current hardcoded `runSubTask()` flow into the guardrails framework. Add
blueprint loading from `.telecoder/blueprint.md`.

| Task | What | Files |
|------|------|-------|
| 1.1 | Define `Blueprint` type (English loader + Go func), `Run` struct | `pkg/blueprint/blueprint.go` |
| 1.2 | Blueprint discovery: `.telecoder/blueprint.md` → prepend to prompt | `pkg/blueprint/loader.go` |
| 1.3 | Guardrails framework: pre-guard, post-guard, retry loop | `pkg/guardrail/guardrail.go` |
| 1.4 | Built-in guardrails: secret scan, lint, test, size limit | `pkg/guardrail/builtin.go` |
| 1.5 | Custom guardrails from `.telecoder/guardrails.yaml` | `pkg/guardrail/custom.go` |
| 1.6 | Wire into engine (`WithBlueprint()`, `WithGuardrails()` on Builder) | `telecoder.go`, `engine.go` |
| 1.7 | Scoped rules discovery in pre-guard (`CLAUDE.md`, `.cursorrules`, etc.) | `pkg/guardrail/rules.go` |

**Eval**: `go test ./...` passes. Default behavior identical but now with
guardrails enforced. English blueprints work via `.telecoder/blueprint.md`.

### Phase 2: Memory Security (trust)

Companies won't use a framework that leaks secrets into agent prompts.

| Task | What | Files |
|------|------|-------|
| 2.1 | Secret scanner (regex: AWS, GH tokens, JWT, keys) | `pkg/memory/security.go` |
| 2.2 | Integrate scanner into `insertChunk()` | `pkg/memory/codebase.go` |
| 2.3 | Skip sensitive files in `IsIndexable()` | `pkg/memory/chunker.go` |
| 2.4 | Content sanitization in `enrichPrompt()` | `pkg/memory/sanitize.go` |
| 2.5 | Audit events on memory operations | `pkg/memory/audit.go` |

**Eval**: No `.env` or `*.pem` files indexed. No API keys in retrieved chunks.

### Phase 3: Developer Experience (adoption)

| Task | What |
|------|------|
| 3.1 | `docker compose up` quickstart (one command, works) |
| 3.2 | "Build your first async agent in 5 minutes" tutorial |
| 3.3 | Example blueprints gallery (security, CI-aware, multi-agent) |
| 3.4 | README rewrite focused on the blueprint story |
| 3.5 | GitHub Actions template for "TeleCoder as CI bot" |

### Phase 4: Ecosystem (network effects)

| Task | What |
|------|------|
| 4.1 | MCP server — expose memory as MCP tools |
| 4.2 | MCP client — consume external tools as memory providers |
| 4.3 | Ollama embedder — zero-API-key local memory |
| 4.4 | Webhook-driven reindexing (reindex on git push) |
| 4.5 | Provider interfaces for memory (Qdrant, Mem0, etc.) |

---

## What NOT to Build (Yet)

The v3 design.md proposed a full conversational agent (Agent Loop, Heartbeat,
Skills, Gateway). That's the right long-term vision, but it's **too much
surface area for adoption**.

Projects get popular by doing **one thing brilliantly**, then expanding:
- Docker started as containers, then added Compose, Swarm, Hub
- FastAPI started as a framework, then added background tasks, WebSockets
- Next.js started as SSR React, then added API routes, middleware

TeleCoder's "one thing": **blueprint-orchestrated async agents in sandboxes —
for any task, with any outcome**. Once that's the default, expand to
conversations, heartbeat, and proactive monitoring.

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
   is the default runtime, but the `sandbox.Runtime` interface supports E2B,
   Modal, Fly, Firecracker, or any container platform.

### From Stripe Minions

4. **Separate agentic from deterministic** — English blueprints dispatch
   agents. Deterministic guardrails enforce quality and security. The agent
   can't bypass the guardrails. This is the hybrid orchestration pattern.

5. **Shift feedback left** — Lint locally before pushing. Test locally before
   creating the PR. Don't waste CI cycles on obvious failures.

6. **Bounded retries** — MaxRevisions = 1-2. More retries don't help — they
   waste tokens. Fix the prompt or the blueprint instead.

7. **Scoped rules** — What's good for human developers is good for agents.
   Discover and inject `CLAUDE.md`, `.cursorrules`, etc. automatically.

8. **Pre-warm everything** — Sandbox pool, code index, rules cache. The first
   session should feel instant.

### For Maximum Adoption

9. **30-second quickstart** — `docker compose up` and `curl`. No setup wizard,
   no account creation, no cloud dependency. Just works.

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
| Blueprint orchestration | Yes | No | No | No |
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

3. **The blueprint pattern becomes a standard** — other tools adopt the same
   concept of hybrid deterministic + agentic orchestration.

4. **Contributors add agents, channels, and blueprints** without touching the
   core engine — the extension points are that clean.

5. **The README example works in 30 seconds** for any developer with Docker
   installed.
