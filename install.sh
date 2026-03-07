#!/usr/bin/env bash
set -euo pipefail

# TeleCoder Installer
# Usage: curl -fsSL https://your-domain/install.sh | bash
#   or:  git clone ... && cd telecoder && sudo ./install.sh

INSTALL_DIR="/usr/local/share/telecoder"
BIN_LINK="/usr/local/bin/telecoder"
TELECODER_CONFIG="/etc/telecoder"

echo "=== TeleCoder Installer ==="
echo ""

# Check for root
if [ "$(id -u)" -ne 0 ]; then
    echo "Please run as root (or with sudo)."
    exit 1
fi

# Check dependencies
for dep in bash git sqlite3 tmux; do
    if ! command -v "$dep" &>/dev/null; then
        echo "$dep not found. Installing..."
        apt-get update -qq && apt-get install -y -qq "$dep"
    fi
done

echo "Dependencies OK."

# Copy telecoder to install dir
echo "Installing to ${INSTALL_DIR}..."
mkdir -p "$INSTALL_DIR"
cp -r bin/ lib/ config.example.sh "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/bin/telecoder" "$INSTALL_DIR"/lib/*.sh

# Symlink to PATH
ln -sf "$INSTALL_DIR/bin/telecoder" "$BIN_LINK"

# Copy config if not present
mkdir -p "$TELECODER_CONFIG"
if [ ! -f "$TELECODER_CONFIG/config.sh" ]; then
    cp config.example.sh "$TELECODER_CONFIG/config.sh"
    # Point data dir to /var/lib for system install
    sed -i 's|\$HOME/.telecoder|/var/lib/telecoder|' "$TELECODER_CONFIG/config.sh"
    echo "Config written to $TELECODER_CONFIG/config.sh"
fi

# Create data dirs
source "$TELECODER_CONFIG/config.sh"
mkdir -p "${TELECODER_DATA}"/{workspaces,logs}

echo ""
echo "=== TeleCoder installed ==="
echo ""
echo "Next steps:"
echo "  1. Edit $TELECODER_CONFIG/config.sh"
echo "     - Set TELECODER_RUNTIME if claude is not in PATH"
echo "  2. Initialize:"
echo "     telecoder init"
echo "  3. Create your first session:"
echo "     telecoder create --repo-url https://github.com/you/repo"
echo "     telecoder run <session-id> 'fix the failing tests'"
echo ""
echo "Sessions run in tmux. Close your terminal — they keep going."
echo "Come back with: telecoder attach <id>"
echo ""
