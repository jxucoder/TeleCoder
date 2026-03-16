#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${1:-telecoder}"
HOST="${2:-127.0.0.1}"
PORT="${3:-7080}"

systemctl status "${SERVICE_NAME}.service" --no-pager
echo
curl -sS "http://${HOST}:${PORT}/health"
echo
echo
journalctl -u "${SERVICE_NAME}.service" -n 40 --no-pager
