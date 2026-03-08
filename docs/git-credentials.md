# Git Credential Guide

TeleCoder needs git access to clone and optionally push to your repos.

## SSH Keys (Recommended)

```bash
ssh-keygen -t ed25519 -C "telecoder@your-vps"
```

Add the public key to your GitHub/GitLab account as a deploy key:

```bash
cat ~/.ssh/id_ed25519.pub
```

## HTTPS with Token

```bash
git config --global credential.helper store
```

Then create `~/.git-credentials`:

```
https://your-token@github.com
```

## Testing Access

```bash
git clone https://github.com/you/your-repo /tmp/test-clone
rm -rf /tmp/test-clone
```

## Auto-Push

To auto-push branches, set in config:

```bash
# ~/.config/telecoder/config.sh
TELECODER_AUTO_PUSH=true
TELECODER_BRANCH_PREFIX="telecoder/"
```
