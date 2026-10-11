#!/usr/bin/env bash
# 本地开发启动后端：连接 docker-compose.local.yml 启动的 PostgreSQL / Redis，
# 前台由 anheyu-app-frontend 仓库的 `npm run dev` 单独提供（默认 http://localhost:3000）。
# 用法: bash scripts/dev-local.sh
set -euo pipefail

cd "$(dirname "$0")/.."

export ANHEYU_DATABASE_TYPE=postgres
export ANHEYU_DATABASE_HOST=127.0.0.1
export ANHEYU_DATABASE_PORT=5434
export ANHEYU_DATABASE_USER=anheyu
export ANHEYU_DATABASE_NAME=anheyu
export ANHEYU_DATABASE_PASSWORD="${ANHEYU_DATABASE_PASSWORD:-changeme}"
export ANHEYU_REDIS_ADDR=127.0.0.1:6380
export ANHEYU_REDIS_PASSWORD="${ANHEYU_REDIS_PASSWORD:-changeme}"
# 使用外部（next dev）前台，后端不再自行拉起内置 Next.js
export ANHEYU_FRONTEND_URL="${ANHEYU_FRONTEND_URL:-http://localhost:3000}"

exec go run .
