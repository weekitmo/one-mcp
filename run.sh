#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

if [ -f ".env" ]; then
  set -a
  # shellcheck disable=SC1091
  source ".env"
  set +a
fi

export PATH="$PATH:/usr/local/bin"

PORT="${PORT:-3299}"
FRONTEND_MODE="${FRONTEND_MODE:-dist}"
VITE_PORT="${VITE_PORT:-3300}"

RUNTIME_DIR=".run"
BACKEND_PID_FILE="$RUNTIME_DIR/one-mcp.pid"
BACKEND_LOG_FILE="$RUNTIME_DIR/backend.log"
FRONTEND_PID_FILE="$RUNTIME_DIR/vite.pid"
FRONTEND_LOG_FILE="$RUNTIME_DIR/frontend.log"

SKIP_PNPM_BUILD=0
while getopts ":s" opt; do
  case "$opt" in
    s)
      SKIP_PNPM_BUILD=1
      ;;
    \?)
      echo "Usage: $0 [-s]"
      exit 1
      ;;
  esac
done

mkdir -p "$RUNTIME_DIR"

echo "Stopping existing processes first..."
PORT="$PORT" FRONTEND_MODE="$FRONTEND_MODE" VITE_PORT="$VITE_PORT" ./stop.sh >/dev/null 2>&1 || true

if [ "$FRONTEND_MODE" = "dev" ]; then
  echo "Installing frontend dependencies (dev mode)..."
  pnpm --dir frontend install
  if [ "$SKIP_PNPM_BUILD" -eq 0 ] && [ ! -f "frontend/dist/index.html" ]; then
    echo "frontend/dist missing; building once for backend embed..."
    pnpm --dir frontend run build
  fi
else
  echo "Building frontend (dist mode)..."
  pnpm --dir frontend install
  if [ "$SKIP_PNPM_BUILD" -eq 1 ]; then
    echo "Skipping pnpm build due to -s flag."
  else
    pnpm --dir frontend run build
  fi
fi

echo "Building backend service..."
go build -o one-mcp .

wait_for_port() {
  local port="$1"
  local timeout_seconds="${2:-12}"
  local elapsed=0
  while [ "$elapsed" -lt "$timeout_seconds" ]; do
    if lsof -ti TCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done
  return 1
}

if [ "$FRONTEND_MODE" = "dev" ]; then
  echo "Starting frontend development server with nohup..."
  nohup pnpm --dir frontend run dev >"$FRONTEND_LOG_FILE" 2>&1 &
  FRONTEND_PID=$!
  echo "$FRONTEND_PID" >"$FRONTEND_PID_FILE"
fi

echo "Starting backend service with nohup..."
nohup ./one-mcp >"$BACKEND_LOG_FILE" 2>&1 &
BACKEND_PID=$!
echo "$BACKEND_PID" >"$BACKEND_PID_FILE"

if ! wait_for_port "$PORT"; then
  echo "Backend failed to listen on port $PORT. Check $BACKEND_LOG_FILE:"
  tail -n 40 "$BACKEND_LOG_FILE" || true
  exit 1
fi

if [ "$FRONTEND_MODE" = "dev" ] && ! wait_for_port "$VITE_PORT"; then
  echo "Frontend failed to listen on port $VITE_PORT. Check $FRONTEND_LOG_FILE:"
  tail -n 40 "$FRONTEND_LOG_FILE" || true
  exit 1
fi

echo
echo "Services started in background:"
echo "- Backend: http://localhost:$PORT (PID: $BACKEND_PID)"
echo "  Log: $BACKEND_LOG_FILE"
if [ "$FRONTEND_MODE" = "dev" ]; then
  echo "- Frontend (dev): http://localhost:$VITE_PORT (PID: $FRONTEND_PID)"
  echo "  Log: $FRONTEND_LOG_FILE"
else
  echo "- Frontend served by backend at http://localhost:$PORT/"
fi
echo
echo "Use ./stop.sh to stop services."
