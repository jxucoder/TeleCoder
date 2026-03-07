# TeleCoder Vision

## One Sentence

TeleCoder is the simplest way to run a coding agent on your own VPS, keep it
working after your laptop is closed, and come back later to a real result.

## The User

TeleCoder is for:

- individual developers
- indie hackers
- freelancers
- very small teams

It is for people who want a remote coding agent quickly, not a company platform.

## The Problem

Local coding agents are powerful, but the laptop is the wrong place for
long-running work.

Your laptop:

- sleeps
- disconnects
- gets cluttered
- holds personal credentials
- is awkward for running multiple agent tasks at once

The user does not actually want "AI on my laptop."
The user wants:

- a coding agent that keeps working
- a workspace that stays alive
- something they control
- something easy to set up

## The Core Insight

The simplest useful product is not "a better coding agent."

It is:

**a remote coding box with sessions**

That is enough to create value.

## What TeleCoder Is

TeleCoder is:

- a remote-first coding tool
- centered on persistent sessions
- installed on a VPS the user controls
- focused on useful outputs like diffs, branches, PRs, logs, and test results
- simple to set up and simple to understand

## What TeleCoder Is Not

TeleCoder is not:

- an enterprise developer platform
- a general automation framework
- a chat app with shell access
- a replacement for every IDE workflow
- a product that needs complex cloud architecture in v1

## Product Promise

The promise should be concrete:

1. Get a Linux VPS
2. Run one install command
3. Add model and git credentials
4. Start a session
5. Let it work remotely
6. Resume later from anywhere

If this does not feel easy, the product has failed.

## The V1 Goal

The first version should do one thing very well:

**run persistent remote coding sessions on a VPS**

Everything else is secondary.

That means v1 should be optimized for:

- fast setup
- reliable sessions
- simple resume
- clear outputs
- basic verification

Not for:

- large-team workflows
- enterprise auth
- deep integrations
- broad automation

## V1 Defaults

To stay simple, v1 should make a few hard choices.

### Platform

V1 should target one common Linux VPS setup first.

Example:

- Ubuntu VPS
- one install command
- one long-running TeleCoder service

### Runtime

V1 should support **one runtime well**.

The simplest default is:

- Claude Code first

Support for Codex or other runtimes can come later.

### Persistence

V1 should persist state locally on the VPS.

That means:

- session metadata in SQLite
- logs and artifacts on local disk
- one workspace or worktree per session

This is simple, understandable, and good enough for the first version.

### Isolation

V1 does not need perfect isolation.

The simplest reliable starting point is:

- one process per session
- one workspace per session

Containers or stronger sandboxing can come later.

### Git

V1 should assume the VPS can clone and push to the user's repo.

Core value does not depend on deep GitHub integration.

Useful v1 outputs are:

- local changes
- diff summary
- branch
- pushed branch when configured

PR creation is valuable, but not required for the first useful version.

## User Experience

The UX should feel like:

**"this is my remote coding box."**

### First Run

The first-run experience should be minimal:

- install TeleCoder
- add credentials
- point it at a repo
- start using it

The setup target is minutes, not hours.

### Starting Work

The user should be able to say things like:

- "fix this failing test"
- "review this diff and make the requested changes"
- "refactor this module and run the tests"
- "investigate why CI is failing"

No special command language should be required for normal use.

### During Work

After the session starts, the user should be free to:

- close the browser
- close the laptop
- disconnect SSH
- switch devices

The session should keep running remotely.

### Returning Later

When the user comes back, they should see:

- what the agent did
- what changed
- what passed
- what failed
- what the next step is

The experience should favor continuation, not restarting from zero.

## Core Product Concept

The core object in TeleCoder is the **session**.

A session is a remote unit of work with:

- a repo
- a working state
- prompt history
- execution history
- outputs
- status

Typical lifecycle:

`created -> running -> verifying -> completed | failed | stopped`

This should stay simple.

## Outputs

TeleCoder should produce concrete outputs, not just chat text.

Typical outputs:

- diff summary
- changed files
- branch
- PR when configured
- logs
- test results
- failure diagnosis

The output should make the session useful even when the task is incomplete.

## Verification

TeleCoder should try to prove its work in simple ways.

In v1, that mostly means:

- run tests when possible
- run linters when possible
- show what passed
- show what failed
- summarize failures clearly

This alone makes the product much more useful than a remote shell with LLM text.

## Product Surface

Keep v1 narrow.

### v1

- CLI
- simple web UI

That is enough.

### Later

- GitHub entry points
- small-team sharing
- optional chat surfaces

These are later features, not the product core.

## Architecture

Keep the mental model obvious.

```
[User]
  |
  v
[CLI / Web]
  |
  v
[TeleCoder on VPS]
 - TeleCoder service
 - SQLite state
 - local artifacts
  |
  +-----------------------------+
  |                             |
  v                             v
[Session Workspace]        [Session Workspace]
 - repo                    - repo
 - working state           - working state
 - agent runtime           - agent runtime
 - logs                    - logs
 - outputs                 - outputs
```

This is enough for the vision.

If stronger isolation is needed, sessions can later run in:

- separate processes
- worktrees
- containers
- VMs

But v1 should choose the simplest reliable approach.

## The Agent

TeleCoder is the product around the agent, not the agent itself.

Inside each session, TeleCoder runs a coding agent runtime.

Important simplification:

v1 should support **one runtime well**, and that runtime should be part of the
default install story.

Multiple runtimes can come later.

The user should care that TeleCoder works remotely and reliably.
They should not need to care about the internal runtime abstraction.

## Security

The main safety property is simple:

the agent runs on the VPS, not on the user's laptop.

That means:

- mistakes happen away from the personal machine
- credentials can be scoped to the VPS
- the remote workspace can be controlled more cleanly

This is not enterprise-grade security in v1.
It is a simpler and safer default than local execution.

## Best Early Use Cases

The best early use cases are:

- overnight coding work
- bug fixes
- refactors
- PR follow-up
- test failure investigation
- long-running tasks the user does not want on a laptop
- deployment watch and report

These are the right starting point because they fit the product's core value.

## Scope Boundary

TeleCoder should focus on coding and coding-adjacent work.

### In Scope

- writing and changing code
- fixing tests
- refactors
- PR follow-up
- CI investigation
- release or deploy readiness checks
- monitoring a deployment and reporting status
- checking logs, errors, and basic metrics when they relate to software changes

### Out Of Scope

- general personal assistant work
- scheduling
- email drafting
- generic research unrelated to software work
- sales, marketing, or operations workflows that do not connect back to code

This keeps the product simple.

The rule is:

if the task helps change, verify, ship, or debug software, it probably belongs
in TeleCoder.

## Why Not Just Use Claude Code

Claude Code is already very strong.

TeleCoder should not try to win by claiming to be a better general coding agent.

TeleCoder should win on a narrower promise:

**your own remote coding box on your own VPS**

If a user wants the best hosted general coding agent experience, Claude Code may
still be the better product.

TeleCoder matters when the user specifically wants:

- self-hosting
- a persistent remote workspace
- a setup they control
- a session that keeps running on their own machine

It should also be simpler than nearby self-hosted projects.

The wedge is not "more features."
The wedge is:

- fewer decisions
- faster setup
- one opinionated path that works
- a clearer remote-session model

## Success Criteria

TeleCoder is successful if users say:

- "I set it up fast."
- "I can leave it running and come back later."
- "It is more comfortable than running agents on my laptop."
- "It saved me time on real work."

## Roadmap

### v1

- 5-minute VPS setup
- one runtime
- CLI
- simple web UI
- persistent sessions
- run, stop, resume
- logs and test visibility
- SQLite state on local disk
- one workspace or worktree per session

### v2

- stronger isolation
- better git and PR support
- multiple concurrent sessions
- simple small-team sharing

### v3

- runtime choice
- optional chat entry points
- more polished collaboration

## Final Framing

TeleCoder is a simple remote coding session product for individuals and very
small teams.

The product is not "AI for everything."
The product is:

**a coding agent that lives on your VPS instead of on your laptop**
