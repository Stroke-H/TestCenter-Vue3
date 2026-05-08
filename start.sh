#!/bin/bash

# Define colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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

get_lan_ips() {
    local DEFAULT_IFACE
    DEFAULT_IFACE=$(route get default 2>/dev/null | awk '/interface:/{print $2; exit}')

    if [ -n "$DEFAULT_IFACE" ]; then
        local DEFAULT_IP
        DEFAULT_IP=$(ipconfig getifaddr "$DEFAULT_IFACE" 2>/dev/null)
        if [ -n "$DEFAULT_IP" ]; then
            echo "$DEFAULT_IP"
        fi
    fi

    ifconfig 2>/dev/null | awk '/inet / && $2 !~ /^127\./ && $2 !~ /^198\.18\./ {print $2}' | awk '!seen[$0]++'
}

get_mdns_host() {
    local LOCAL_HOST_NAME
    LOCAL_HOST_NAME=$(scutil --get LocalHostName 2>/dev/null)

    if [ -n "$TESTCENTER_LAN_HOST" ]; then
        echo "$TESTCENTER_LAN_HOST"
        return 0
    fi

    if [ -n "$LOCAL_HOST_NAME" ]; then
        printf "%s.local\n" "$LOCAL_HOST_NAME" | tr '[:upper:]' '[:lower:]'
        return 0
    fi

    echo "strokeh.local"
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
        STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$URL" || true)

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
    exit 1
}

echo -e "\n${GREEN}Checking port availability...${NC}"
check_and_resolve_port 8080 "main|testcenter" "Go Backend"
check_and_resolve_port 5173 "node|vite|esbuild" "Vue Frontend"

echo -e "\n${GREEN}All checks passed. Booting up servers...${NC}\n"

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

# 6. Graceful Shutdown Handler
cleanup() {
    echo -e "\n\n${RED}Shutting down TestCenter Platform...${NC}"
    echo "Stopping Go Server (PID: $GO_PID)..."
    kill $GO_PID 2>/dev/null
    
    echo "Stopping Vue Server (PID: $VUE_PID)..."
    kill $VUE_PID 2>/dev/null
    
    echo -e "${GREEN}All services successfully stopped. Goodbye!${NC}"
    exit 0
}

# Trap SIGINT and SIGTERM signals
trap cleanup SIGINT SIGTERM

LAN_HOST=$(get_mdns_host)
LAN_IPS=$(get_lan_ips)

echo -e "\n${GREEN}==========================================${NC}"
echo -e "${GREEN}  TestCenter is now running concurrently! ${NC}"
echo -e "${GREEN}  Local Frontend: http://localhost:5173/dashboard${NC}"
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
echo -e "${GREEN}  Press Ctrl+C to stop all services.      ${NC}"
echo -e "${GREEN}==========================================${NC}\n"

# Keep script running and wait for background processes
wait
