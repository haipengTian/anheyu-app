# 阶段 5：部署与上线

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在新服务器上用 Docker Compose 跑起二开后的 anheyu-app（HTTPS、每日备份），先挂临时子域名验收，再把 `thpvip.cn` 切过去。

**Architecture:** GitHub Actions 在 `custom` 分支推送时，构建前台（`next build` standalone）和后端（Linux 静态二进制），用上游 Dockerfile 的 `full` 目标打成一个镜像，推送到私有 GHCR。服务器上 4 个容器：`anheyu`（后端，内部再拉起 Next.js）、`postgres:17`、`redis:7`、`caddy`（反向代理 + 自动 HTTPS）。只有 Caddy 暴露 80/443。

**前置条件：** 阶段 1、2、4 完成（阶段 3 已跳过）并已推送 `custom` 分支（推送前征得用户同意）；用户提供新服务器（建议 ≥ 2GB 内存、Linux x86_64、能 SSH）和域名解析权限。

**凭证原则：** 服务器密码、SSH 私钥、GHCR 令牌、数据库密码都由用户在自己的终端里处理。Claude 只给命令和检查输出，不在对话里出现任何密钥。

---

### Task D1：图床 HTTPS 方案（用户决定）

现状：`https://img.thpvip.cn` 返回的是签给 `www.cloudcdn.site` 的自签名证书，已于 2026-09-19 过期。新站是 HTTPS，浏览器会把页面里的 `http://img.thpvip.cn/...` 自动升级成 https 请求，结果加载失败。

2026-10-12 决定不迁移旧内容（跳过阶段 3）后，受影响的只剩 `tools/brand/settings.json` 里引用的几张图（头像、Logo、页脚上下班图片），以及以后新内容如果还想用这个图床。

| 方案 | 做法 | 优点 | 缺点 |
|---|---|---|---|
| **A（推荐）** | 在图床/CDN 服务商控制台给 `img.thpvip.cn` 配置免费 HTTPS 证书 | 改动最小；以后写文章还能继续用这个图床 | 要登录 CDN 控制台操作，需确认服务商支持 |
| B | 把头像、Logo、页脚那几张图上传到新站后台（自带存储），改 `settings.json` 里的地址 | 不再依赖旧图床；只有几张图 | 以后新内容的图片也要传到新站或另找图床 |

**Step 1：** 把上表给用户，等用户选。
**Step 2（A）：** 用户配置完成后验证：`curl -sS -o /dev/null -w "%{http_code}\n" "https://img.thpvip.cn/hexo-blog/%E5%A4%B4%E5%83%8F.jpg"` → `200`，且不加 `-k` 不报证书错误。然后把 `tools/brand/settings.json` 里的 `http://img.thpvip.cn` 全部改成 https 后提交。
**Step 2（B）：** 用户在新站后台上传图片后，把得到的地址替换进 `settings.json` 并提交；`grep -c "img.thpvip.cn" tools/brand/settings.json` → `0`。

---

### Task D2：镜像构建工作流

**Files:**
- Create: `.github/workflows/custom-image.yml`（后端仓库）

**Step 1：写工作流**

```yaml
# [custom] 构建二开镜像并推送到 GHCR（与上游 build.yml 互不影响）
name: Build custom image

on:
  push:
    branches: [custom]
  workflow_dispatch:
    inputs:
      frontend_ref:
        description: "前台仓库的分支或提交"
        default: custom

permissions:
  contents: read
  packages: write

jobs:
  image:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout backend
        uses: actions/checkout@v4

      - name: Checkout frontend
        uses: actions/checkout@v4
        with:
          repository: ${{ github.repository_owner }}/anheyu-app-frontend
          ref: ${{ inputs.frontend_ref || 'custom' }}
          path: frontend

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: "22"
          cache: npm
          cache-dependency-path: frontend/package-lock.json

      - name: Build frontend
        working-directory: frontend
        run: |
          npm ci
          npm run build

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build backend
        env:
          CGO_ENABLED: "0"
          GOOS: linux
          GOARCH: amd64
        run: |
          SHORT_SHA="${GITHUB_SHA::7}"
          BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
          PKG=github.com/anzhiyu-c/anheyu-app/internal/pkg/version
          go build -trimpath \
            -ldflags "-s -w -X '${PKG}.Version=custom-${SHORT_SHA}' -X '${PKG}.Commit=${SHORT_SHA}' -X '${PKG}.Date=${BUILD_DATE}'" \
            -o anheyu-app .

      - name: Image name
        id: meta
        run: echo "image=ghcr.io/${GITHUB_REPOSITORY_OWNER,,}/anheyu-app" >> "$GITHUB_OUTPUT"

      - uses: docker/setup-buildx-action@v3

      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - uses: docker/build-push-action@v6
        with:
          context: .
          target: full
          platforms: linux/amd64
          push: true
          tags: |
            ${{ steps.meta.outputs.image }}:custom
            ${{ steps.meta.outputs.image }}:custom-${{ github.sha }}
```

**Step 2：本地语法检查**

Run: `node -e "require('fs').readFileSync('.github/workflows/custom-image.yml','utf8')" && docker run --rm -v "$PWD":/repo -w /repo rhysd/actionlint:latest .github/workflows/custom-image.yml`
Expected: actionlint 无输出（拉镜像需要用户同意；不方便时跳过这一步，以 Step 4 的实际运行结果为准）。

**Step 3：提交**

```bash
git add .github/workflows/custom-image.yml
git commit -m "ci: 新增二开镜像构建工作流"
```

**Step 4：由用户在 GitHub 网页上操作，然后推送**
1. fork 的 `anheyu-app` → Actions：启用 Actions；在左侧把上游的 `Build, Release and Publish with GoReleaser`、`stale`、`issue_*` 等工作流逐个 **Disable**，只保留 `Build custom image`
2. 征得同意后推送两个仓库的 `custom` 分支：`git push -u origin custom`（前台先推，后端后推）
3. 等工作流跑完 → Expected：绿色；Packages 里出现 `anheyu-app`，标签 `custom` 和 `custom-<sha>`；把包的可见性保持 **Private**

---

### Task D3：生产部署文件

**Files（后端仓库 `deploy/` 目录）:**
- Create: `deploy/docker-compose.prod.yml`
- Create: `deploy/Caddyfile`
- Create: `deploy/.env.example`
- Create: `deploy/deploy.sh`
- Create: `deploy/backup.sh`
- Create: `deploy/README.md`

**Step 1：`deploy/docker-compose.prod.yml`**

```yaml
# [custom] 生产编排：只有 caddy 暴露端口
name: anheyu

services:
  anheyu:
    image: ghcr.io/haipengtian/anheyu-app:${ANHEYU_TAG:-custom}
    restart: always
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      - ANHEYU_DATABASE_TYPE=postgres
      - ANHEYU_DATABASE_HOST=postgres
      - ANHEYU_DATABASE_PORT=5432
      - ANHEYU_DATABASE_USER=anheyu
      - ANHEYU_DATABASE_NAME=anheyu
      - ANHEYU_DATABASE_PASSWORD=${ANHEYU_DATABASE_PASSWORD:?在 .env 中设置}
      - ANHEYU_REDIS_ADDR=redis:6379
      - ANHEYU_REDIS_PASSWORD=${ANHEYU_REDIS_PASSWORD:?在 .env 中设置}
      - REVALIDATE_TOKEN=${REVALIDATE_TOKEN:?在 .env 中设置}
    volumes:
      - ./data:/anheyu/data
      - ./static:/anheyu/static
      - ./backup:/anheyu/backup
    expose:
      - "8091"

  postgres:
    image: postgres:17
    restart: unless-stopped
    environment:
      - POSTGRES_USER=anheyu
      - POSTGRES_DB=anheyu
      - POSTGRES_PASSWORD=${ANHEYU_DATABASE_PASSWORD:?在 .env 中设置}
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U anheyu -d anheyu"]
      interval: 5s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7
    restart: unless-stopped
    command: redis-server --requirepass ${ANHEYU_REDIS_PASSWORD:?在 .env 中设置}
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD-SHELL", "redis-cli -a \"$$REDIS_PASSWORD\" ping | grep PONG"]
      interval: 5s
      timeout: 5s
      retries: 10
    environment:
      - REDIS_PASSWORD=${ANHEYU_REDIS_PASSWORD}

  caddy:
    image: caddy:2
    restart: always
    depends_on:
      - anheyu
    ports:
      - "80:80"
      - "443:443"
    environment:
      - SITE_DOMAIN=${SITE_DOMAIN:?在 .env 中设置}
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config

volumes:
  pgdata:
  redisdata:
  caddy_data:
  caddy_config:
```

**Step 2：`deploy/Caddyfile`**

```
{$SITE_DOMAIN} {
	encode zstd gzip
	reverse_proxy anheyu:8091
}
```

**Step 3：`deploy/.env.example`**（真实的 `.env` 只在服务器上，用户自己生成）

```
# 先用临时子域名，验收通过后改成 thpvip.cn
SITE_DOMAIN=new.thpvip.cn
ANHEYU_TAG=custom
# 用 openssl rand -hex 24 生成，三个值各不相同
ANHEYU_DATABASE_PASSWORD=
ANHEYU_REDIS_PASSWORD=
REVALIDATE_TOKEN=
```

**Step 4：`deploy/deploy.sh`**

```bash
#!/usr/bin/env bash
# [custom] 拉取最新镜像并重启；数据卷不受影响
set -euo pipefail
cd "$(dirname "$0")"
[ -f .env ] || { echo "缺少 .env，参考 .env.example" >&2; exit 1; }
docker compose -f docker-compose.prod.yml pull anheyu
docker compose -f docker-compose.prod.yml up -d --wait
docker image prune -f
docker compose -f docker-compose.prod.yml ps
```

**Step 5：`deploy/backup.sh`**

```bash
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
```

**Step 6：`deploy/README.md`**：写清首次部署、更新（`./deploy.sh`）、备份与恢复命令（恢复命令见 Task D6），以及「备份要同步到服务器以外」的提醒。

**Step 7：本地校验**

Run: `cd deploy && cp .env.example .env.check && sed -i 's/^\(ANHEYU_DATABASE_PASSWORD\|ANHEYU_REDIS_PASSWORD\|REVALIDATE_TOKEN\)=$/\1=x/' .env.check && docker compose --env-file .env.check -f docker-compose.prod.yml config --quiet && echo OK; rm -f .env.check`
Expected: `OK`。
Run: `bash -n deploy/deploy.sh && bash -n deploy/backup.sh && echo OK` → `OK`。

**Step 8：提交**

```bash
git add deploy/
git commit -m "chore(deploy): 新增生产编排、Caddy、部署与备份脚本"
```

---

### Task D4：服务器准备（用户执行，Claude 核对输出）

1. 规格检查：`uname -m; free -h; df -h /; cat /etc/os-release | head -2` → x86_64、内存 ≥ 2GB、磁盘余量 ≥ 10GB
2. 安装 Docker（按发行版官方文档），确认 `docker compose version`
3. 防火墙只放行 22、80、443
4. 登录 GHCR：用户在 GitHub 生成只有 `read:packages` 权限的 token，在服务器执行 `docker login ghcr.io -u haipengTian`，粘贴 token（不要发到对话里）
5. 把 `deploy/` 目录传到服务器，例如 `/opt/anheyu/`；`cp .env.example .env`，用 `openssl rand -hex 24` 生成三个值填进去；`chmod 600 .env`
6. DNS：添加 `new.thpvip.cn` 的 A 记录指向新服务器；`dig +short new.thpvip.cn` 返回新服务器 IP

---

### Task D5：首次部署

> 2026-10-12 决定不迁移旧内容（跳过阶段 3），这里不再有数据导入步骤，新站从零开始。

1. 服务器：`cd /opt/anheyu && chmod +x deploy.sh backup.sh && ./deploy.sh` → 4 个容器 `running`/`healthy`
2. `curl -sS https://new.thpvip.cn/api/public/essays` → `{"code":200,...}`，证书有效（Caddy 自动签发）
3. 用户在浏览器打开 `https://new.thpvip.cn/login` 注册管理员（第一个注册的账号）
4. 后台「系统设置」把 `SITE_URL` 设为 `https://new.thpvip.cn`；导入 `tools/brand/settings.json`
5. 按 `tools/brand/README.md`「导入后还要在后台手动做的」清单逐项处理（默认文章、友链、3 个页面、打赏收款码）；关于页已包含在 `settings.json` 里，不用手填
6. 把阶段 4 Task R4 Step 3 的残留检查脚本里的 `http://localhost:3000` 换成 `https://new.thpvip.cn` 再跑一遍 → 无输出

---

### Task D6：备份与恢复演练

1. 服务器：`./backup.sh` → 生成 `backups/db-*.dump`、`backups/data-*.tar.gz`
2. 定时任务：`crontab -e` 加 `30 3 * * * /opt/anheyu/backup.sh >> /opt/anheyu/backups/backup.log 2>&1`
3. 异地副本：和用户确定方式（对象存储 / 定期 `scp` 到本机），写进 `deploy/README.md`
4. **恢复演练**（在同一台服务器上用临时库，不动线上库）：
   ```bash
   docker compose -f docker-compose.prod.yml exec -T postgres createdb -U anheyu restore_check
   docker compose -f docker-compose.prod.yml exec -T postgres pg_restore -U anheyu -d restore_check < backups/db-<最新>.dump
   docker compose -f docker-compose.prod.yml exec -T postgres psql -U anheyu -d restore_check -c "select count(*) from articles; select count(*) from essays;"
   docker compose -f docker-compose.prod.yml exec -T postgres dropdb -U anheyu restore_check
   ```
   Expected：文章数、短文数与线上一致

---

### Task D7：上线验收（逐项截图或贴输出）

- [ ] 后台发布一篇测试文章，前台首页和文章页**立即**可见（`REVALIDATE_TOKEN` 生效）；验收后删除
- [ ] 短文增删改查正常，`/essay` 和首页滚动条正确
- [ ] 图片全部能加载（浏览器控制台无 mixed content / 证书错误）
- [ ] 残留检查脚本无输出
- [ ] 手机端浏览正常（`resize_window` 375×812）
- [ ] HTTPS 证书有效，`http://` 自动跳转 `https://`
- [ ] 备份与恢复演练通过

---

### Task D8：切换正式域名（验收全部通过、用户确认后）

1. 服务器 `.env`：`SITE_DOMAIN=thpvip.cn`；后台 `SITE_URL` 改为 `https://thpvip.cn`
2. DNS：`thpvip.cn` 的 A 记录改为新服务器 IP（切换前把 TTL 调低到 300）
3. `./deploy.sh`，等 Caddy 为 `thpvip.cn` 签好证书：`curl -sSI https://thpvip.cn | head -1` → `HTTP/2 200`
4. 再跑一遍残留检查和 D7 前 3 项
5. 旧服务器保留一周；一周后由用户决定下线 Hexo、Webhook（`pm2 delete hexo-webhook`）和旧站点；`new.thpvip.cn` 的 DNS 记录删除

---

## 阶段 5 验收（向用户做最终汇报）

1. D7 清单全部勾选及证据
2. 镜像标签、服务器上 `docker compose ps` 输出
3. 备份定时任务与异地副本方式
4. 遗留事项清单（如：旧服务器下线日期、需要用户后续手动维护的配置）
