#!/usr/bin/env bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

usage() {
  cat <<'EOF'
Usage: install-ubuntu-vps.sh [options]

Install or upgrade the TeleCoder Bun/acpx stack on Ubuntu and write a systemd
service plus env file.

Options:
  --app-dir <path>        TeleCoder checkout to serve (default: current dir)
  --service-name <name>   systemd service name (default: telecoder)
  --env-file <path>       environment file path (default: /etc/telecoder/telecoder.env)
  --data-dir <path>       TeleCoder data dir (default: /var/lib/telecoder)
  --dry-run               Print actions without mutating the host
  --no-start              Install files but do not enable/start the service
  --help                  Show this help

Config env:
  BUN_VERSION             Bun version to install (default: 1.3.10)
  NODE_MAJOR              Node major to install (default: 22)
  ACPX_VERSION            acpx npm version to install (default: 0.3.0)
  TELECODER_LISTEN_HOST   Host written to env file (default: 127.0.0.1)
  TELECODER_LISTEN_PORT   Port written to env file (default: 7080)
EOF
}

die() {
  echo "error: $*" >&2
  exit 1
}

run() {
  if [[ "${DRY_RUN}" == "1" ]]; then
    printf '+'
    for arg in "$@"; do
      printf ' %q' "${arg}"
    done
    printf '\n'
    return 0
  fi

  "$@"
}

run_with_env() {
  if [[ "${DRY_RUN}" == "1" ]]; then
    printf '+ env'
    for arg in "$@"; do
      printf ' %q' "${arg}"
    done
    printf '\n'
    return 0
  fi

  env "$@"
}

write_file() {
  local path="$1"
  local content="$2"

  if [[ "${DRY_RUN}" == "1" ]]; then
    printf '>>> %s\n%s\n' "${path}" "${content}"
    return 0
  fi

  mkdir -p "$(dirname "${path}")"
  printf '%s\n' "${content}" >"${path}"
}

write_file_if_missing() {
  local path="$1"
  local content="$2"

  if [[ -e "${path}" ]]; then
    echo "info: leaving existing file in place: ${path}"
    return 0
  fi

  write_file "${path}" "${content}"
}

require_command() {
  local command_name="$1"
  command -v "${command_name}" >/dev/null 2>&1 || die "required command not found: ${command_name}"
}

version_at_path() {
  local path="$1"
  local command_name="$2"

  if [[ ! -x "${path}" ]]; then
    return 1
  fi

  "${path}" --version 2>/dev/null | awk 'NR==1{print $NF}'
}

DRY_RUN=0
START_SERVICE=1
APP_DIR=""
SERVICE_NAME="telecoder"
ENV_FILE="/etc/telecoder/telecoder.env"
DATA_DIR="/var/lib/telecoder"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app-dir)
      [[ $# -ge 2 ]] || die "--app-dir requires a value"
      APP_DIR="$2"
      shift 2
      ;;
    --service-name)
      [[ $# -ge 2 ]] || die "--service-name requires a value"
      SERVICE_NAME="$2"
      shift 2
      ;;
    --env-file)
      [[ $# -ge 2 ]] || die "--env-file requires a value"
      ENV_FILE="$2"
      shift 2
      ;;
    --data-dir)
      [[ $# -ge 2 ]] || die "--data-dir requires a value"
      DATA_DIR="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --no-start)
      START_SERVICE=0
      shift
      ;;
    --help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1"
      ;;
  esac
done

[[ "${EUID}" -eq 0 ]] || die "run this script as root"
require_command awk
require_command curl
require_command git
require_command systemctl

APP_DIR="${APP_DIR:-$(pwd)}"
APP_DIR="$(cd "${APP_DIR}" && pwd)"
[[ -f "${APP_DIR}/package.json" ]] || die "missing package.json in app dir: ${APP_DIR}"
[[ -f "${APP_DIR}/src/cli.ts" ]] || die "missing src/cli.ts in app dir: ${APP_DIR}"

source /etc/os-release
[[ "${ID:-}" == "ubuntu" ]] || die "Ubuntu is required; found ID=${ID:-unknown}"

BUN_VERSION="${BUN_VERSION:-1.3.10}"
NODE_MAJOR="${NODE_MAJOR:-22}"
ACPX_VERSION="${ACPX_VERSION:-0.3.0}"
BUN_INSTALL_ROOT="${BUN_INSTALL_ROOT:-/opt/bun}"
BUN_BIN="${BUN_INSTALL_ROOT}/bin/bun"
SYSTEMD_UNIT_PATH="/etc/systemd/system/${SERVICE_NAME}.service"
ENV_EXAMPLE_PATH="${ENV_FILE}.example"
LISTEN_HOST="${TELECODER_LISTEN_HOST:-127.0.0.1}"
LISTEN_PORT="${TELECODER_LISTEN_PORT:-7080}"

echo "info: app dir=${APP_DIR}"
echo "info: service=${SERVICE_NAME}"
echo "info: env file=${ENV_FILE}"
echo "info: data dir=${DATA_DIR}"
echo "info: bun=${BUN_VERSION} node=${NODE_MAJOR} acpx=${ACPX_VERSION}"

run apt-get update
run apt-get install -y ca-certificates curl git jq unzip

node_major=""
if command -v node >/dev/null 2>&1; then
  node_major="$(node -p "process.versions.node.split('.')[0]")"
fi

if [[ "${node_major}" != "${NODE_MAJOR}" ]]; then
  run curl -fsSL -o /tmp/telecoder-nodesource.sh "https://deb.nodesource.com/setup_${NODE_MAJOR}.x"
  run bash /tmp/telecoder-nodesource.sh
  run apt-get install -y nodejs
fi

installed_bun_version="$(version_at_path "${BUN_BIN}" bun || true)"
if [[ "${installed_bun_version}" != "${BUN_VERSION}" ]]; then
  run mkdir -p "${BUN_INSTALL_ROOT}"
  run curl -fsSL -o /tmp/telecoder-bun-install.sh https://bun.sh/install
  run_with_env BUN_INSTALL="${BUN_INSTALL_ROOT}" bash /tmp/telecoder-bun-install.sh "bun-v${BUN_VERSION}"
fi

run ln -sf "${BUN_BIN}" /usr/local/bin/bun
run npm install -g "acpx@${ACPX_VERSION}"

run mkdir -p /etc/telecoder
run mkdir -p "${DATA_DIR}"

ENV_CONTENT="$(cat <<EOF
# TeleCoder host config
PATH=${BUN_INSTALL_ROOT}/bin:/usr/local/bin:/usr/bin:/bin
TELECODER_DATA_DIR=${DATA_DIR}
TELECODER_LISTEN_HOST=${LISTEN_HOST}
TELECODER_LISTEN_PORT=${LISTEN_PORT}
TELECODER_ACPX_COMMAND=acpx
TELECODER_DEFAULT_AGENT=codex
TELECODER_POLICY_MODE=observe
TELECODER_PROMPT_TIMEOUT_SECONDS=300
TELECODER_SESSION_HEARTBEAT_SECONDS=5
TELECODER_SESSION_STALE_SECONDS=30

# Runtime auth examples
# OPENAI_API_KEY=
# ANTHROPIC_API_KEY=
# OPENROUTER_API_KEY=
# TELECODER_AGENT_CLAUDE_COMMAND=
# TELECODER_AGENT_OPENCODE_COMMAND=
EOF
)"

UNIT_CONTENT="$(cat <<EOF
[Unit]
Description=TeleCoder Bun service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${APP_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${BUN_BIN} src/cli.ts serve
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
)"

write_file "${ENV_EXAMPLE_PATH}" "${ENV_CONTENT}"
write_file_if_missing "${ENV_FILE}" "${ENV_CONTENT}"
write_file "${SYSTEMD_UNIT_PATH}" "${UNIT_CONTENT}"

run systemctl daemon-reload

if [[ "${START_SERVICE}" == "1" ]]; then
  run systemctl enable --now "${SERVICE_NAME}.service"
else
  echo "info: service install complete; skipped enable/start"
fi

echo "info: finished"
echo "info: env example=${ENV_EXAMPLE_PATH}"
echo "info: systemd unit=${SYSTEMD_UNIT_PATH}"
if [[ "${START_SERVICE}" == "1" ]]; then
  echo "info: health check: curl -sS http://${LISTEN_HOST}:${LISTEN_PORT}/health"
  echo "info: logs: journalctl -u ${SERVICE_NAME}.service -n 100 --no-pager"
fi
