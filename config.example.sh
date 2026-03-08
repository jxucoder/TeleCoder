# TeleCoder configuration
# Source this file — it's just shell variables.

# Where TeleCoder stores sessions, logs, and the database
TELECODER_DATA="${TELECODER_DATA:-$HOME/.telecoder}"

# Runtime binary (claude CLI)
TELECODER_RUNTIME="${TELECODER_RUNTIME:-claude}"

# Git branch prefix for session branches
TELECODER_BRANCH_PREFIX="${TELECODER_BRANCH_PREFIX:-telecoder/}"

# Auto-push after session completes
TELECODER_AUTO_PUSH="${TELECODER_AUTO_PUSH:-false}"

# Web UI host/port (future)
TELECODER_HOST="${TELECODER_HOST:-127.0.0.1}"
TELECODER_PORT="${TELECODER_PORT:-7830}"
