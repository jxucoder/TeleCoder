#!/usr/bin/env bash
#
# OpenTL Deploy Script
#
# Run on any fresh Ubuntu server (22.04+):
#   git clone https://github.com/jxucoder/opentl.git
#   cd opentl
#   cp .env.example .env   # edit with your tokens
#   ./deploy.sh
#
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

info()  { echo -e "${GREEN}[+]${NC} $1"; }
warn()  { echo -e "${YELLOW}[!]${NC} $1"; }
error() { echo -e "${RED}[x]${NC} $1"; exit 1; }

echo ""
echo "  ╔═══════════════════════════════════╗"
echo "  ║   OpenTL Deploy                   ║"
echo "  ║   Send a task, get a PR.          ║"
echo "  ╚═══════════════════════════════════╝"
echo ""

# --- 1. Check .env ---

if [ ! -f .env ]; then
    if [ -f .env.example ]; then
        error ".env file not found. Run:\n  cp .env.example .env\n  nano .env  # fill in your tokens\n  ./deploy.sh"
    else
        error ".env file not found. Create one with GITHUB_TOKEN, ANTHROPIC_API_KEY, and TELEGRAM_BOT_TOKEN."
    fi
fi

# Source .env to validate required vars.
set -a
source .env
set +a

if [ -z "${GITHUB_TOKEN:-}" ] || [ "$GITHUB_TOKEN" = "ghp_your_token_here" ]; then
    error "GITHUB_TOKEN is not set in .env"
fi

if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -z "${OPENAI_API_KEY:-}" ]; then
    error "At least one of ANTHROPIC_API_KEY or OPENAI_API_KEY must be set in .env"
fi

if [ -n "${TELEGRAM_BOT_TOKEN:-}" ] && [ "$TELEGRAM_BOT_TOKEN" != "123456789:ABCdefGhIjKlMnOpQrStUvWxYz" ]; then
    info "Telegram bot token found"
else
    warn "No TELEGRAM_BOT_TOKEN set -- Telegram bot will be disabled"
fi

# --- 2. Install Docker if needed ---

if ! command -v docker &> /dev/null; then
    info "Installing Docker..."
    curl -fsSL https://get.docker.com | sh
    sudo usermod -aG docker "$USER"
    info "Docker installed. You may need to log out and back in for group changes."
    # Use sudo for the rest of this script since the group change hasn't taken effect.
    DOCKER="sudo docker"
    COMPOSE="sudo docker compose"
else
    info "Docker already installed"
    DOCKER="docker"
    COMPOSE="docker compose"
fi

# Ensure Docker is running.
if ! $DOCKER info &> /dev/null; then
    info "Starting Docker..."
    sudo systemctl start docker
    sudo systemctl enable docker
fi

# --- 3. Build sandbox image ---

info "Building sandbox image (this takes a few minutes on first run)..."
$DOCKER build -f docker/base.Dockerfile -t opentl-sandbox .

# --- 4. Start with Docker Compose ---

info "Starting OpenTL..."
$COMPOSE -f docker/compose.yml up -d --build

# --- 5. Verify ---

echo ""
info "Waiting for server to start..."
sleep 3

if $DOCKER ps | grep -q opentl; then
    echo ""
    info "OpenTL is running!"
    echo ""
    echo "  Server:    http://$(hostname -I | awk '{print $1}'):7080"
    echo "  Health:    http://$(hostname -I | awk '{print $1}'):7080/health"
    echo ""
    if [ -n "${TELEGRAM_BOT_TOKEN:-}" ] && [ "$TELEGRAM_BOT_TOKEN" != "123456789:ABCdefGhIjKlMnOpQrStUvWxYz" ]; then
        echo "  Telegram:  Open your bot and send a message!"
        echo ""
    fi
    echo "  Logs:      $COMPOSE -f docker/compose.yml logs -f"
    echo "  Stop:      $COMPOSE -f docker/compose.yml down"
    echo "  Restart:   $COMPOSE -f docker/compose.yml restart"
    echo ""
else
    warn "Container may not have started. Check logs:"
    echo "  $COMPOSE -f docker/compose.yml logs"
fi
