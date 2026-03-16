#!/usr/bin/env bash
set -euo pipefail

set -a
. /root/.config/telecoder/runtime.env
set +a

export TELECODER_DATA_DIR="/root/.telecoder-ts-eval-api"
export TELECODER_ACPX_COMMAND="/root/bin/acpx-local"
export TELECODER_DEFAULT_AGENT="codex"

pkill -f "telecoder-ts-eval/src/cli.ts serve" >/dev/null 2>&1 || true

nohup /root/.bun/bin/bun /root/telecoder-ts-eval/src/cli.ts serve \
  >/root/telecoder-ts-eval.log 2>&1 </dev/null &

sleep 2
curl -sS http://127.0.0.1:7080/health
echo
sed -n '1,20p' /root/telecoder-ts-eval.log

