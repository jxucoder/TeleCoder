# Runtime Setup: Claude Code

TeleCoder v1 uses Claude Code as its coding agent runtime.

## Install Claude Code

```bash
npm install -g @anthropic-ai/claude-code
```

## Set API Key

Export your Anthropic API key:

```bash
export ANTHROPIC_API_KEY="sk-ant-your-key-here"
```

Add it to your shell profile so it persists:

```bash
echo 'export ANTHROPIC_API_KEY="sk-ant-your-key-here"' >> ~/.bashrc
```

## Verify Runtime

```bash
claude --print "say hello"
```

You should see a response from Claude.

## Runtime Configuration

If `claude` is not in PATH, set it in your config:

```bash
# ~/.config/telecoder/config.sh
TELECODER_RUNTIME="/usr/local/bin/claude"
```

## How TeleCoder Uses Claude Code

When you run a session, TeleCoder:

1. Launches `claude -p '<prompt>'` inside a tmux session
2. Captures stdout and stderr to log files
3. The tmux session survives terminal disconnects
4. You can attach to it live with `telecoder attach <id>`
5. Or stop it with `telecoder stop <id>`
