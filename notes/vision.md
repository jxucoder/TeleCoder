# TeleCoder Vision

## One Sentence

TeleCoder is a self-hosted coding agent that runs on your VPS, keeps working
when you disconnect, and acts within boundaries you control.

## Who It Is For

TeleCoder is for:

- individual developers
- indie hackers
- freelancers
- very small teams

It is for people who want an always-on coding agent they control on their own
machine, not a hosted platform and not an enterprise system.

## The Problem

Most coding agents are session tools. You open a terminal, give a prompt, wait
for an answer, and the useful context dies with the session.

That is fine for short interactive work. It breaks down when the user wants the
agent to keep running after they close the laptop, reconnect later from another
device, or keep an eye on a repo while they are away.

The laptop is the wrong host for this behavior. It sleeps, disconnects, and is
not meant to be a durable worker. A VPS is a better host:

- it stays online
- it is under the user's control
- the repo, credentials, and runtime stay on a machine the user owns
- it can keep state between sessions

What the user actually wants:

- a coding agent that keeps working after they disconnect
- a persistent place to resume work later
- useful outputs: diffs, logs, test results, summaries
- standing instructions for repetitive repo work
- clear approval gates for risky actions
- a simple way to see what happened while they were away

## The Product Thesis

The interesting product is not a better prompt box.

The interesting product is:

**an always-on coding agent that can watch, act, verify, and report within
clear operating boundaries**

Reactive prompting matters, but it is table stakes. The real step forward is a
system that can keep going without the user babysitting a terminal.

## What TeleCoder Is

TeleCoder is:

- a self-hosted service installed on a VPS the user controls
- a persistent remote session system for coding work
- an orchestration layer above existing coding runtimes
- opinionated at the core and extensible at the edges
- proactive when configured through watches
- reactive when the user wants to prompt it directly
- focused on code work, repo state, verification, and summaries
- reachable through CLI first, with web and chat surfaces where useful

TeleCoder runs coding agents through `acpx`, which gives it a common runtime
interface instead of forcing TeleCoder to build its own per-agent protocol.

## What TeleCoder Is Not

TeleCoder is not:

- a general personal assistant
- a hosted SaaS where TeleCoder owns the infrastructure
- a replacement for the user's IDE
- a new foundation model or a new coding agent runtime
- an "autonomous at any cost" system
- a workflow engine for every business function

## The Core Promise

The product promise is simple:

1. install TeleCoder on a Linux VPS
2. connect credentials and a repo
3. start a session or configure a watch
4. leave
5. come back to useful results and clear decisions

If the user returns to confusion, surprise changes, or raw model output, the
product has failed.

## Two Modes

TeleCoder supports both reactive and proactive use. Both matter, but proactive
behavior is the differentiator.

### Reactive

The user sends a prompt and TeleCoder runs the work on the VPS:

- "Fix the failing test in auth.go"
- "Refactor the billing module"
- "What changed in this package?"

This must already be a good product on its own: durable sessions, durable
workspaces, logs, artifacts, and resumable state.

### Proactive

The user defines standing instructions. TeleCoder reacts to events within those
instructions:

- watch CI on `main`; if it fails, investigate and draft a fix
- watch a PR for review comments and prepare updates
- run the full test suite nightly and report flaky tests
- keep a working branch current and report conflicts
- monitor a deploy window when the required integration exists

The user should not need to sit in front of a terminal waiting for the moment
that work becomes actionable.

## The Safety Model

This is the most important design requirement.

TeleCoder is only useful if it is both active and predictable. That means the
system needs more than approval buttons. It needs explicit containment rules.

### Rule 1: Autonomous writes stay contained

By default, proactive write actions must happen in an isolated workspace or
branch owned by TeleCoder, not directly on the user's primary branch.

TeleCoder may:

- investigate
- run tests and linters
- generate patches
- apply low-risk changes in its own workspace
- commit to a TeleCoder-managed branch when policy allows

TeleCoder may not, without explicit approval:

- push to a shared remote
- open or update a PR on the user's behalf
- merge
- deploy
- delete branches, tags, or infrastructure
- rewrite a branch the user is actively using

Containment matters more than autonomy. A safe draft branch is useful. A silent
write to the wrong branch is not.

### Rule 2: Risk is graduated

Actions are grouped by risk:

| Risk | Examples | Default |
|------|----------|---------|
| Read-only | investigate, analyze, summarize, test, lint | Auto |
| Contained write | format, small fix, rebase in TeleCoder branch | Auto or Ask |
| Repo write | commit, push branch, post PR reply, open PR | Ask |
| High-risk | merge, deploy, delete, privileged action | Always ask |

Users can override defaults per watch, per repo, or globally:

```yaml
trust:
  auto_approve: [investigate, test_run, lint_run]
  ask_before: [commit, push_branch, pr_reply]
  always_ask: [merge, deploy, delete]
```

### Rule 3: Spending must be bounded

Every watch and every job should have a budget. When providers expose reliable
token or cost telemetry, TeleCoder can enforce dollar budgets directly. When
they do not, TeleCoder should fall back to a normalized budget such as request
count, token estimate, runtime duration, or hard stop conditions.

The product requirement is not "perfect billing math." The product requirement
is "no runaway work and no silent runaway spend."

### Rule 4: Failed loops must stop

TeleCoder needs circuit breakers:

- stop after repeated failures on the same issue
- stop if new changes make the situation worse
- stop if the agent is not making progress
- stop if required verification cannot run
- stop when a budget is exhausted

When TeleCoder stops, it should report what it tried, what it learned, and what
decision it needs from the user.

### Rule 5: Trust includes security boundaries

For a self-hosted agent, "trust" also includes:

- where credentials are stored
- which credentials a repo or watch is allowed to use
- how webhook signatures are verified
- which chat channels are allowed to approve actions
- how user identity is confirmed for approvals
- what audit trail exists for sensitive actions

Approval flows are part of the trust model, but not the whole trust model.

### The Standard

The user should feel:

**this agent is useful, visible, and under control**

## Architecture

```
[User]
  |
  v
[CLI / Web / Chat]
  |
  v
[TeleCoder Service on User VPS]
 - session engine
 - event router
 - policy engine
 - trust and approval layer
 - scheduler
 - notification layer
 - SQLite state
 - local artifacts and logs
 - credential and webhook handling
  |
  +---------------------------------------+
  |                                       |
  v                                       v
[Reactive Path]                     [Proactive Path]
 - prompt from user                 - GitHub webhooks
 - direct session                   - cron schedules
 - resume session                   - repo state watchers
                                     - future external event sources
  |                                       |
  v                                       v
[Job / Session Layer]
 - one durable session per task
 - queueing and prioritization
 - isolated workspace per session
 - verification and result capture
 - branch ownership and write lease
  |
  v
[acpx Runtime Layer]
 - ACP over JSON-RPC 2.0 / stdio
 - persistent sessions
 - prompt queueing
 - reconnect and cancellation
  |
  +----+----+----+----+
  |    |    |    |    |
  v    v    v    v    v
[Claude] [Codex] [Gemini] [OpenCode] [others]
```

### Why `acpx`

`acpx` already solves the runtime protocol layer:

- persistent ACP sessions
- named sessions
- prompt queueing
- cancellation
- reconnect after process failure
- support for multiple runtimes through one interface

TeleCoder should not rebuild that. TeleCoder should own the product layer above
it: sessions, watches, policies, approvals, verification, and user experience.
That keeps the core product opinionated while leaving room to add runtimes and
integrations through standard interfaces.

## Sessions

Sessions are the base product unit.

A session should have:

- a durable workspace
- a durable conversation and event history
- visible status
- logs and artifacts
- verification results when configured
- a clear result summary

If sessions are weak, proactive behavior will be weak too.

## Watches

Watches are standing instructions: monitor a condition, evaluate what happened,
and decide whether to act.

### Example Watch Types

**CI Watch**: monitor CI for a branch. On failure, investigate, verify, and
prepare a fix when possible.

```bash
telecoder watch ci --repo api-server --branch main
```

**PR Watch**: monitor a PR for review comments or status changes and prepare
code updates or draft replies.

```bash
telecoder watch pr 231 --repo api-server --address-comments
```

**Schedule Watch**: run a task on a schedule.

```bash
telecoder watch schedule --repo api-server --cron "0 2 * * *" --task "run full test suite, report flaky tests"
```

**Branch Watch**: keep a TeleCoder-managed working branch current and report
conflicts before they affect the user.

```bash
telecoder watch branch --repo api-server --branch telecoder/auth-fix --rebase-on main
```

**Deploy Watch**: monitor a deploy window when the repo is connected to the
required logs or telemetry source.

```bash
telecoder watch deploy --repo api-server --for 30m
```

### Watch Lifecycle

1. The user creates a watch.
2. TeleCoder stores the watch and registers its trigger conditions.
3. An event fires.
4. The policy engine decides whether to ignore, investigate, act, or ask.
5. TeleCoder records the outcome and sends the right summary or approval
   request.

### Watch Output

A watch should produce something useful even when it stops short of acting:

- "CI failed on `main`. Root cause identified. Draft fix prepared on branch `telecoder/ci-auth-fix`. Approve push?"
- "PR 231 has 3 new comments. 2 addressed in code, 1 needs product input. Draft replies ready."
- "Nightly test run: 47 passed, 2 flaky. Investigation attached."
- "Deploy stable after 30 minutes. No significant error change detected."

The user should benefit from the investigation even when they reject the next
action.

## The Return Experience

This is the signature user experience.

When the user comes back after hours away, TeleCoder should present a concise
summary of what changed, what completed, what failed, and what needs approval.

```text
Since you were last here (8 hours ago):

Watches:
  CI on main: failed once, draft fix prepared
  PR 231: 2 comments addressed, 1 needs input
  Nightly tests: 47/49 passed, 2 flaky

Sessions:
  fix-checkout-tests: complete
  refactor-billing: waiting for approval

Approvals:
  Push branch telecoder/pr-231-fixes?
  Commit refactor-billing changes?
```

This summary should be:

- concise first
- actionable
- available across surfaces
- honest about uncertainty
- linked to the underlying logs, diffs, and verification

The product should feel like a reliable handoff, not like a pile of background
noise.

## User Experience

### First Run

- install TeleCoder on a VPS
- configure model and git credentials
- connect a repo
- start a session or create a watch

The setup bar is low: one clear path, minimal choices, and fast time to first
useful result.

### During Work

After starting a session or enabling a watch, the user should be free to:

- close the laptop
- disconnect SSH
- switch devices
- come back later

The machine stays on. The work stays stateful.

### Approval Moments

TeleCoder should only interrupt when a real decision exists:

- "I found the CI failure and prepared a fix. Push the branch?"
- "I addressed 2 of 3 PR comments. Post the replies?"
- "I found flaky tests. Investigate further?"

The user can approve, reject, or request more detail.

## Scope Boundary

### In Scope

- code changes, refactors, bug fixes
- test and lint verification
- CI failure diagnosis
- PR review handling
- branch management in TeleCoder-owned branches
- repo and code questions
- changelogs and release summaries
- scheduled maintenance work
- deploy monitoring when the required integration exists

### Out of Scope

- general personal assistant tasks
- email, calendar, and scheduling
- broad business automation
- sales, marketing, or operations workflows
- "click around the internet and do everything" behavior

The rule is simple: if it helps change, verify, debug, review, or summarize
software work, it belongs. Otherwise it probably does not.

## Extensibility

TeleCoder should be opinionated at the core and extensible at the edges.

The core should stay narrow and reliable:

- sessions
- watches
- policies and approvals
- verification
- summaries

Developers should be able to extend TeleCoder with:

- new watch sources
- new notification or approval channels
- repo-specific verification commands
- custom policies and trust rules
- additional runtimes and integrations through standard interfaces

The goal is not a plugin marketplace and not a generic automation framework.
The goal is a system developers can adapt to their own workflows without losing
the product's clarity.

## Product Direction

TeleCoder should build depth in a narrow space:

- durable remote coding sessions
- safe proactive repo work
- strong verification and summaries
- simple self-hosted operation

It should not win by claiming to do everything. It should win by being the most
trustworthy way to let a coding agent keep working on your own machine.

## Success Criteria

TeleCoder is successful if users say:

- "I set it up without drama."
- "I left it running and came back to useful work."
- "I trust the boundaries."
- "I can see what happened quickly."
- "It helped with repo work without surprising me."
- "It is simpler than stitching this together myself."

## Final Framing

TeleCoder is a self-hosted coding agent for people who want durable sessions
and controlled automation on their own VPS.

It keeps working when they disconnect. It watches what they ask it to watch. It
acts in contained workspaces. It verifies what it can. It reports clearly. It
asks before crossing a boundary.

The product is:

**a coding agent that works while you are away, on a machine you control, with
trust you can inspect**
