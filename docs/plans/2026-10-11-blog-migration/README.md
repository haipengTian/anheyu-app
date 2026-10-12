# 博客动态化（二开 anheyu-app）实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把 Hexo 静态博客换成二开的 anheyu-app：新增「即刻短文」，完成品牌化，部署到新服务器上线。旧内容不迁移，新站从零开始（2026-10-12 用户决定，见文末「计划变更记录」）。

**Architecture:** 上游代码不动结构，二开只在 `custom` 分支上做"新增文件 + 少量挂载点"。即刻短文按上游 `doc_series` 模块的分层（ent schema → domain model/repo 接口 → ent repo → service → handler → 路由）新增；前台按 `doc-series` 后台页和 `recentcomments` 前台页的写法新增。

**Tech Stack:** Go 1.25 + Ent + Gin（后端）；Next.js 16 + React 19 + TypeScript + HeroUI + TanStack Query + Vitest（前台）；Docker Compose + Caddy + GitHub Actions + GHCR（部署）。

**设计文档：** [`../2026-10-11-anheyu-migration-design.md`](../2026-10-11-anheyu-migration-design.md)

---

## 仓库与分支

| 仓库 | 本地路径 | 工作分支 |
|---|---|---|
| 后端 | `E:\workspace\code\blog\anheyu-app` | `custom` |
| 前台 | `E:\workspace\code\blog\anheyu-app-frontend` | `custom` |
| 旧博客（只读参考） | `E:\workspace\personal\code\hexo-blog` | `master`（不改动；品牌配置从这里取站点名、头像等） |

计划里的路径：后端任务写的是相对 `anheyu-app/` 的路径，前台任务写的是相对 `anheyu-app-frontend/` 的路径。

## 阶段总览

| 阶段 | 文件 | 产出 | 依赖 |
|---|---|---|---|
| 1 | [01-essay-backend.md](01-essay-backend.md) | 即刻短文后端：表、接口、测试 | 无 |
| 2 | [02-essay-frontend.md](02-essay-frontend.md) | 即刻短文前台页、首页滚动条、后台管理页 | 阶段 1 |
| ~~3~~ | ~~[03-content-migration.md](03-content-migration.md)~~ | **已跳过**（2026-10-12）：旧内容不迁移，文件保留备查 | — |
| 4 | [04-branding.md](04-branding.md) | 2 处代码改动 + 品牌配置 JSON + 后台数据清理 | 阶段 2 |
| 5 | [05-deploy.md](05-deploy.md) | 镜像构建、生产编排、HTTPS、备份、切换域名 | 阶段 1、2、4；**需要用户提供新服务器** |

## 执行规则

1. **一次一个任务**：严格按任务里的步骤做——先写测试、确认失败、再实现、确认通过、最后提交。
2. **每个任务结束都要有证据**：贴出任务里「验证」步骤的实际命令输出，不能只说"通过了"。
3. **每个阶段结束停下来**，向用户演示和汇报（截图 / 接口返回 / 测试结果），用户确认后再开始下一阶段。
4. **提交规范**：
   - 只在 `custom` 分支提交，**不推送**，除非用户明确要求。
   - 提交信息用约定式提交（`feat:` / `fix:` / `chore:` / `test:` / `docs:`），结尾加：
     ```
     Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
     ```
   - 提交前运行该任务列出的验证命令（构建 + 相关测试）；纯文档提交除外。
5. **对上游文件的改动**必须在代码旁加 `// [custom]`（或 `{/* [custom] */}`）注释，方便以后合并上游时识别。全部挂载点清单见下方。
6. **遇到计划与代码不符**（上游代码变了、命令报错和预期不同）：停下来报告，不要自行绕过。

## 对上游文件的全部改动（挂载点清单）

| 仓库 | 文件 | 改动 | 阶段 |
|---|---|---|---|
| 后端 | `cmd/server/app.go` | 加 2 个 import + 3 行注册短文路由 | 1 |
| 后端 | `ent/*`（生成代码） | `go generate ./ent` 生成；合并上游冲突时重新生成即可 | 1 |
| 前台 | `src/config/admin-menu.ts` | 「内容管理」组加一个菜单项 | 2 |
| 前台 | `src/components/home/HomePageContent.tsx` | 挂载 `EssayBar` | 2 |
| 前台 | `src/components/home/HomePageContent.test.tsx` | mock 掉 `EssayBar`（测试里没有 `QueryClientProvider`） | 2 |
| 前台 | `src/components/admin/article-editor/extensions/music-block.tsx` | `fetchMusicDetail` 加 `export` | 2 |
| 前台 | `src/providers/console-printer.tsx` | import 指向新增的 `console-printer-custom.ts`（控制台欢迎语改为读取站点名，上游 `console-printer.ts` 不动） | 4 |
| 前台 | `src/app/(frontend)/posts/[id]/page.tsx`、`src/app/(frontend)/doc/[id]/page.tsx` | `<meta property="article:author">` 默认值改为站点拥有者 | 4 |
| 前台 | `src/components/post/PostDetailContent.tsx` | 底部栏改用新增的 `buildPostFooterText`（去掉写死的「安知鱼」兜底，文案改为「来自 X 的最新文章」） | 4（R4 用户决定） |

其余全部是新增文件。

## 本地开发环境（每次开工前）

```bash
cd /e/workspace/code/blog/anheyu-app
docker compose -f docker-compose.local.yml up -d --wait
bash scripts/dev-local.sh            # 后端，监听 8091（建议放后台或单独终端）
```

前台（PowerShell，单独终端）：

```powershell
cd E:\workspace\code\blog\anheyu-app-frontend
$env:NODE_OPTIONS = '--max-http-header-size=65536'; npx next dev
```

访问 http://localhost:3000 。第一个注册的账号自动成为管理员（由用户本人注册，Claude 不代建账号、不输入密码）。

## 已知风险（执行中需要用户决定的事）

| 风险 | 何时处理 |
|---|---|
| 图床 `img.thpvip.cn` 的 HTTPS 证书是自签名且已于 2026-09-19 过期；新站上 HTTPS 后 http 图片会被浏览器升级为 https 而加载失败 | 已处理（D1）：暂不用该图床，品牌图片放在前台 `public/static/brand/`；以后接入新图床时再换 |
| 上游的 `build.yml` 在推送 `main` 时会在 fork 里运行并可能失败 | 阶段 5 任务 D2：在 fork 的 Actions 设置里禁用上游工作流 |
| 切换正式域名后，旧 Hexo 文章的网址全部失效（内容仍在 hexo-blog 仓库） | 阶段 5 任务 D8 前和用户再确认一次；需要的文章可在后台粘贴 Markdown 重新发布 |

## 计划变更记录

| 日期 | 变更 | 影响 |
|---|---|---|
| 2026-10-12 | 用户决定不迁移旧 Hexo 内容，跳过阶段 3（9 个任务） | 阶段 4：首页分类按钮默认留空，「相册集」菜单在 R3 确认；阶段 5：D1 方案简化，D5 去掉数据导入，D7 去掉迁移统计 |
| 2026-10-12 | D1：暂不使用 `img.thpvip.cn`，品牌图片放进前台仓库；打赏先关闭，上线后在正式站后台上传自己的收款码再开启 | `tools/brand/settings.json` 不再引用旧图床 |
| 2026-10-12 | 阶段 2 执行中的调整：后台表单上传框加「加入图片」按钮（用户选定）；首页挂载容器加 `empty:hidden`；滚动条图标不套上游描边样式；发布时间输入框加空占位符 | 见前台仓库 `custom` 分支提交记录 |
