#!/usr/bin/env bash
# TestCenter macOS launcher: checks the runtime, starts backend then frontend,
# and prints LAN diagnostics. Run from any directory with: bash /path/to/start.sh
set -Eeuo pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_ENV_FILE="${PROJECT_ROOT}/.env.lan.local"
DB_CONFIG_FILE="${PROJECT_ROOT}/server/data/database_config.json"
BACKEND_BINARY="${TMPDIR:-/tmp}/testcenter-backend-${UID}"
BACKEND_PID=""
FRONTEND_PID=""
SLEEP_GUARD_PID=""

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Homebrew's Apple Silicon default path is not present in every non-login shell.
if [[ -d /opt/homebrew/bin ]]; then
  export PATH="/opt/homebrew/bin:${PATH}"
fi

info() { printf '%b\n' "${GREEN}$*${NC}"; }
warn() { printf '%b\n' "${YELLOW}$*${NC}"; }
fail() { printf '%b\n' "${RED}✗ $*${NC}" >&2; exit 1; }

http_status() {
  # Connection failures are expected while a child process is still booting.
  # Return its status silently; the caller retries and reports only final failures.
  curl --noproxy '*' -s -o /dev/null -w '%{http_code}' --max-time 5 "$1" 2>/dev/null || true
}

get_lan_ips() {
  local default_iface=""
  default_iface="$(route get default 2>/dev/null | awk '/interface:/{print $2; exit}')"
  {
    if [[ -n "$default_iface" ]]; then
      ipconfig getifaddr "$default_iface" 2>/dev/null || true
    fi
    ifconfig 2>/dev/null | awk '/inet / && $2 !~ /^127\./ && $2 !~ /^198\.18\./ {print $2}'
  } | awk 'NF && !seen[$0]++'
}

is_ipv4_host() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

ip_list_contains() {
  local needle="$1"
  local haystack="$2"
  while IFS= read -r item; do
    [[ "$item" == "$needle" ]] && return 0
  done <<< "$haystack"
  return 1
}

resolve_host_ips() {
  local host="$1"
  if command -v dscacheutil >/dev/null 2>&1; then
    dscacheutil -q host -a name "$host" 2>/dev/null | awk '/ip_address:/{print $2}' | awk '!seen[$0]++'
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing command: $1. ${2}"
  info "✓ ${1} is available."
}

pid_for_port() {
  lsof -nP -iTCP:"$1" -sTCP:LISTEN -t 2>/dev/null | head -n 1 || true
}

stop_previous_instance() {
  local port="$1"
  local expected_pattern="$2"
  local service_name="$3"
  local pid command_line attempt

  pid="$(pid_for_port "$port")"
  [[ -z "$pid" ]] && return 0

  command_line="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  if ! grep -qiE "$expected_pattern" <<< "$command_line"; then
    fail "Port ${port} is occupied by an unexpected process (PID ${pid}): ${command_line}. Stop it manually before starting TestCenter."
  fi

  warn "⚠ Stopping previous ${service_name} instance on port ${port} (PID ${pid})..."
  kill "$pid" 2>/dev/null || true
  for attempt in {1..10}; do
    kill -0 "$pid" 2>/dev/null || return 0
    sleep 1
  done
  warn "  Previous process did not exit in time; forcing it to stop."
  kill -KILL "$pid" 2>/dev/null || true
}

cleanup() {
  trap - EXIT INT TERM
  printf '\n'
  warn "Stopping TestCenter..."
  [[ -n "$FRONTEND_PID" ]] && kill "$FRONTEND_PID" 2>/dev/null || true
  [[ -n "$BACKEND_PID" ]] && kill "$BACKEND_PID" 2>/dev/null || true
  [[ -n "$SLEEP_GUARD_PID" ]] && kill "$SLEEP_GUARD_PID" 2>/dev/null || true
  [[ -n "$FRONTEND_PID" ]] && wait "$FRONTEND_PID" 2>/dev/null || true
  [[ -n "$BACKEND_PID" ]] && wait "$BACKEND_PID" 2>/dev/null || true
  [[ -n "$SLEEP_GUARD_PID" ]] && wait "$SLEEP_GUARD_PID" 2>/dev/null || true
  info "All services stopped."
}

start_sleep_guard() {
  if ! command -v caffeinate >/dev/null 2>&1; then
    warn "⚠ caffeinate is unavailable; macOS system sleep can pause scheduled tasks."
    return 0
  fi

  # Keep the Mac itself awake while this launcher is alive. The display may
  # still turn off normally. -i blocks idle sleep; -s also blocks system sleep
  # while connected to AC power.
  caffeinate -i -s -w "$$" >/dev/null 2>&1 &
  SLEEP_GUARD_PID=$!
  if kill -0 "$SLEEP_GUARD_PID" 2>/dev/null; then
    info "✓ macOS system-sleep guard is active; display sleep remains enabled."
  else
    fail "Unable to start the macOS system-sleep guard."
  fi
}

wait_for_http() {
  local url="$1"
  local pid="$2"
  local label="$3"
  local accepted_statuses="$4"
  local status=""
  local attempt

  for attempt in {1..60}; do
    kill -0 "$pid" 2>/dev/null || fail "${label} exited during startup. Check the terminal output above."
    status="$(http_status "$url")"
    if [[ " $accepted_statuses " == *" $status "* ]]; then
      info "✓ ${label} is ready (HTTP ${status})."
      return 0
    fi
    sleep 1
  done
  fail "${label} did not become ready at ${url}. Check the terminal output above."
}

print_lan_diagnostics() {
  local lan_host="$1"
  local lan_ips="$2"
  local frontend_status backend_status resolved_ips="" resolved_ip=""
  local domain_matches_lan=0

  frontend_status="$(http_status "http://${lan_host}:5173/dashboard")"
  backend_status="$(http_status "http://${lan_host}:8080/api/auth/me")"

  info "\n=========================================="
  info "  TestCenter is running"
  info "=========================================="
  info "  Local frontend: http://127.0.0.1:5173/dashboard"
  info "  Share This Address: http://${lan_host}:5173/dashboard"
  info "  Stable LAN Frontend: http://${lan_host}:5173/dashboard"
  info "  Stable LAN Backend:  http://${lan_host}:8080"
  info "  Runtime logs: current terminal"

  if [[ -n "$lan_ips" ]]; then
    info "  LAN IP fallback:"
    while IFS= read -r lan_ip; do
      [[ -n "$lan_ip" ]] && info "    http://${lan_ip}:5173/dashboard"
    done <<< "$lan_ips"
  fi

  if [[ "$frontend_status" == "200" ]]; then
    info "  ✓ Configured frontend address is reachable locally."
  else
    warn "  ⚠ Configured frontend address returned HTTP ${frontend_status:-000}."
  fi
  if [[ "$backend_status" == "200" || "$backend_status" == "401" ]]; then
    info "  ✓ Configured backend address is reachable locally (HTTP ${backend_status})."
  else
    warn "  ⚠ Configured backend address returned HTTP ${backend_status:-000}."
  fi

  if is_ipv4_host "$lan_host"; then
    if ! ip_list_contains "$lan_host" "$lan_ips"; then
      warn "  ⚠ ${lan_host} is no longer assigned to this Mac. Update ${RUNTIME_ENV_FILE} before sharing the address."
    fi
  else
    resolved_ips="$(resolve_host_ips "$lan_host")"
    if [[ -z "$resolved_ips" ]]; then
      warn "  ⚠ ${lan_host} cannot be resolved on this Mac. Check LAN DNS or hosts."
    else
      while IFS= read -r resolved_ip; do
        if ip_list_contains "$resolved_ip" "$lan_ips"; then
          domain_matches_lan=1
          break
        fi
      done <<< "$resolved_ips"
      if [[ "$domain_matches_lan" -eq 0 ]]; then
        warn "  ⚠ ${lan_host} resolves to a different device. Update LAN DNS/hosts to this Mac's IP."
      fi
    fi
  fi

  warn "  Other devices must be on the same LAN; firewall, VPN, guest-network and AP isolation can block access."
  info "  Press Ctrl+C to stop both services."
}

[[ "$(uname -s)" == "Darwin" ]] || fail "This launcher is for macOS."

info "=========================================="
info "   Starting TestCenter Environment Check"
info "=========================================="

require_command node "Install Node 22 with: brew install node@22"
require_command npm "Install Node with: brew install node@22"
require_command go "Install Go with: brew install go"
require_command curl "Install macOS Command Line Tools first."
require_command lsof "Install macOS Command Line Tools first."
require_command mysql "Install MySQL with: brew install mysql"
if command -v k6 >/dev/null 2>&1; then
  info "✓ k6 is available."
else
  warn "⚠ k6 is missing; the platform can start, but k6 execution features will fail."
fi

[[ -f "$RUNTIME_ENV_FILE" ]] || warn "⚠ ${RUNTIME_ENV_FILE} is missing; the current LAN IP will be used for display only."
if [[ -f "$RUNTIME_ENV_FILE" ]]; then
  info "✓ Loading LAN runtime config from .env.lan.local."
  set -a
  # shellcheck disable=SC1090
  source "$RUNTIME_ENV_FILE"
  set +a
fi

[[ -f "$DB_CONFIG_FILE" ]] || fail "Database config is missing. Run: MIGRATION_CONFIRM=YES bash scripts/import_database_macos.sh"
if ! lsof -nP -iTCP:3306 -sTCP:LISTEN >/dev/null 2>&1; then
  fail "MySQL is not listening on port 3306. Start it with: brew services start mysql"
fi

cd "$PROJECT_ROOT"

if [[ ! -d node_modules ]]; then
  warn "Installing locked frontend dependencies..."
  npm ci
fi

info "Downloading Go module dependencies..."
(cd server && go mod download)

info "Building Go backend..."
(cd server && go build -o "$BACKEND_BINARY" .)

info "Checking port availability..."
stop_previous_instance 8080 'testcenter|go-build|/server' 'Go backend'
stop_previous_instance 5173 'vite|node|esbuild' 'Vue frontend'

trap cleanup EXIT INT TERM
start_sleep_guard
export TESTCENTER_BACKEND_HOST="0.0.0.0"
export TESTCENTER_BACKEND_PORT="8080"
export TESTCENTER_DB_CONFIG_FILE="$DB_CONFIG_FILE"

info "Starting Go backend on 0.0.0.0:8080..."
(cd "$PROJECT_ROOT/server" && exec "$BACKEND_BINARY") &
BACKEND_PID=$!
wait_for_http "http://127.0.0.1:8080/api/auth/me" "$BACKEND_PID" "Backend" "200 401"

info "Starting Vue frontend on 0.0.0.0:5173..."
npm run dev &
FRONTEND_PID=$!
wait_for_http "http://127.0.0.1:5173/dashboard" "$FRONTEND_PID" "Frontend" "200"

LAN_IPS="$(get_lan_ips)"
LAN_HOST="${TESTCENTER_LAN_HOST:-}"
if [[ -z "$LAN_HOST" ]]; then
  LAN_HOST="$(awk 'NF {print; exit}' <<< "$LAN_IPS")"
fi
[[ -n "$LAN_HOST" ]] || LAN_HOST="127.0.0.1"

print_lan_diagnostics "$LAN_HOST" "$LAN_IPS"

while kill -0 "$BACKEND_PID" 2>/dev/null && kill -0 "$FRONTEND_PID" 2>/dev/null; do
  sleep 2
done

if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
  fail "Backend stopped unexpectedly. Review the terminal output above."
fi
fail "Frontend stopped unexpectedly. Review the terminal output above."
