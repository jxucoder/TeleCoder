# Quick Start

Get TeleCoder running on your VPS in under 5 minutes.

## Prerequisites

- Ubuntu 22.04+ VPS (2GB+ RAM recommended)
- Git, tmux, sqlite3 (installed automatically by install script)
- Claude Code CLI (`npm install -g @anthropic-ai/claude-code`)
- An Anthropic API key (set as `ANTHROPIC_API_KEY` env var)

## Install

```bash
git clone https://github.com/you/telecoder
cd telecoder
sudo ./install.sh
```

## Configure

Edit `~/.config/telecoder/config.sh` (user install) or `/etc/telecoder/config.sh` (system install):

```bash
TELECODER_RUNTIME="claude"   # path to claude CLI
```

Make sure your `ANTHROPIC_API_KEY` is set in your shell environment.

## Initialize

```bash
telecoder init
```

## Create and run a session

```bash
# Create a session from a repo
telecoder create --repo-url https://github.com/you/your-repo

# Run a task
telecoder run <session-id> fix the failing tests in src/auth.py

# Check status
telecoder list

# View output
telecoder logs <session-id>

# See full details
telecoder inspect <session-id>
```

## Close your laptop

The session runs in tmux on the VPS. Come back later:

```bash
# Check results
telecoder inspect <session-id>

# Or attach to the live tmux session
telecoder attach <session-id>
```

## Stop or resume

```bash
telecoder stop <session-id>
telecoder run <session-id> now run the linter and fix warnings
```
