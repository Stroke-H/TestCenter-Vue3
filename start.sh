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

echo -e "\n${GREEN}Checking port availability...${NC}"
check_and_resolve_port 8080 "main|testcenter" "Go Backend"
check_and_resolve_port 5173 "node|vite|esbuild" "Vue Frontend"

echo -e "\n${GREEN}All checks passed. Booting up servers...${NC}\n"

# 4. Start Go Backend in background
echo -e "${GREEN}► Starting Go Backend (Dispatcher) on :8080...${NC}"
cd server
go run main.go &
GO_PID=$!
cd ..

# Give the backend a second to initialize
sleep 1

# 5. Start Vue Frontend in background
echo -e "${GREEN}► Starting Vue 3 Frontend Server...${NC}"
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

echo -e "\n${GREEN}==========================================${NC}"
echo -e "${GREEN}  TestCenter is now running concurrently! ${NC}"
echo -e "${GREEN}  Backend: http://localhost:8080          ${NC}"
echo -e "${GREEN}  Press Ctrl+C to stop all services.      ${NC}"
echo -e "${GREEN}==========================================${NC}\n"

# Keep script running and wait for background processes
wait
