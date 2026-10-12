# 生产部署（[custom] 二开新增）

服务器上 4 个容器：`anheyu`（后端，内部再拉起 Next.js 前台）、`postgres:17`、`redis:7`、`caddy`（反向代理 + 自动 HTTPS）。只有 Caddy 暴露 80/443。

镜像由后端仓库 `custom` 分支的 `Build custom image` 工作流构建，推送到 `ghcr.io/haipengtian/anheyu-app`（标签 `custom` 和 `custom-<sha>`）。

## 首次部署

1. 服务器装好 Docker（含 `docker compose`），防火墙只放行 22、80、443
2. 用只有 `read:packages` 权限的 GitHub token 登录：`docker login ghcr.io -u haipengTian`（token 不要写进任何文件）
3. 把本目录传到服务器，例如 `/opt/anheyu/`
4. `cp .env.example .env`，用 `openssl rand -hex 24` 生成三个不同的值填进去，`chmod 600 .env`
5. 域名 A 记录指向服务器后：`chmod +x deploy.sh backup.sh && ./deploy.sh`
6. 浏览器打开 `https://<SITE_DOMAIN>/login` 注册管理员（第一个注册的账号）
7. 后台「系统设置 → 站点信息」把 `SITE_URL` 设为 `https://<SITE_DOMAIN>`；「备份导入」导入 `tools/brand/settings.json`
8. 按 `tools/brand/README.md` 的清单清理初始化数据

## 更新

`custom` 分支推送后等工作流跑完，在服务器执行 `./deploy.sh`。数据在 `data/` 和 Docker 卷里，不受影响。

回滚到某次构建：`.env` 里把 `ANHEYU_TAG` 改成 `custom-<完整 sha>`，再 `./deploy.sh`。

## 备份

`./backup.sh` 生成 `backups/db-<时间>.dump`（数据库）和 `backups/data-<时间>.tar.gz`（上传文件、配置），保留 14 天。

定时任务（每天 03:30）：

```
30 3 * * * /opt/anheyu/backup.sh >> /opt/anheyu/backups/backup.log 2>&1
```

**备份必须再同步到服务器以外**（对象存储或定期拷回本机），否则服务器坏了备份也一起没了。

## 恢复

先在临时库演练，确认备份可用：

```bash
docker compose -f docker-compose.prod.yml exec -T postgres createdb -U anheyu restore_check
docker compose -f docker-compose.prod.yml exec -T postgres pg_restore -U anheyu -d restore_check < backups/db-<时间>.dump
docker compose -f docker-compose.prod.yml exec -T postgres psql -U anheyu -d restore_check -c "select count(*) from articles; select count(*) from essays;"
docker compose -f docker-compose.prod.yml exec -T postgres dropdb -U anheyu restore_check
```

真正恢复到线上库（会覆盖现有数据，先停应用）：

```bash
docker compose -f docker-compose.prod.yml stop anheyu
docker compose -f docker-compose.prod.yml exec -T postgres pg_restore -U anheyu -d anheyu --clean --if-exists < backups/db-<时间>.dump
tar -xzf backups/data-<时间>.tar.gz
docker compose -f docker-compose.prod.yml start anheyu
```
