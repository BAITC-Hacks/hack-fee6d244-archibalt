#!/bin/bash
# Локальный dev-стенд с горячей перезагрузкой: Go API на :8080 (перезапуск при изменении *.go), Vite с HMR на :5173.
# ponytail: поллинг mtime раз в 2 с вместо fswatch/air — ноль зависимостей. Postgres — docker compose up -d db.
cd "$(dirname "$0")"
set -a; [ -f .env ] && . ./.env; set +a
export DATABASE_URL="${DATABASE_URL:-postgres://postgres:hack@localhost:5432/hack?sslmode=disable}"
export PORT="${PORT:-8080}" STATIC_DIR=/nonexistent DEV_PROXY=http://127.0.0.1:5173
docker compose up -d db >/dev/null 2>&1
LOG=.agents/dev-go.log; mkdir -p .agents
sig() { find cmd internal db go.mod go.sum -type f \( -name '*.go' -o -name '*.sql' -o -name 'go.*' \) -newer .agents/.dev-stamp 2>/dev/null | head -1; }
build() { go build -o .agents/dev-server.new ./cmd/server 2>>"$LOG"; }
start() { .agents/dev-server >>"$LOG" 2>&1 & GO_PID=$!; echo "[dev] Go API :$PORT pid $GO_PID"; }
rebuild() { touch .agents/.dev-stamp; if build; then mv .agents/dev-server.new .agents/dev-server; stop; start; else echo "[dev] build failed — старый сервер продолжает работать, см. $LOG"; fi; }
stop() { [ -n "$GO_PID" ] && kill "$GO_PID" 2>/dev/null; wait "$GO_PID" 2>/dev/null; }
trap 'stop; kill $VITE_PID 2>/dev/null; exit 0' INT TERM
touch -t 200001010000 .agents/.dev-stamp; build && mv .agents/dev-server.new .agents/dev-server && start
( cd web && npx vite --host 127.0.0.1 --port 5173 >>../.agents/dev-vite.log 2>&1 ) & VITE_PID=$!  # только localhost, не LAN
echo "[dev] открой http://localhost:8080 (Go проксирует фронт в Vite :5173 с HMR); логи .agents/dev-*.log"
while true; do sleep 2; if [ -n "$(sig)" ]; then echo "[dev] изменения в Go — пересборка"; rebuild; fi; done
