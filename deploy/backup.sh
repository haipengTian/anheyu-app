#!/usr/bin/env bash
# [custom] 每日备份：数据库 + data 目录，保留 14 天
set -euo pipefail
cd "$(dirname "$0")"
STAMP="$(date +%Y%m%d-%H%M%S)"
DEST="backups"
mkdir -p "$DEST"
docker compose -f docker-compose.prod.yml exec -T postgres pg_dump -U anheyu -d anheyu -Fc > "$DEST/db-$STAMP.dump"
tar -czf "$DEST/data-$STAMP.tar.gz" data
find "$DEST" -type f -mtime +14 -delete
echo "备份完成: $DEST/db-$STAMP.dump $DEST/data-$STAMP.tar.gz"
