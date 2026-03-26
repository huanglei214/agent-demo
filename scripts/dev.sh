#!/bin/sh

set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
API_PID=""
WEB_PID=""

cleanup() {
  status=$?
  trap - INT TERM EXIT

  if [ -n "$API_PID" ]; then
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi

  if [ -n "$WEB_PID" ]; then
    kill "$WEB_PID" 2>/dev/null || true
    wait "$WEB_PID" 2>/dev/null || true
  fi

  exit "$status"
}

trap cleanup INT TERM EXIT

echo "[dev] starting local API on http://${HOST:-127.0.0.1}:${PORT:-8080}"
(
  cd "$ROOT_DIR"
  make serve \
    PROVIDER="${PROVIDER:-mock}" \
    MODEL="${MODEL:-}" \
    WORKSPACE="${WORKSPACE:-$ROOT_DIR}" \
    HOST="${HOST:-127.0.0.1}" \
    PORT="${PORT:-8080}" 2>&1 | sed 's/^/[api] /'
) &
API_PID=$!

sleep 1

echo "[dev] starting web UI on http://127.0.0.1:5173"
(
  cd "$ROOT_DIR"
  make web-dev 2>&1 | sed 's/^/[web] /'
) &
WEB_PID=$!

wait "$API_PID" "$WEB_PID"
