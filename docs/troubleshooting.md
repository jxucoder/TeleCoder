# Troubleshooting

## "claude: command not found"

```bash
npm install -g @anthropic-ai/claude-code
which claude
```

If installed but not in PATH, set it explicitly:

```bash
# ~/.config/telecoder/config.sh
TELECODER_RUNTIME="/usr/local/bin/claude"
```

## Session starts but no output

Check stderr:

```bash
telecoder logs <id> --stream stderr
```

Common causes: missing API key, network issues, bad binary path.

## Session stuck in "running"

The tmux session may have exited. `telecoder inspect <id>` auto-detects this and updates the status.

Or check manually:

```bash
tmux ls | grep tc-<id>
```

## Git clone fails

Test git access directly:

```bash
git clone <your-repo-url> /tmp/test
```

See [Git Credentials Guide](git-credentials.md).

## tmux not found

```bash
sudo apt install -y tmux
```

## Logs location

```bash
# stdout
telecoder logs <id>

# stderr
telecoder logs <id> --stream stderr

# Raw files
ls ~/.telecoder/logs/
```

## Reset everything

```bash
rm -rf ~/.telecoder
telecoder init
```
