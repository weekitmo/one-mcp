#!/bin/bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

PORT="${PORT:-${1:-3299}}"
FRONTEND_MODE="${FRONTEND_MODE:-dist}"
VITE_PORT="${VITE_PORT:-3300}"

RUNTIME_DIR=".run"
BACKEND_PID_FILE="$RUNTIME_DIR/one-mcp.pid"
FRONTEND_PID_FILE="$RUNTIME_DIR/vite.pid"

mkdir -p "$RUNTIME_DIR"

stop_pid_from_file() {
  local pid_file="$1"
  local name="$2"
  local pid

  if [ ! -f "$pid_file" ]; then
    return 0
  fi

  pid="$(cat "$pid_file" 2>/dev/null || true)"
  if [ -z "$pid" ]; then
    rm -f "$pid_file"
    return 0
  fi

  if ps -p "$pid" >/dev/null 2>&1; then
    echo "Stopping $name by PID $pid..."
    kill -TERM "$pid" 2>/dev/null || true
    sleep 1
    if ps -p "$pid" >/dev/null 2>&1; then
      kill -9 "$pid" 2>/dev/null || true
    fi
  fi

  rm -f "$pid_file"
}

stop_listeners_by_port() {
  local port="$1"
  local name="$2"
  local pids

  pids="$(lsof -ti TCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -z "$pids" ]; then
    return 0
  fi

  echo "Stopping $name by port $port: $pids"
  for pid in $pids; do
    kill -TERM "$pid" 2>/dev/null || true
  done

  sleep 1

  pids="$(lsof -ti TCP:"$port" -sTCP:LISTEN 2>/dev/null || true)"
  if [ -n "$pids" ]; then
    for pid in $pids; do
      kill -9 "$pid" 2>/dev/null || true
    done
  fi
}

wait_port_closed() {
  local port="$1"
  local timeout_seconds="${2:-10}"
  local elapsed=0

  while [ "$elapsed" -lt "$timeout_seconds" ]; do
    if ! lsof -ti TCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    elapsed=$((elapsed + 1))
  done

  echo "Warning: port $port is still in LISTEN state after ${timeout_seconds}s."
  return 1
}

stop_pid_from_file "$BACKEND_PID_FILE" "backend"
stop_listeners_by_port "$PORT" "backend"
wait_port_closed "$PORT" 10 || true

if [ "$FRONTEND_MODE" = "dev" ] || [ -f "$FRONTEND_PID_FILE" ]; then
  stop_pid_from_file "$FRONTEND_PID_FILE" "frontend"
  stop_listeners_by_port "$VITE_PORT" "frontend"
  wait_port_closed "$VITE_PORT" 10 || true
fi

echo "Stop completed. Backend port: $PORT"
