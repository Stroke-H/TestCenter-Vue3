#!/bin/bash

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color
RUNTIME_ENV_FILE=".env.lan.local"

echo -e "${GREEN}==========================================${NC}"
echo -e "${GREEN}   Starting TestCenter Environment Check  ${NC}"
echo -e "${GREEN}==========================================${NC}"

# 1. Dependency Checks
echo -e "Checking requirements..."

command -v node >/dev/null 2>&1 || { echo -e "${RED}✗ Error: Node.js (npm) is not installed.${NC} Please install Node.js."; exit 1; }
echo -e "${GREEN}✓ Node.js is installed.${NC}"

command -v go >/dev/null 2>&1 || { echo -e "${RED}✗ Error: Go is not installed.${NC} Please install Go 1.25+."; exit 1; }
echo -e "${GREEN}✓ Go is installed.${NC}"

command -v k6 >/dev/null 2>&1 || { echo -e "${YELLOW}⚠ Warning: k6 is not installed globally.${NC} (The platform will start, but k6 execution features will fail. Run 'brew install k6' if you need it.)"; }
if command -v k6 >/dev/null 2>&1; then
    echo -e "${GREEN}✓ k6 is installed.${NC}"
fi

load_runtime_env() {
    if [ -f "$RUNTIME_ENV_FILE" ]; then
        echo -e "${GREEN}✓ Loading LAN runtime config from ${RUNTIME_ENV_FILE}.${NC}"
        set -a
        . "./${RUNTIME_ENV_FILE}"
        set +a
    else
        echo -e "${YELLOW}⚠ No ${RUNTIME_ENV_FILE} found. Falling back to Bonjour/mDNS host discovery.${NC}"
    fi
}

is_mdns_host() {
    case "$1" in
        *.local) return 0 ;;
        *) return 1 ;;
    esac
}

is_ipv4_host() {
    case "$1" in
        *[!0-9.]* | "" | *..* | .* | *.) return 1 ;;
        *.*.*.*) return 0 ;;
        *) return 1 ;;
    esac
}

load_runtime_env

get_lan_ips() {
    local DEFAULT_IFACE
    DEFAULT_IFACE=$(route get default 2>/dev/null | awk '/interface:/{print $2; exit}')
    {
        if [ -n "$DEFAULT_IFACE" ]; then
            local DEFAULT_IP
            DEFAULT_IP=$(ipconfig getifaddr "$DEFAULT_IFACE" 2>/dev/null)
            if [ -n "$DEFAULT_IP" ]; then
                echo "$DEFAULT_IP"
            fi
        fi

        ifconfig 2>/dev/null | awk '/inet / && $2 !~ /^127\./ && $2 !~ /^198\.18\./ {print $2}'
    } | awk '!seen[$0]++'
}

get_mdns_host() {
    if [ -n "$TESTCENTER_LAN_HOST" ]; then
        echo "$TESTCENTER_LAN_HOST"
        return 0
    fi

    echo "www.inspdance.com"
}

http_status() {
    local URL=$1
    curl --noproxy "*" -s -o /dev/null -w "%{http_code}" --max-time 5 "$URL" || true
}

resolve_host_ips() {
    local HOST=$1
    if command -v dscacheutil >/dev/null 2>&1; then
        dscacheutil -q host -a name "$HOST" 2>/dev/null | awk '/ip_address:/{print $2}' | awk '!seen[$0]++'
        return 0
    fi

    if command -v getent >/dev/null 2>&1; then
        getent hosts "$HOST" 2>/dev/null | awk '{print $1}' | awk '!seen[$0]++'
    fi
}

ip_list_contains() {
    local NEEDLE=$1
    local HAYSTACK=$2

    while IFS= read -r ITEM; do
        if [ "$ITEM" = "$NEEDLE" ]; then
            return 0
        fi
    done <<< "$HAYSTACK"

    return 1
}

check_stable_host_resolution() {
    local LAN_HOST=$1
    local LAN_IPS=$2

    if is_mdns_host "$LAN_HOST"; then
        return 0
    fi

    if is_ipv4_host "$LAN_HOST"; then
        echo -e "${YELLOW}  IP Fallback Self-check:${NC}"
        if ip_list_contains "$LAN_HOST" "$LAN_IPS"; then
            echo -e "${YELLOW}    ⚠ ${LAN_HOST} belongs to this Mac, but IP literals are not a stable user-facing entry.${NC}"
            echo -e "${YELLOW}      Prefer a fixed hostname such as www.inspdance.com or testcenter.lan for shared access.${NC}"
        else
            echo -e "${YELLOW}    ⚠ IP fallback ${LAN_HOST} is not currently found on this Mac.${NC}"
            echo -e "${YELLOW}      Check whether the wired network changed IP, then update ${RUNTIME_ENV_FILE} if needed.${NC}"
        fi
        return 0
    fi

    echo -e "${GREEN}  Stable DNS Self-check:${NC}"
    local RESOLVED_IPS
    RESOLVED_IPS=$(resolve_host_ips "$LAN_HOST")

    if [ -z "$RESOLVED_IPS" ]; then
        echo -e "${YELLOW}    ⚠ ${LAN_HOST} is not resolvable on this Mac. Add/update your router DNS, local DNS, or hosts rule.${NC}"
        return 0
    fi

    echo -e "${GREEN}    Resolved ${LAN_HOST} to:${NC}"
    while IFS= read -r RESOLVED_IP; do
        if [ -n "$RESOLVED_IP" ]; then
            echo -e "${GREEN}      ${RESOLVED_IP}${NC}"
        fi
    done <<< "$RESOLVED_IPS"

    while IFS= read -r RESOLVED_IP; do
        if [ -n "$RESOLVED_IP" ] && ip_list_contains "$RESOLVED_IP" "$LAN_IPS"; then
            echo -e "${GREEN}    ✓ ${LAN_HOST} points to this Mac's LAN IP.${NC}"
            return 0
        fi
    done <<< "$RESOLVED_IPS"

    echo -e "${YELLOW}    ⚠ ${LAN_HOST} does not currently resolve to this Mac's LAN IP.${NC}"
    echo -e "${YELLOW}      Keep the user-facing domain unchanged, but update its DNS/hosts mapping to one of these LAN IPs:${NC}"
    while IFS= read -r LAN_IP; do
        if [ -n "$LAN_IP" ]; then
            echo -e "${YELLOW}      ${LAN_IP}${NC}"
        fi
    done <<< "$LAN_IPS"
}

print_remote_access_help() {
    local LAN_HOST=$1
    local LAN_IPS=$2

    echo -e "${YELLOW}  Remote Device Troubleshooting:${NC}"
    echo -e "${YELLOW}    1. 先在另一台设备打开 Stable LAN Frontend。${NC}"
    if is_mdns_host "$LAN_HOST"; then
        echo -e "${YELLOW}    2. 如果 ${LAN_HOST} 打不开，但 LAN IP Fallback 能打开，说明是该设备的 .local / Bonjour 解析问题。${NC}"
        echo -e "${YELLOW}    3. 如果 hostname 和 IP fallback 都打不开，通常是没在同一网段、开了访客网络隔离、VPN 接管，或 AP/client isolation 阻止了设备互访。${NC}"
        echo -e "${YELLOW}    4. Windows 设备如果只在 .local 上失败，通常需要 Bonjour 支持；临时可直接使用 LAN IP Fallback。${NC}"
    elif is_ipv4_host "$LAN_HOST"; then
        echo -e "${YELLOW}    2. 当前使用固定有线 IP：${LAN_HOST}。如果别的设备打不开，先确认对方和这台 Mac 在同一办公网络内。${NC}"
        echo -e "${YELLOW}    3. 如果固定 IP 偶尔失效，通常是有线网口重新拿到了新 IP；重新启动脚本会在 Fixed IP Self-check 中提示。${NC}"
        echo -e "${YELLOW}    4. 如果 IP 一直不通，优先检查访客网络隔离、VPN 接管、防火墙或 AP/client isolation。${NC}"
    else
        echo -e "${YELLOW}    2. 如果 ${LAN_HOST} 打不开，但 LAN IP Fallback 能打开，优先检查该设备的 DNS 是否正确，或是否还在命中旧缓存。${NC}"
        echo -e "${YELLOW}    3. 如果 hostname 和 IP fallback 都打不开，通常是没在同一网段、开了访客网络隔离、VPN 接管，或 AP/client isolation 阻止了设备互访。${NC}"
        echo -e "${YELLOW}    4. 使用固定域名时，关键前提是所有客户端都能把 ${LAN_HOST} 解析到当前局域网 IP，而不是依赖 Bonjour 广播。${NC}"
    fi
    if [ -n "$LAN_IPS" ]; then
        local PRIMARY_IP
        PRIMARY_IP=$(printf "%s\n" "$LAN_IPS" | awk 'NF {print; exit}')
        if [ -n "$PRIMARY_IP" ]; then
            echo -e "${YELLOW}    5. 最直接的排查法：让对方先试 http://${PRIMARY_IP}:5173/dashboard${NC}"
        fi
    fi
    if is_mdns_host "$LAN_HOST"; then
        echo -e "${YELLOW}    6. 如果你要求所有设备都稳定使用 ${LAN_HOST}，那访问设备必须支持 Bonjour/mDNS，且网络必须允许 UDP 5353 组播在同一广播域内传播。${NC}"
        echo -e "${YELLOW}    7. 若业务要求是“无论设备类型如何都必须稳定访问”，不要只依赖 .local；应改为真实 DNS 域名或局域网 DNS 方案。${NC}"
    elif is_ipv4_host "$LAN_HOST"; then
        echo -e "${YELLOW}    6. 当前已切到固定 IP 模式；推荐只把这个有线 IP 分享给同网段使用者。${NC}"
    else
        echo -e "${YELLOW}    6. 当前已切到固定域名模式；如果个别设备仍不稳定，优先检查 ${LAN_HOST} 在该设备上的解析结果是否命中当前局域网 IP。${NC}"
    fi
}

# 2. Setup Node Modules if missing
if [ ! -d "node_modules" ]; then
    echo -e "\n${YELLOW}Setting up frontend dependencies...${NC}"
    npm install
fi

# 3. Setup Go Modules if missing
if [ ! -f "server/go.sum" ]; then
    echo -e "\n${YELLOW}Downloading backend Go dependencies...${NC}"
    cd server && go mod tidy && cd ..
fi

# 4. Port Conflict Resolution
check_and_resolve_port() {
    local PORT=$1
    local EXPECTED_KEYWORD=$2
    local SERVICE_NAME=$3

    local PID=$(lsof -t -i:$PORT -sTCP:LISTEN 2>/dev/null | head -n 1)
    if [ -n "$PID" ]; then
        local CMD_FULL=$(ps -p $PID -o command= | head -n 1)
        echo -e "${YELLOW}⚠ Port $PORT is currently occupied by PID $PID (${CMD_FULL:0:50}...).${NC}"
        
        # Check if it matches expected keyword (case insensitive, extended regex)
        if echo "$CMD_FULL" | grep -qiE "$EXPECTED_KEYWORD"; then
            echo -e "${YELLOW}  ↳ Identified as previous $SERVICE_NAME instance. Auto-killing...${NC}"
            kill -9 $PID 2>/dev/null || true
            sleep 1
        else
            echo -e "${RED}✗ Error: Port $PORT is occupied by an unexpected service.${NC}"
            echo -e "  Please stop this service manually and try again."
            exit 1
        fi
    fi
}

wait_for_backend_ready() {
    local URL="http://127.0.0.1:8080/api/auth/me"
    local MAX_ATTEMPTS=30
    local ATTEMPT=1

    echo -e "${YELLOW}⏳ Waiting for Go backend to become ready...${NC}"
    while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
        local STATUS
        STATUS=$(http_status "$URL")

        if [ "$STATUS" = "200" ] || [ "$STATUS" = "401" ]; then
            echo -e "${GREEN}✓ Go backend is ready (HTTP $STATUS).${NC}"
            return 0
        fi

        if ! kill -0 $GO_PID 2>/dev/null; then
            echo -e "${RED}✗ Go backend exited before becoming ready.${NC}"
            wait $GO_PID
            exit 1
        fi

        sleep 1
        ATTEMPT=$((ATTEMPT + 1))
    done

    echo -e "${RED}✗ Go backend did not become ready within ${MAX_ATTEMPTS}s.${NC}"
    stop_children
    exit 1
}

wait_for_frontend_ready() {
    local URL="http://127.0.0.1:5173/dashboard"
    local MAX_ATTEMPTS=30
    local ATTEMPT=1

    echo -e "${YELLOW}⏳ Waiting for Vue frontend to become ready...${NC}"
    while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
        local STATUS
        STATUS=$(http_status "$URL")

        if [ "$STATUS" = "200" ]; then
            echo -e "${GREEN}✓ Vue frontend is ready (HTTP $STATUS).${NC}"
            return 0
        fi

        if ! kill -0 $VUE_PID 2>/dev/null; then
            echo -e "${RED}✗ Vue frontend exited before becoming ready.${NC}"
            wait $VUE_PID
            stop_children
            exit 1
        fi

        sleep 1
        ATTEMPT=$((ATTEMPT + 1))
    done

    echo -e "${RED}✗ Vue frontend did not become ready within ${MAX_ATTEMPTS}s.${NC}"
    stop_children
    exit 1
}

check_lan_entry() {
    local LAN_HOST=$1
    local LAN_IPS=$2
    local FRONTEND_URL="http://${LAN_HOST}:5173/dashboard"
    local BACKEND_URL="http://${LAN_HOST}:8080/api/auth/me"
    local FRONTEND_STATUS
    local BACKEND_STATUS

    FRONTEND_STATUS=$(http_status "$FRONTEND_URL")
    BACKEND_STATUS=$(http_status "$BACKEND_URL")

    echo -e "${GREEN}  LAN Entry Self-check:${NC}"
    if [ "$FRONTEND_STATUS" = "200" ]; then
        echo -e "${GREEN}    ✓ Stable frontend reachable from this Mac: ${FRONTEND_URL}${NC}"
    else
        echo -e "${RED}    ✗ Stable frontend check failed on this Mac: ${FRONTEND_URL} (HTTP ${FRONTEND_STATUS:-000})${NC}"
    fi

    if [ "$BACKEND_STATUS" = "200" ] || [ "$BACKEND_STATUS" = "401" ]; then
        echo -e "${GREEN}    ✓ Stable backend reachable from this Mac: ${BACKEND_URL} (HTTP ${BACKEND_STATUS})${NC}"
    else
        echo -e "${RED}    ✗ Stable backend check failed on this Mac: ${BACKEND_URL} (HTTP ${BACKEND_STATUS:-000})${NC}"
    fi

    if [ -n "$LAN_IPS" ]; then
        while IFS= read -r LAN_IP; do
            if [ -z "$LAN_IP" ]; then
                continue
            fi

            local IP_FRONTEND_URL="http://${LAN_IP}:5173/dashboard"
            local IP_FRONTEND_STATUS
            IP_FRONTEND_STATUS=$(http_status "$IP_FRONTEND_URL")

            if [ "$IP_FRONTEND_STATUS" = "200" ]; then
                echo -e "${GREEN}    ✓ IP fallback reachable from this Mac: ${IP_FRONTEND_URL}${NC}"
            else
                echo -e "${YELLOW}    ⚠ IP fallback check failed on this Mac: ${IP_FRONTEND_URL} (HTTP ${IP_FRONTEND_STATUS:-000})${NC}"
            fi
        done <<< "$LAN_IPS"
    fi

    if [ "$FRONTEND_STATUS" = "200" ] && { [ "$BACKEND_STATUS" = "200" ] || [ "$BACKEND_STATUS" = "401" ]; }; then
        echo -e "${GREEN}    ✓ Local LAN self-check passed. External devices should use Stable LAN Frontend.${NC}"
        echo -e "${YELLOW}    ⚠ This confirms local LAN binding and hostname access, but cannot prove another device/VLAN/firewall can reach it.${NC}"
    else
        echo -e "${RED}    ✗ Stable LAN entry is not healthy on this Mac. Fix the failed check above before sharing the address.${NC}"
    fi
}

stop_children() {
    if [ -n "$GO_PID" ]; then
        echo "Stopping Go Server (PID: $GO_PID)..."
        kill $GO_PID 2>/dev/null
    fi

    if [ -n "$VUE_PID" ]; then
        echo "Stopping Vue Server (PID: $VUE_PID)..."
        kill $VUE_PID 2>/dev/null
    fi
}

# Graceful Shutdown Handler
cleanup() {
    echo -e "\n\n${RED}Shutting down TestCenter Platform...${NC}"
    stop_children

    echo -e "${GREEN}All services successfully stopped. Goodbye!${NC}"
    exit 0
}

echo -e "\n${GREEN}Checking port availability...${NC}"
check_and_resolve_port 8080 "main|testcenter" "Go Backend"
check_and_resolve_port 5173 "node|vite|esbuild" "Vue Frontend"

echo -e "\n${GREEN}All checks passed. Booting up servers...${NC}\n"

# Trap SIGINT and SIGTERM signals before child processes start.
trap cleanup SIGINT SIGTERM

# 4. Start Go Backend in background
echo -e "${GREEN}► Starting Go Backend (Dispatcher) on 0.0.0.0:8080...${NC}"
cd server
TESTCENTER_BACKEND_HOST=0.0.0.0 TESTCENTER_BACKEND_PORT=8080 go run main.go &
GO_PID=$!
cd ..

# Wait for backend readiness before starting the frontend proxy.
wait_for_backend_ready

# 5. Start Vue Frontend in background
echo -e "${GREEN}► Starting Vue 3 Frontend Server on 0.0.0.0:5173...${NC}"
npm run dev &
VUE_PID=$!

# Wait for Vite readiness before printing/share-checking LAN URLs.
wait_for_frontend_ready

LAN_HOST=$(get_mdns_host)
LAN_IPS=$(get_lan_ips)

echo -e "\n${GREEN}==========================================${NC}"
echo -e "${GREEN}  TestCenter is now running concurrently! ${NC}"
if [ -f "$RUNTIME_ENV_FILE" ]; then
    if is_ipv4_host "$LAN_HOST"; then
        echo -e "${YELLOW}  Stable Host Mode: IP fallback via ${RUNTIME_ENV_FILE}${NC}"
    else
        echo -e "${GREEN}  Stable Host Mode: configured hostname via ${RUNTIME_ENV_FILE}${NC}"
    fi
else
    echo -e "${GREEN}  Stable Host Mode: default DNS hostname${NC}"
fi
echo -e "${GREEN}  Local Frontend: http://localhost:5173/dashboard${NC}"
echo -e "${GREEN}  Share This Address: http://${LAN_HOST}:5173/dashboard${NC}"
echo -e "${GREEN}  Stable LAN Frontend: http://${LAN_HOST}:5173/dashboard${NC}"
echo -e "${GREEN}  Stable LAN Backend:  http://${LAN_HOST}:8080${NC}"
if [ -z "$LAN_IPS" ]; then
    echo -e "${YELLOW}  LAN IP Fallback: http://<your-computer-ip>:5173/dashboard${NC}"
else
    echo -e "${GREEN}  LAN IP Fallback:${NC}"
    while IFS= read -r LAN_IP; do
        echo -e "${GREEN}    http://${LAN_IP}:5173/dashboard${NC}"
    done <<< "$LAN_IPS"
fi
echo -e "${YELLOW}  Note: other devices should use Stable LAN Frontend, not their own localhost.${NC}"
check_lan_entry "$LAN_HOST" "$LAN_IPS"
check_stable_host_resolution "$LAN_HOST" "$LAN_IPS"
print_remote_access_help "$LAN_HOST" "$LAN_IPS"
echo -e "${GREEN}  Press Ctrl+C to stop all services.      ${NC}"
echo -e "${GREEN}==========================================${NC}\n"

# Keep script running and wait for background processes
wait
