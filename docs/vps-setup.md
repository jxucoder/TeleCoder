# VPS Setup Guide

## Recommended VPS

- **Provider**: Any (DigitalOcean, Hetzner, Linode, AWS Lightsail, etc.)
- **OS**: Ubuntu 22.04 or 24.04
- **RAM**: 2GB minimum, 4GB recommended
- **Storage**: 20GB+ (depends on repo sizes)

## Initial VPS Setup

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install dependencies (install.sh does this too)
sudo apt install -y git tmux sqlite3 curl
```

## Install Node.js (for Claude Code)

```bash
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt install -y nodejs
```

## Install Claude Code CLI

```bash
npm install -g @anthropic-ai/claude-code
```

Verify:

```bash
claude --version
```

## Install TeleCoder

```bash
cd /opt
git clone https://github.com/your-org/telecoder.git
cd telecoder
sudo ./install.sh
```

## Firewall

If you want remote access later:

```bash
sudo ufw allow 22/tcp    # SSH
```
