#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${PROJECT_ROOT}/logs"
BACKEND_LOG="${LOG_DIR}/backend-macos.log"
FRONTEND_LOG="${LOG_DIR}/frontend-macos.log"
BACKEND_PID=""
FRONTEND_PID=""

fail() {
  printf 'ERROR: %s\n' "$1" >&2
  exit 1
}

cleanup() {
  trap - EXIT INT TERM
  [[ -n "$FRONTEND_PID" ]] && kill "$FRONTEND_PID" 2>/dev/null || true
  [[ -n "$BACKEND_PID" ]] && kill "$BACKEND_PID" 2>/dev/null || true
  wait "$FRONTEND_PID" 2>/dev/null || true
  wait "$BACKEND_PID" 2>/dev/null || true
}

wait_for_http() {
  local url="$1"
  local process_id="$2"
  local label="$3"
  local attempts=60
  local status=""
  while (( attempts > 0 )); do
    kill -0 "$process_id" 2>/dev/null || fail "$label exited during startup. Check the log file."
    status="$(curl --noproxy '*' -s -o /dev/null -w '%{http_code}' --max-time 2 "$url" || true)"
    if [[ "$status" == "200" || "$status" == "401" ]]; then
      return 0
    fi
    attempts=$((attempts - 1))
    sleep 1
  done
  fail "$label did not become ready: $url"
}

check_port() {
  local port="$1"
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    fail "Port $port is already in use. Stop the existing service first."
  fi
}

[[ "$(uname -s)" == "Darwin" ]] || fail "This script must run on macOS."
for command_name in node npm go curl lsof; do
  command -v "$command_name" >/dev/null 2>&1 || fail "Missing command: $command_name"
done

cd "$PROJECT_ROOT"
mkdir -p "$LOG_DIR"

if [[ -f "${PROJECT_ROOT}/.env.lan.local" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${PROJECT_ROOT}/.env.lan.local"
  set +a
fi

export TESTCENTER_BACKEND_HOST="0.0.0.0"
export TESTCENTER_BACKEND_PORT="8080"
export TESTCENTER_DB_CONFIG_FILE="${PROJECT_ROOT}/server/data/database_config.json"

[[ -f "$TESTCENTER_DB_CONFIG_FILE" ]] || fail "Database config is missing. Run scripts/import_database_macos.sh first."

if [[ ! -d "${PROJECT_ROOT}/node_modules" ]]; then
  printf 'Installing frontend dependencies...\n'
  npm ci
fi

printf 'Downloading Go dependencies...\n'
(cd "${PROJECT_ROOT}/server" && go mod download)

check_port 8080
check_port 5173
trap cleanup EXIT INT TERM

printf 'Starting backend first...\n'
(cd "${PROJECT_ROOT}/server" && exec go run .) >"$BACKEND_LOG" 2>&1 &
BACKEND_PID=$!
wait_for_http "http://127.0.0.1:8080/api/auth/me" "$BACKEND_PID" "Backend"
printf 'Backend is ready.\n'

printf 'Starting frontend...\n'
npm run dev >"$FRONTEND_LOG" 2>&1 &
FRONTEND_PID=$!
wait_for_http "http://127.0.0.1:5173/dashboard" "$FRONTEND_PID" "Frontend"

LAN_IP="$(route get default 2>/dev/null | awk '/interface:/{print $2; exit}' | xargs -I{} ipconfig getifaddr {} 2>/dev/null || true)"
printf '\nTestCenter is running.\n'
printf 'Local:   http://127.0.0.1:5173/dashboard\n'
[[ -n "$LAN_IP" ]] && printf 'LAN:     http://%s:5173/dashboard\n' "$LAN_IP"
printf 'Backend: http://127.0.0.1:8080\n'
printf 'Logs:    %s\n' "$LOG_DIR"
printf 'Press Ctrl+C to stop both services.\n\n'

while kill -0 "$BACKEND_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
  sleep 2
done

if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
  printf 'Backend stopped unexpectedly. Last log lines:\n' >&2
  tail -n 40 "$BACKEND_LOG" >&2 || true
else
  printf 'Frontend stopped unexpectedly. Last log lines:\n' >&2
  tail -n 40 "$FRONTEND_LOG" >&2 || true
fi
exit 1
