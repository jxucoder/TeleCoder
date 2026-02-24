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
| **Sandbox** | Docker container lifecycle, pre-warm pool | ~500 | You want isolated execution |
| **Engine** | Session management, events, memory, dispatch | ~1500 | You want to orchestrate agents |
| **Blueprint** | Workflow orchestration (deterministic + agentic steps) | ~500 | You want reliable multi-step flows |
| **Channels** | Slack, Telegram, GitHub, CLI, HTTP API | ~200 each | You want integrations |

Total core: **~4500 lines of Go**. The rest is channels, tests, CLI, and web UI.

---

## Blueprints — The Core Differentiator

A blueprint is a **Go function** that defines the workflow for processing a
task. Not necessarily a "coding task" — it could be a question, audit, review,
migration, or anything you'd ask an agent to do. Blueprints mix deterministic
steps (lint, test, git) with agentic steps (implement, fix, analyze). This is
the pattern Stripe proved works at scale.

```go
// Blueprint defines a workflow for processing a task.
// It receives a Run with building blocks and orchestrates them.
// The blueprint decides the output: PR, text reply, report, comments, etc.
type Blueprint func(ctx context.Context, run *Run) error
```

### The Default Blueprint

```go
func DefaultBlueprint(ctx context.Context, run *Run) error {
    // 1. Gather context (deterministic)
    //    Pull relevant code, notes, past sessions → enrich prompt
    run.GatherContext()

    // 2. Implement (agentic)
    //    Run the coding agent with the enriched prompt
    if err := run.Implement(); err != nil {
        return err
    }

    // 3. Did the agent change any files?
    if !run.HasChanges() {
        // No code changes — the agent produced a text answer.
        // (Question, analysis, explanation, etc.)
        return run.Finalize()
    }

    // 4. Lint + auto-fix (deterministic)
    run.LintFix()

    // 5. Test (deterministic)
    testResult := run.Test()

    // 6. Fix failures (agentic, bounded retries)
    for round := 0; !testResult.Passed && round < run.MaxRevisions; round++ {
        run.Fix("Tests failed:\n" + testResult.Output)
        run.LintFix()
        testResult = run.Test()
    }

    // 7. Finalize — auto-detects: changed files → PR, else text reply
    return run.Finalize()
}
```

**`run.Finalize()` is the smart default.** It checks what happened during the
run and produces the right output:
- Files changed → commit, push, create PR
- Text output only → return text reply
- Blueprint called `run.Reply()` explicitly → use that

No Blueprint type. No Node interface. No DAG executor. No YAML. **Just a
function.** This is the NanoClaw principle: code IS configuration.

### Why this matters

Every other open-source coding agent is a **single loop**: prompt → agent →
output. That works for demos. It doesn't work for production, because:

- Agents skip linting. Blueprints don't.
- Agents don't know when to stop retrying. Blueprints bound it.
- Agents can't mix deterministic and agentic steps. Blueprints compose them.
- Agents produce different output formats. Blueprints normalize the flow.
- Agents always produce one kind of output. Blueprints adapt to the task.

Stripe learned this the hard way and built blueprints internally. TeleCoder
makes the pattern open source.

### Custom Blueprints

Blueprints aren't just for "write code, make PR." They define any async
agent workflow:

```go
// Code review — reads a PR, produces comments (no PR output)
func ReviewPR(ctx context.Context, run *Run) error {
    run.GatherContext()

    // Agent analyzes the diff and produces structured feedback
    review, err := run.RunAgent("Review this PR. List issues by severity.")
    if err != nil { return err }

    // Post comments on the PR (deterministic)
    run.CommentOnPR(run.Session.PRNumber, review)
    return nil
}

// Security audit — scans the codebase, produces a report (no PR)
func SecurityAudit(ctx context.Context, run *Run) error {
    run.GatherContext()

    // Run scanners (deterministic)
    semgrep := run.Exec("semgrep", "--config=auto", "--json", ".")
    trivy := run.Exec("trivy", "fs", "--format", "json", ".")

    // Agent synthesizes findings into a readable report (agentic)
    report, err := run.RunAgent(fmt.Sprintf(
        "Summarize these security findings:\n\nSemgrep:\n%s\n\nTrivy:\n%s",
        semgrep.Output, trivy.Output,
    ))
    if err != nil { return err }

    // Return the report as text — no PR, no code changes
    run.Reply(report)
    return nil
}

// Multi-agent: plan with a fast model, implement with a strong one
func PlanAndImplement(ctx context.Context, run *Run) error {
    run.GatherContext()

    plan, err := run.RunAgentWith("haiku", "Create a step-by-step plan for: "+run.Prompt)
    if err != nil { return err }

    run.SetPrompt("Implement this plan:\n" + plan)
    if err := run.Implement(); err != nil { return err }

    run.LintFix()
    run.Test()
    return run.Finalize() // auto-detect: PR if changes, text if not
}

// CI-aware: wait for CI and fix failures
func CIAware(ctx context.Context, run *Run) error {
    run.GatherContext()
    if err := run.Implement(); err != nil { return err }
    run.LintFix()
    run.Test()
    run.Push()
    pr := run.CreatePR()

    ciResult := run.WaitForCI(pr, 10*time.Minute)
    if !ciResult.Passed {
        run.Fix("CI failed:\n" + ciResult.Logs)
        run.LintFix()
        run.Test()
        run.PushAmend()
    }

    return nil
}

// Question-only: never creates a PR, always returns text
func AnswerQuestion(ctx context.Context, run *Run) error {
    run.GatherContext()
    answer, err := run.RunAgent(run.Prompt)
    if err != nil { return err }
    run.Reply(answer)
    return nil
}
```

### The Run Object

```go
// Run provides building blocks that blueprints compose.
type Run struct {
    Session      *model.Session
    Prompt       string          // the enriched prompt
    ContainerID  string          // active sandbox container
    MaxRevisions int

    engine  *Engine             // access to sandbox, store, memory, git
    agent   agent.CodingAgent   // the coding agent to use
}

// --- Deterministic steps ---

func (r *Run) GatherContext()                           // enrich prompt with memory + rules
func (r *Run) LintFix()                                 // run linter with --fix
func (r *Run) Test() *VerifyResult                      // run test suite
func (r *Run) Push()                                    // git commit + push
func (r *Run) PushAmend()                               // amend + force push
func (r *Run) Exec(cmd ...string) *ExecResult           // run arbitrary command
func (r *Run) HasChanges() bool                         // did the agent modify files?

// --- Agentic steps ---

func (r *Run) Implement() error                         // run coding agent with current prompt
func (r *Run) Fix(feedback string) error                // run agent with fix prompt
func (r *Run) RunAgent(prompt string) (string, error)   // run agent, return text output
func (r *Run) RunAgentWith(model, prompt string) (string, error)

// --- Output (the blueprint decides what the result looks like) ---

func (r *Run) Finalize() error                          // smart default: PR if changes, text if not
func (r *Run) Reply(text string)                        // return a text answer
func (r *Run) CreatePR() *PRResult                      // create a pull request
func (r *Run) CommentOnPR(prNumber int, body string)    // post comments on existing PR
func (r *Run) WaitForCI(pr *PRResult, timeout time.Duration) *CIResult

// --- State ---

func (r *Run) SetPrompt(prompt string)
func (r *Run) Emit(eventType, data string)              // emit SSE event
```

The key insight: **output is not hardcoded**. A blueprint for code tasks calls
`Finalize()` (auto-detects PR vs text). A blueprint for code review calls
`CommentOnPR()`. A blueprint for security audits calls `Reply()`. The
framework doesn't assume what the agent is doing — the blueprint does.

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

### Phase 1: Blueprints (the differentiator)

**Goal**: Extract the current hardcoded `runSubTask()` flow into the blueprint
pattern. Zero behavior change — just make the orchestration explicit and
customizable.

| Task | What | Files |
|------|------|-------|
| 1.1 | Define `Blueprint` type and `Run` struct | `pkg/blueprint/blueprint.go` |
| 1.2 | Implement `DefaultBlueprint` (mirrors current flow) | `pkg/blueprint/default.go` |
| 1.3 | Split `runVerify()` into `LintFix()` and `Test()` | `pkg/blueprint/steps.go` |
| 1.4 | Wire blueprints into engine (`WithBlueprint()` on Builder) | `telecoder.go`, `engine.go` |
| 1.5 | Add `WaitForCI()` step (poll GitHub Actions) | `pkg/blueprint/ci.go` |
| 1.6 | Add scoped rules discovery in `GatherContext()` | `pkg/blueprint/rules.go` |

**Eval**: `go test ./...` passes. Default behavior identical. Custom blueprints
work via Builder.

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

2. **Code IS configuration** — No YAML schema to learn. Blueprints are Go
   functions. Skills (future) are markdown files. If you can read the code,
   you understand the system.

3. **Containers by default** — Every coding task runs in an isolated Docker
   container. No scary permissions. No `--dangerously-skip-permissions`.

### From Stripe Minions

4. **Hybrid orchestration** — Mix deterministic steps (lint, test, git) with
   agentic steps (implement, fix). This is the blueprint pattern.

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
| Sandbox by default | Docker | Optional | Yes | Docker |
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
