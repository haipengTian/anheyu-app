# 博客动态化：二开 anheyu-app 设计文档

- 日期：2026-10-11
- 状态：设计已确认，待实施
- 现状：Hexo 8 + AnZhiYu 主题（静态站），Gitee 托管，Webhook 自动构建，域名 thpvip.cn

## 1. 目标与范围

把博客从「本地写 Markdown → git push → 静态生成」改为「后台在线编辑、保存即生效」的动态站，保留 AnZhiYu 的 UI、动画与布局风格。

基于 AnZhiYu 作者开源的动态版 [anzhiyu-c/anheyu-app](https://github.com/anzhiyu-c/anheyu-app)（Go 后端）+ [anzhiyu-c/anheyu-app-frontend](https://github.com/anzhiyu-c/anheyu-app-frontend)（Next.js 前台/后台，子模块）进行二次开发。许可证 GPL-3.0。

**本次要做：**
1. 即刻短文（开源版缺失，需全栈自研）
2. 现有内容批量迁移
3. 品牌化改造（前台不出现原作者信息）
4. 部署到新服务器并切换域名

**明确不做（YAGNI）：**
- 访客注册、评论等互动功能
- 面试导航页（`source/interview-map`）移植
- 旧文章链接兼容（旧地址直接 404）
- PRO 功能（付费/密码文章、第三方登录、AI 写作等）

## 2. 仓库与工作方式

- **本地目录**：`E:\workspace\personal\code\anheyu-app`
- **GitHub fork**：`<你>/anheyu-app`、`<你>/anheyu-app-frontend`（需用户本人在 GitHub 网页上 fork）。后端仓库的 `.gitmodules` 改为指向自己的前台 fork（https 地址）。
- **远程**：`origin` = 自己的 fork，`upstream` = anzhiyu-c 原仓库。
- **基线版本**：后端 release `v1.8.25`（2026-09-23），前台取该 tag 中子模块记录的提交（前台仓库无 tag）。
- **分支**：
  - `main`：只做上游同步，不放自己的改动
  - `custom`：全部二开改动；升级时 `upstream → main → merge 进 custom`
- **降低合并冲突的原则**：新增文件优先，少改原文件。新功能做成独立模块，对原代码只改必要的挂载点；作者的私人页面不删文件，只去掉入口。
- **本地开发**（2026-10-11 已跑通）：
  - 代码目录：`E:\workspace\code\blog\anheyu-app` 与 `E:\workspace\code\blog\anheyu-app-frontend` 并排克隆（GitHub 账号 `haipengTian`）。日常前台开发在并排的前台仓库中进行，后端仓库的 `frontend` 子模块仅供构建镜像使用，发版时更新子模块指针。
  - 依赖服务：`docker compose -f docker-compose.local.yml up -d`，PostgreSQL 17 → `127.0.0.1:5434`，Redis → `127.0.0.1:6380`（避开本机已占用的 5432、5433、6379）。
  - 后端：`bash scripts/dev-local.sh`，通过 `ANHEYU_*` 环境变量连接上述依赖，`ANHEYU_FRONTEND_URL` 指向外部前台；监听 8091。后端不读取 `.env`，被 git 跟踪的 `data/conf.ini` 不做修改。
  - 前台：上游 `dev` 脚本依赖未声明的 `cross-env`，本地改用 PowerShell 命令 `$env:NODE_OPTIONS='--max-http-header-size=65536'; npx next dev` 启动，不改 `package.json`。访问 http://localhost:3000 ，`/api` 由 Next 代理到 8091。
  - 第一个注册的账号自动成为管理员。
  - 本机已有 Go 1.26、Node 22、Docker 29；后端为纯 Go（不依赖 cgo），可在 Windows 上直接编译。

上游技术栈：Go + Ent（ORM）+ Gin；Next.js 16 + React 19 + TypeScript + HeroUI + Tailwind 4 + TanStack Query + Zustand；测试为 Go test + Vitest。

## 3. 即刻短文

开源版现状：首页只剩样式文件 `src/components/home/HomeTop/EssayBar.module.css`，没有组件、后端表、接口和后台管理页。

### 3.1 后端（照搬「相册」模块分层，全部为新文件）

| 层 | 文件 |
|---|---|
| Schema | `ent/schema/essay.go` |
| 领域模型 / 仓储接口 | `pkg/domain/model/essay.go`、`pkg/domain/repository/essay_repo.go` |
| 仓储实现 | `internal/infra/persistence/ent/essay_repo.go` |
| Service | `pkg/service/essay/service.go` |
| Handler | `pkg/handler/essay/handler.go` |

**`essay` 表字段**：

| 字段 | 说明 |
|---|---|
| `id`、`created_at`、`updated_at` | 标准字段；软删除沿用 `SoftDeleteMixin` |
| `content` | 文字内容 |
| `published_at` | 发布时间，可手动指定（迁移时保留原日期） |
| `images` / `videos` | JSON 字符串数组 |
| `address` / `from` | 地点 / 来源 |
| `link_url` / `link_title` | 外链卡片 |
| `music_id` | 网易云歌曲 ID |
| `is_published` | 是否公开 |

**接口**：
- 后台：`/api/essays`，增删改查 + 分页，`JWTAuth + AdminAuth`
- 公开：`/api/public/essays`，分页，只返回公开的；首页滚动条取最新 N 条

**校验**：文字与媒体至少有一项；URL 必须为 http(s)；`music_id` 必须为数字；所有字符串字段限制最大长度；错误返回沿用项目统一的 `response.Response` 格式。

### 3.2 前台（Next.js）

- `/essay` 页面：卡片流；图片宫格（复用现有大图预览）、视频、地点与来源标签、外链卡片、音乐卡片（复用 `src/components/MusicPlayer` 和网易云音乐接口）
- 首页 `EssayBar` 组件：沿用现有样式，轮播最新短文，点击跳转 `/essay`
- 后台 `/admin/essays`：列表 + 编辑表单；图片可复用现有上传接口，也可直接粘贴图片 URL
- API 客户端、类型与 hooks：`src/lib/api/essay.ts`、`src/types/essay.ts`，使用 TanStack Query

### 3.3 原代码挂载点（只改这 4 处）

1. `internal/infra/router/router.go`：注册路由
2. 依赖注入：组装 essay 的 repo、service、handler
3. `HomeTop`：挂载 `EssayBar`
4. 后台菜单：增加「即刻短文」入口

### 3.4 测试

- Go：service 校验逻辑、handler 的请求与响应
- Vitest：`EssayBar`、短文列表渲染

## 4. 内容迁移

**工具**：`tools/hexo-migrate/`（Node.js，放在 fork 仓库中，全部为新文件）。

### 4.1 文章（21 篇已发布 + 5 篇草稿）

- 上游导入接口为 `POST /articles/import`，接受 JSON 或 ZIP。导入器**不会**把 Markdown 渲染成 HTML，会按 abbrlink 或标题跳过已存在的文章。
- 字段映射：

| Hexo | 导入格式 |
|---|---|
| `title` | `title` |
| 正文 | `content_md`，并另外渲染出 `content_html` |
| `_posts` / `_drafts` | `status` = 已发布 / `DRAFT` |
| `date` | `created_at` |
| `cover` / `top_img` | `cover_url` / `top_img_url` |
| `categories` / `tags` | `categories` / `tags` |
| `excerpt` | `summaries` |

- **缺 `date` 的 4 篇**用该文件在 git 中首次提交的时间：claude与codex的搭配使用、关于使用codex的各种畅想、亚马逊云服务器配置、开发环境安装指南。
- **Hexo 标签语法**（`label` ×23、`span` ×7、`tabs` ×3、`btn` ×3，分布在 Java基础、MySQL索引详情、破解文件资源、nacos适配各数据库版本资源）：先转换为前台 `src/lib/marked-extensions.ts` 支持的语法，再**复用该渲染器**生成 `content_html`，保证与编辑器的输出一致。
- **迁移前修正**：`关于使用codex的各种畅想.md` 的标题与另一篇重复，会被导入器跳过，需要先改标题。
- 导入方式：后台「文章管理 → 导入」，勾选自动创建分类和标签。

### 4.2 其他内容

| 内容 | 处理方式 |
|---|---|
| 短文 `source/_data/essay.yml` | 脚本调用 `/api/essays` 批量写入，管理员凭证通过环境变量传入；那首 QQ 音乐的歌手动换成网易云 ID |
| 相册 `source/_data/album.yml` | 转换为 `/albums/import` 要求的格式 |
| 关于页 `about.yml` | 在后台手动重新录入 |
| 图片 | 都在 `img.thpvip.cn` 图床上，URL 不变，不需要搬 |
| 不迁移 | `interview-map`、`_posts/测试文章`、`_posts/绘图` |

### 4.3 验证

- 脚本输出统计：总数 26，失败 0
- 重点抽查：用了 Hexo 标签的 4 篇、代码最多的 JUC 并发编程
- 显示异常的文章：在编辑器中打开并重新保存，让系统重新生成 HTML

## 5. 品牌化改造

> 2026-10-11 已对照源码核实（`internal/configdef/definition.go` 中有几百个后台配置项），本节据此修订：品牌化以**配置**为主，代码改动只有两处。

**后台配置（不改代码）**：以下配置项的默认值都是原作者的信息，全部改成自己的：
- 站点：`APP_NAME`、副标题、关键词、描述、关于链接、ICP 备案号
- 视觉：Logo（通用 / 192 / 512 / 横向日夜两版）、favicon、默认头像、颜色令牌 `APPEARANCE_TOKENS`
- 布局：主菜单、导航下拉菜单、首页顶部 `HOME_TOP`、技能模块、一图流
- 页脚：拥有者、起始年份、社交栏、链接列表、徽标、底部栏链接、运行时间
- 前台站长信息、评论博主邮箱、各类邮件主题与模板

做法：把以上配置整理成 `tools/brand/settings.json`，通过后台「配置导入」（`/config/import`）一次导入。本地和新服务器复用同一份文件。

**后台数据清理（不改代码）**：后端初始化时会写入两个「安知鱼」默认友链，以及隐私政策页、版权页（正文里链接到 `/update`），在后台删除或改写。

**代码改动（只有两处）**：
- `src/utils/console-printer.ts`：控制台固定打印「欢迎使用安知鱼 / Powered by AnHeYu / 你正在访问安知鱼的博客」，改为读取站点配置中的名称
- `src/app/(frontend)/posts/[id]/page.tsx`、`src/app/(frontend)/doc/[id]/page.tsx`：`<meta name="author">` 在文章没有 `copyright_author` 时默认写「安知鱼」，改为默认取站点拥有者的名字（迁移过来的文章都会走到这个默认值）

**不改的**：前台其余写死的「AnHeYu / 安和鱼」都是配置为空时的兜底值，配好之后不会出现；后台管理界面里的占位文字只有管理员能看到。改它们只会增加和上游合并时的冲突。

**作者的私人页面** `/equipment`、`/air-conditioner`、`/update`：只从菜单里去掉入口，保留文件。

页脚**不保留**「Powered by AnHeYu」。

**保持原样**：主色调、动画、布局。

**合规**：源码中的 LICENSE 和版权声明照常保留（GPL 要求）。

**验证**：脚本扫描构建产物中用户可见的文本，确认没有「安知鱼 / 安和鱼 / AnHeYu / anheyu.com」残留（LICENSE 与源码注释除外）。

## 6. 部署与上线

**构建**：在 fork 中新增 GitHub Actions 工作流（不改上游的 `build.yml`）。推送 `custom` 分支或打 tag 时，构建 Next.js standalone 和 Go 二进制，使用官方 Dockerfile 的 `full` 目标打成镜像，推送到私有的 GHCR。

**新服务器**（建议 2GB 以上内存），Docker Compose 编排：

| 服务 | 说明 |
|---|---|
| `anheyu` | 后端 + 前台在同一个容器 |
| `postgres:17`、`redis` | 只在内网暴露，不开放公网端口 |
| `caddy` | 反向代理，自动签发和续期 HTTPS 证书 |

**发布**：服务器上执行 `deploy.sh`（`docker compose pull && docker compose up -d`）。不再需要 Webhook：内容在后台在线编辑，只有改代码才需要部署。

**备份**：数据库成为唯一的数据源。
- 每日 `pg_dump`，压缩后保留 14 天
- 同步一份到服务器以外的地方
- 上线前演练一次恢复

**切换步骤**：
1. 用临时子域名（例如 `new.thpvip.cn`）部署并验收
2. 把 `thpvip.cn` 的 DNS 切到新服务器
3. 旧服务器保留一周，之后下线 Hexo 和 Webhook

## 7. 上线验收清单

- [ ] 后台发布文章后，前台立即可见
- [ ] 短文增删改查正常，`/essay` 页面与首页 `EssayBar` 显示正确
- [ ] 迁移统计：26 篇，失败 0；抽查的文章渲染正确
- [ ] 品牌检查脚本没有残留
- [ ] 移动端显示正常
- [ ] 全站 HTTPS 生效
- [ ] 备份恢复演练成功

## 8. 风险与待确认事项

| 项 | 说明 / 应对 |
|---|---|
| PRO 功能的限制方式 | 尚未核实开源版中 PRO 功能是如何被限制的；本次不依赖任何 PRO 功能 |
| ~~后台可配置项的范围~~ | 已核实，见第 5 节 |
| 上游合并冲突 | 遵守「新增文件优先」原则，每次同步后跑一遍测试和验收清单 |
| ~~前台包管理器~~ | 已确认使用 npm（`package-lock.json`）；上游 `dev` 脚本缺少 `cross-env` 依赖，本地用替代命令启动 |
| 新服务器规格 | 由用户提供，建议内存 ≥ 2GB |
