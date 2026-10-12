#!/usr/bin/env bash
# [custom] 拉取最新镜像并重启；数据卷不受影响
set -euo pipefail
cd "$(dirname "$0")"
[ -f .env ] || { echo "缺少 .env，参考 .env.example" >&2; exit 1; }
docker compose -f docker-compose.prod.yml pull anheyu
docker compose -f docker-compose.prod.yml up -d --wait
docker image prune -f
docker compose -f docker-compose.prod.yml ps
