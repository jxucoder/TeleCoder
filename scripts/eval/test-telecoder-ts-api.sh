#!/usr/bin/env bash
set -euo pipefail

REPO="git@github.com-telecoder-test:jxucoder/telecoder-test-repo.git"
PROMPT="Reply exactly TELECODER_API_OK and do not modify any files."

BODY=$(printf '{"repo":"%s","agent":"codex","prompt":"%s"}' "$REPO" "$PROMPT")
SESSION_JSON=$(curl -sS -H "content-type: application/json" -d "$BODY" http://127.0.0.1:7080/api/sessions)
echo "$SESSION_JSON"

SESSION_ID=$(printf '%s' "$SESSION_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')

sleep 8
echo "SESSION:$SESSION_ID"
curl -sS "http://127.0.0.1:7080/api/sessions/$SESSION_ID"
echo
curl -sS "http://127.0.0.1:7080/api/sessions/$SESSION_ID/events"
