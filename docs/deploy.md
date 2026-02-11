# Deploy OpenTL

Get a VPS, run one script, message your bot from your phone.

## Quick Start

### 1. Get a Server

Any VPS with 4GB+ RAM running Ubuntu 22.04+. Cheapest options:

| Provider | Spec | Price | Link |
|---|---|---|---|
| Hetzner | 2 vCPU, 4GB RAM | ~$4.5/mo | [hetzner.com/cloud](https://www.hetzner.com/cloud) |
| DigitalOcean | 1 vCPU, 2GB RAM | $12/mo | [digitalocean.com](https://www.digitalocean.com) |
| Vultr | 1 vCPU, 2GB RAM | $10/mo | [vultr.com](https://www.vultr.com) |

When creating the server, choose **Ubuntu 24.04**. Add your SSH key.

### 2. SSH In and Deploy

```bash
ssh root@YOUR_SERVER_IP

# Clone the repo
git clone https://github.com/jxucoder/opentl.git
cd opentl

# Set up your tokens
cp .env.example .env
nano .env   # fill in GITHUB_TOKEN, ANTHROPIC_API_KEY, TELEGRAM_BOT_TOKEN

# Deploy (installs Docker, builds images, starts everything)
./deploy.sh
```

That's it. The script handles Docker installation, image building, and startup.

### 3. Message Your Bot

Open Telegram on your phone and send your bot a message:

```
fix the broken login page --repo your-org/your-repo
```

## What You Need Before Deploying

Gather these tokens before you start (takes ~5 minutes total):

1. **GitHub Token** -- [github.com/settings/tokens](https://github.com/settings/tokens) (select `repo` scope)
2. **Anthropic API Key** -- [console.anthropic.com](https://console.anthropic.com/settings/keys)
3. **Telegram Bot Token** -- message [@BotFather](https://t.me/BotFather) on Telegram, send `/newbot`, copy the token

## Managing the Server

```bash
# View logs
docker compose -f docker/compose.yml logs -f

# Restart
docker compose -f docker/compose.yml restart

# Stop
docker compose -f docker/compose.yml down

# Update (pull latest code and redeploy)
git pull && ./deploy.sh
```

## How Much RAM Do I Need?

Each coding task spins up a Docker container. Rough sizing:

| Concurrent Tasks | RAM Needed |
|---|---|
| 1 at a time | 4GB |
| 2-3 at a time | 8GB |
| 4+ at a time | 16GB |

For personal use (one task at a time from Telegram), a 4GB server is fine.

## Security Notes

- The server listens on port 7080 (HTTP API). You can firewall this if you don't need the web UI remotely.
- The Telegram bot uses **outbound** long-polling -- no inbound ports needed for it.
- Your `.env` contains secrets -- don't commit it to git (it's already in `.gitignore`).
