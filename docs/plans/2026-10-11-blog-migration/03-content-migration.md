# 阶段 3：内容迁移工具

> **⛔ 已跳过（2026-10-12）**：用户决定不迁移旧 Hexo 内容，新站从零开始。本文件保留备查，**不要执行**。以后如果要重新迁移，先和用户确认，再按 README 的「计划变更记录」恢复阶段 4、5 里被删掉的导入步骤。

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 写一个可重复运行的迁移工具，把 hexo-blog 的 21 篇文章、5 篇草稿、`essay.yml`、`album.yml` 导入 anheyu-app；本地先跑通，阶段 5 原样对新服务器再跑一次。

**Architecture:** 工具放在**前台仓库**的 `tools/hexo-migrate/`（不是设计文档里写的后端仓库）：它要复用前台的 `src/lib/marked-extensions.ts` 来渲染 HTML，放在前台仓库里路径最短。工具有自己的 `package.json` 和 `node_modules`，**不改前台根目录的 `package.json`**。两个命令：
- `build`：只读 hexo-blog，生成 `out/articles.json`、`out/essays.json`、`out/albums.json`、`out/report.md`（纯本地，可反复跑）
- `apply`：登录后台，调用接口导入（文章和相册按上游导入接口去重；短文按「发布时间 + 内容」去重），重复运行不会产生重复数据

**Tech Stack:** Node 22、tsx、js-yaml 4.1.1、marked 17.0.1（和前台锁定的版本一致）、Node 自带测试框架 `node:test`。

**仓库：** `E:\workspace\code\blog\anheyu-app-frontend`，分支 `custom`；数据源 `E:\workspace\personal\code\hexo-blog\source`（只读，除了 Task M0）。

**已核实的上游接口：**

| 用途 | 接口 | 要点 |
|---|---|---|
| 登录 | `POST /api/auth/login` `{email,password}` | 返回 `data.accessToken`；之后请求头 `Authorization: Bearer <token>` |
| 文章导入 | `POST /api/articles/import`（multipart） | 字段 `file`、`create_categories`、`create_tags`、`skip_existing`、`default_status`；JSON 结构 `{version, export_at, articles:[...], meta}`；**按标题去重**；**不会**从 Markdown 生成 HTML |
| 相册分类 | `GET/POST /api/album-categories` | POST 体 `{name, description, displayOrder}` |
| 相册导入 | `POST /api/albums/import`（multipart） | 字段 `file`、`skip_existing`；JSON 结构 `{version, export_at, albums:[...], meta}`，按图片地址去重 |
| 短文 | `GET/POST /api/essays` | 阶段 1 新增 |

**Hexo 标签 → anheyu 语法（只处理实际用到的 4 种）：**

| Hexo | 转换为 | 出现位置 |
|---|---|---|
| `{% label 文字 %}`（可带颜色参数） | `==文字==`（渲染为 `<mark>`） | Java基础、MySQL索引详情 |
| `{% span red, 文字 %}` | `<span style="color: #f04a63">文字</span>`；未知颜色转 `**文字**` | Java基础 |
| `{% btn 'url',文字,图标 %}` | `{btn url="url" text="文字" icon="图标"}{/btn}` | 破解文件资源、nacos适配… |
| `{% tabs 名 %}` / `<!-- tab 标题 -->` / `<!-- endtab -->` / `{% endtabs %}` | `:::tabs` / `== tab 标题` / 删除 / `:::` | Java基础（3 组） |

其他 `{% ... %}` 一律原样保留并写进 `report.md` 的警告里。

---

### Task M0：修正重复标题（hexo-blog 仓库）

上游导入器按标题去重，`AI与效率工具/关于使用codex的各种畅想.md` 的标题和 `claude与codex的搭配使用.md` 相同，会被跳过。

**Step 1：** 先向用户确认新标题，默认建议「关于使用 Codex 的各种畅想」（取自文件名）。

**Step 2：** 修改 `E:\workspace\personal\code\hexo-blog\source\_posts\AI与效率工具\关于使用codex的各种畅想.md` 的 front matter `title:` 和 `excerpt:`（如果 excerpt 也是复制来的）。

**Step 3：** 不提交、不推送（hexo-blog 推送会触发线上部署）。告诉用户这个文件被改了，由用户决定是否提交。工具在 Task M4 里也会检测重复标题并报错，作为兜底。

---

### Task M1：工具脚手架

**Files:**
- Create: `tools/hexo-migrate/package.json`
- Create: `tools/hexo-migrate/tsconfig.json`
- Create: `tools/hexo-migrate/.gitignore`
- Create: `tools/hexo-migrate/src/render.ts`
- Test: `tools/hexo-migrate/src/render.test.ts`

**Step 1：`package.json`**

```json
{
  "name": "hexo-migrate",
  "private": true,
  "type": "module",
  "description": "[custom] 把 hexo-blog 内容迁移到 anheyu-app",
  "scripts": {
    "build-data": "tsx src/cli.ts build",
    "apply": "tsx src/cli.ts apply",
    "test": "tsx --test src/*.test.ts"
  },
  "dependencies": {
    "js-yaml": "4.1.1",
    "marked": "17.0.1"
  },
  "devDependencies": {
    "@types/js-yaml": "^4.0.9",
    "@types/node": "^22.0.0",
    "tsx": "^4.20.0"
  }
}
```

**Step 2：`tsconfig.json`**（`@/*` 指向前台的 `src/`，这样能直接 import `@/lib/marked-extensions`）

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "noEmit": true,
    "baseUrl": ".",
    "paths": { "@/*": ["../../src/*"] }
  },
  "include": ["src/**/*.ts"]
}
```

**Step 3：`.gitignore`**

```
node_modules/
out/
```

**Step 4：安装依赖**

Run: `cd tools/hexo-migrate && npm install`
Expected: 生成 `package-lock.json`，无报错。

**Step 5：写失败的测试** `src/render.test.ts`

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { renderMarkdown } from "./render.ts";

test("基础 Markdown", () => {
  assert.match(renderMarkdown("# 标题\n\n正文"), /<h1[^>]*>标题<\/h1>/);
});

test("高亮语法 ==x== 渲染为 mark", () => {
  assert.match(renderMarkdown("这是 ==重点== 内容"), /<mark>重点<\/mark>/);
});

test("tabs 容器渲染为标签页", () => {
  const html = renderMarkdown(":::tabs\n== tab 甲\n内容A\n== tab 乙\n内容B\n:::\n");
  assert.match(html, /class="tabs"/);
  assert.match(html, /data-title="甲"/);
  assert.match(html, /data-title="乙"/);
});

test("btn 行内标签渲染为按钮链接", () => {
  const html = renderMarkdown('{btn url="https://a.com/x.zip" text="下载" icon="anzhiyufont anzhiyu-icon-circle-arrow-right"}{/btn}');
  assert.match(html, /<a class="btn-anzhiyu" href="https:\/\/a.com\/x.zip"/);
  assert.match(html, /<span>下载<\/span>/);
});
```

**Step 6：运行，确认失败**

Run: `cd tools/hexo-migrate && npm test`
Expected: FAIL，找不到 `./render.ts`。

**Step 7：实现** `src/render.ts`

```ts
/**
 * 复用前台的 marked 扩展渲染 HTML，保证与后台编辑器的输出一致。
 * fixTaskListHtml / processHtmlForSave 在 Node 里没有 window 会原样返回，所以不需要调用。
 */
import { marked } from "marked";
import { registerMarkedExtensions } from "@/lib/marked-extensions";

registerMarkedExtensions(marked);

export function renderMarkdown(markdown: string): string {
  return marked.parse(markdown, { async: false }) as string;
}
```

**Step 8：运行，确认通过**

Run: `cd tools/hexo-migrate && npm test`
Expected: 4 个测试 `ok`。如果报 `@/lib/...` 解析失败，确认 tsx 读到了本目录的 `tsconfig.json`（在 `tools/hexo-migrate` 目录内运行）。

**Step 9：提交**

```bash
git add tools/hexo-migrate/package.json tools/hexo-migrate/package-lock.json tools/hexo-migrate/tsconfig.json tools/hexo-migrate/.gitignore tools/hexo-migrate/src/render.ts tools/hexo-migrate/src/render.test.ts
git commit -m "feat(migrate): 迁移工具脚手架，复用前台 Markdown 渲染器"
```

---

### Task M2：Front matter 与日期解析

Hexo 配置 `timezone: ''`，日期按机器本地时区（Asia/Shanghai）解释。js-yaml 默认会把 `2025-10-28 15:00:00` 当成 UTC，所以用 `JSON_SCHEMA` 让日期保持字符串，再按 +08:00 自己解析。

**Files:**
- Test: `tools/hexo-migrate/src/frontmatter.test.ts`
- Create: `tools/hexo-migrate/src/frontmatter.ts`

**Step 1：写失败的测试**

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { parseFrontMatter, parseHexoDate, toStringList } from "./frontmatter.ts";

test("拆分 front matter 和正文（兼容 CRLF）", () => {
  const { data, body } = parseFrontMatter("---\r\ntitle: 你好\r\ntags:\r\n  - Java\r\n---\r\n正文\r\n");
  assert.equal(data.title, "你好");
  assert.deepEqual(data.tags, ["Java"]);
  assert.equal(body.trim(), "正文");
});

test("没有 front matter 时整篇都是正文", () => {
  const { data, body } = parseFrontMatter("只有正文");
  assert.deepEqual(data, {});
  assert.equal(body, "只有正文");
});

test("日期保持字符串，不被 YAML 提前转成 UTC", () => {
  const { data } = parseFrontMatter("---\ndate: 2025-10-28 15:00:00\n---\n");
  assert.equal(data.date, "2025-10-28 15:00:00");
});

test("Hexo 日期按 +08:00 解析", () => {
  assert.equal(parseHexoDate("2025-10-28 15:00:00")?.toISOString(), "2025-10-28T07:00:00.000Z");
  assert.equal(parseHexoDate("2025/07/01 12:00:00")?.toISOString(), "2025-07-01T04:00:00.000Z");
  assert.equal(parseHexoDate("2025/09/09")?.toISOString(), "2025-09-08T16:00:00.000Z");
  assert.equal(parseHexoDate("2022-10-25")?.toISOString(), "2022-10-24T16:00:00.000Z");
});

test("无法识别的日期返回 null", () => {
  assert.equal(parseHexoDate(undefined), null);
  assert.equal(parseHexoDate(""), null);
  assert.equal(parseHexoDate("昨天"), null);
});

test("tags/categories 兼容字符串、数组、空值", () => {
  assert.deepEqual(toStringList(["a", " b ", "", null]), ["a", "b"]);
  assert.deepEqual(toStringList("c"), ["c"]);
  assert.deepEqual(toStringList(null), []);
});
```

**Step 2：运行，确认失败**

Run: `cd tools/hexo-migrate && npm test`
Expected: FAIL，找不到 `./frontmatter.ts`。

**Step 3：实现** `src/frontmatter.ts`

```ts
import yaml from "js-yaml";

export interface FrontMatter {
  data: Record<string, unknown>;
  body: string;
}

const FRONT_MATTER_RE = /^---\r?\n([\s\S]*?)\r?\n---\r?\n?/;

export function parseFrontMatter(raw: string): FrontMatter {
  const match = raw.match(FRONT_MATTER_RE);
  if (!match) return { data: {}, body: raw };
  const loaded = yaml.load(match[1], { schema: yaml.JSON_SCHEMA });
  const data = loaded && typeof loaded === "object" ? (loaded as Record<string, unknown>) : {};
  return { data, body: raw.slice(match[0].length) };
}

const HEXO_DATE_RE = /^(\d{4})[-/](\d{1,2})[-/](\d{1,2})(?:[ T](\d{1,2}):(\d{2})(?::(\d{2}))?)?$/;

/** 按 Asia/Shanghai（+08:00）解析 Hexo 日期；无法识别返回 null */
export function parseHexoDate(value: unknown): Date | null {
  if (typeof value !== "string") return null;
  const m = value.trim().match(HEXO_DATE_RE);
  if (!m) return null;
  const [, y, mo, d, h = "0", mi = "0", s = "0"] = m;
  const pad = (v: string) => v.padStart(2, "0");
  const date = new Date(`${y}-${pad(mo)}-${pad(d)}T${pad(h)}:${pad(mi)}:${pad(s)}+08:00`);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function toStringList(value: unknown): string[] {
  const list = Array.isArray(value) ? value : value == null ? [] : [value];
  return list
    .filter((item): item is string | number => typeof item === "string" || typeof item === "number")
    .map(item => String(item).trim())
    .filter(Boolean);
}
```

**Step 4：运行，确认通过**

Run: `cd tools/hexo-migrate && npm test`
Expected: 全部 `ok`。

**Step 5：提交**

```bash
git add tools/hexo-migrate/src/frontmatter.ts tools/hexo-migrate/src/frontmatter.test.ts
git commit -m "feat(migrate): 解析 Hexo front matter 与 +08:00 日期"
```

---

### Task M3：Hexo 标签转换

**Files:**
- Test: `tools/hexo-migrate/src/hexo-tags.test.ts`
- Create: `tools/hexo-migrate/src/hexo-tags.ts`

**Step 1：写失败的测试**

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { convertHexoTags } from "./hexo-tags.ts";

test("label → 高亮", () => {
  assert.equal(convertHexoTags("{% label 二叉树性质 %}").markdown, "==二叉树性质==");
  assert.equal(convertHexoTags("{% label 重点 blue %}").markdown, "==重点==");
});

test("span red → 红色文字；未知颜色 → 加粗", () => {
  assert.equal(
    convertHexoTags("{% span red, 数据域data %}").markdown,
    '<span style="color: #f04a63">数据域data</span>'
  );
  assert.equal(convertHexoTags("{% span shine, 闪 %}").markdown, "**闪**");
});

test("btn → 行内按钮", () => {
  const { markdown } = convertHexoTags(
    "{% btn 'http://img.thpvip.cn/hexo-blog/nacos.zip',nacos.zip,anzhiyufont anzhiyu-icon-circle-arrow-right %}"
  );
  assert.equal(
    markdown,
    '{btn url="http://img.thpvip.cn/hexo-blog/nacos.zip" text="nacos.zip" icon="anzhiyufont anzhiyu-icon-circle-arrow-right"}{/btn}'
  );
});

test("tabs 整组转换为 ::: 容器", () => {
  const input = [
    "{% tabs List %}",
    "",
    "<!-- tab ArrayList -->",
    "**数组**",
    "<!-- endtab -->",
    "<!-- tab LinkedList -->",
    "**链表**",
    "<!-- endtab -->",
    "{% endtabs %}",
  ].join("\n");
  const { markdown, warnings } = convertHexoTags(input);
  assert.equal(markdown, [":::tabs", "", "== tab ArrayList", "**数组**", "== tab LinkedList", "**链表**", ":::"].join("\n"));
  assert.deepEqual(warnings, []);
});

test("代码块里的标签不转换", () => {
  const input = "```\n{% label 不要动 %}\n```";
  assert.equal(convertHexoTags(input).markdown, input);
});

test("未知标签原样保留并告警", () => {
  const { markdown, warnings } = convertHexoTags("{% note info %}提示{% endnote %}");
  assert.equal(markdown, "{% note info %}提示{% endnote %}");
  assert.equal(warnings.length, 1);
  assert.match(warnings[0], /note/);
});
```

**Step 2：运行，确认失败**

Run: `cd tools/hexo-migrate && npm test`
Expected: FAIL，找不到 `./hexo-tags.ts`。

**Step 3：实现** `src/hexo-tags.ts`

```ts
/**
 * 把 hexo-blog 实际用到的 AnZhiYu 标签转换为 anheyu 前台 marked 扩展支持的语法。
 * 代码块（``` 或 ~~~ 围起来的部分）内不做转换。
 */

const SPAN_COLORS: Record<string, string> = {
  red: "#f04a63",
  blue: "#425aef",
  green: "#57bd6a",
};

const LABEL_COLORS = new Set(["default", "blue", "pink", "red", "purple", "orange", "green"]);

export interface ConvertResult {
  markdown: string;
  warnings: string[];
}

function convertLine(line: string): string {
  return line
    .replace(/^\s*\{%\s*tabs\b[^%]*%\}\s*$/, ":::tabs")
    .replace(/^\s*\{%\s*endtabs\s*%\}\s*$/, ":::")
    .replace(/^\s*<!--\s*tab\s+(.+?)\s*-->\s*$/, (_, title: string) => `== tab ${title}`)
    .replace(/\{%\s*label\s+([^%]+?)\s*%\}/g, (_, args: string) => {
      const parts = args.trim().split(/\s+/);
      if (parts.length > 1 && LABEL_COLORS.has(parts[parts.length - 1])) parts.pop();
      return `==${parts.join(" ")}==`;
    })
    .replace(/\{%\s*span\s+([^,%]+),\s*([^%]+?)\s*%\}/g, (_, style: string, text: string) => {
      const color = SPAN_COLORS[style.trim().split(/\s+/)[0]];
      return color ? `<span style="color: ${color}">${text}</span>` : `**${text}**`;
    })
    .replace(/\{%\s*btn\s+([^%]+?)\s*%\}/g, (_, args: string) => {
      const [url = "", text = "", icon = ""] = args.split(",").map(s => s.trim().replace(/^['"]|['"]$/g, ""));
      return `{btn url="${url}" text="${text}" icon="${icon}"}{/btn}`;
    });
}

export function convertHexoTags(markdown: string): ConvertResult {
  const warnings: string[] = [];
  let fence: string | null = null;
  const out: string[] = [];

  for (const line of markdown.split("\n")) {
    const fenceMatch = line.trim().match(/^(`{3,}|~{3,})/);
    if (fenceMatch) {
      if (fence === null) fence = fenceMatch[1];
      else if (line.trim().startsWith(fence)) fence = null;
      out.push(line);
      continue;
    }
    if (fence !== null) {
      out.push(line);
      continue;
    }
    if (/^\s*<!--\s*endtab\s*-->\s*$/.test(line)) continue;

    const converted = convertLine(line);
    const leftover = converted.match(/\{%\s*(\w+)/);
    if (leftover) warnings.push(`未转换的标签 {% ${leftover[1]} %}：${line.trim()}`);
    out.push(converted);
  }
  return { markdown: out.join("\n"), warnings };
}
```

**Step 4：运行，确认通过**

Run: `cd tools/hexo-migrate && npm test`
Expected: 全部 `ok`。

**Step 5：提交**

```bash
git add tools/hexo-migrate/src/hexo-tags.ts tools/hexo-migrate/src/hexo-tags.test.ts
git commit -m "feat(migrate): 转换 Hexo label/span/btn/tabs 标签"
```

---

### Task M4：文章映射

**Files:**
- Test: `tools/hexo-migrate/src/articles.test.ts`
- Create: `tools/hexo-migrate/src/articles.ts`

规则：
- `_posts/**/*.md` → `PUBLISHED`；`_drafts/*.md` → `DRAFT`
- `created_at`：front matter `date`；没有时用该文件在 git 里**第一次提交**的时间；都没有时报错（不静默用当前时间）
- `summaries`：`excerpt` 非空时为 `[excerpt]`
- 图片域名改写（可选）：传了 `imageHost` 时把 `http://img.thpvip.cn` 改成它（用于阶段 5 的 HTTPS 方案）
- 标题去重：有重复标题时直接报错，列出文件
- 正文先过 `convertHexoTags`，再用 `renderMarkdown` 生成 `content_html`；`content_md` 存转换后的 Markdown（编辑器以后打开看到的是新语法）

**Step 1：写失败的测试**

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildArticleData, rewriteImageHost, type SourcePost } from "./articles.ts";

const post = (over: Partial<SourcePost>): SourcePost => ({
  file: "_posts/分类/文章.md",
  raw: "---\ntitle: 文章\ndate: 2025-10-28 15:00:00\ncategories:\n  - 核心原理与面试\ntags:\n  - Java\nexcerpt: 摘要\ncover: http://img.thpvip.cn/a.png\n---\n正文 {% label 重点 %}\n",
  draft: false,
  gitCreatedAt: null,
  ...over,
});

test("映射已发布文章的全部字段", () => {
  const { data, warnings } = buildArticleData([post({})], {});
  const a = data.articles[0];
  assert.equal(a.title, "文章");
  assert.equal(a.status, "PUBLISHED");
  assert.equal(a.created_at, "2025-10-28T07:00:00.000Z");
  assert.deepEqual(a.categories, ["核心原理与面试"]);
  assert.deepEqual(a.tags, ["Java"]);
  assert.deepEqual(a.summaries, ["摘要"]);
  assert.equal(a.cover_url, "http://img.thpvip.cn/a.png");
  assert.match(a.content_md, /==重点==/);
  assert.match(a.content_html, /<mark>重点<\/mark>/);
  assert.equal(a.copyright, true);
  assert.deepEqual(warnings, []);
});

test("草稿和缺日期时用 git 首次提交时间", () => {
  const raw = "---\ntitle: 草稿\n---\n内容\n";
  const { data } = buildArticleData([post({ raw, draft: true, gitCreatedAt: "2026-04-14T10:00:00+08:00" })], {});
  assert.equal(data.articles[0].status, "DRAFT");
  assert.equal(data.articles[0].created_at, "2026-04-14T02:00:00.000Z");
});

test("既没有 date 也没有 git 时间时报错", () => {
  assert.throws(() => buildArticleData([post({ raw: "---\ntitle: x\n---\n" })], {}), /缺少发布时间/);
});

test("重复标题报错并列出文件", () => {
  assert.throws(
    () => buildArticleData([post({ file: "a.md" }), post({ file: "b.md" })], {}),
    /重复的标题.*a\.md.*b\.md/s
  );
});

test("图片域名改写", () => {
  assert.equal(
    rewriteImageHost("![](http://img.thpvip.cn/a.png) http://img.thpvip.cn/b", "https://img.thpvip.cn"),
    "![](https://img.thpvip.cn/a.png) https://img.thpvip.cn/b"
  );
  const { data } = buildArticleData([post({})], { imageHost: "https://img.thpvip.cn" });
  assert.equal(data.articles[0].cover_url, "https://img.thpvip.cn/a.png");
});

test("Hexo 标签告警带上文件名", () => {
  const raw = "---\ntitle: x\ndate: 2025-01-01\n---\n{% note %}\n";
  const { warnings } = buildArticleData([post({ raw })], {});
  assert.match(warnings[0], /_posts\/分类\/文章\.md/);
});
```

**Step 2：运行，确认失败**

Run: `cd tools/hexo-migrate && npm test`
Expected: FAIL，找不到 `./articles.ts`。

**Step 3：实现** `src/articles.ts`

```ts
import { parseFrontMatter, parseHexoDate, toStringList } from "./frontmatter.ts";
import { convertHexoTags } from "./hexo-tags.ts";
import { renderMarkdown } from "./render.ts";

export const OLD_IMAGE_HOST = "http://img.thpvip.cn";

export interface SourcePost {
  /** 相对 hexo source 目录的路径，用于报错和告警 */
  file: string;
  raw: string;
  draft: boolean;
  /** git 首次提交时间（ISO），没有则为 null */
  gitCreatedAt: string | null;
}

export interface BuildOptions {
  imageHost?: string;
}

/** 与上游 pkg/service/article/import_export_service.go 的 ExportArticleItem 对齐（只填需要的字段） */
export interface ArticleItem {
  title: string;
  content_md: string;
  content_html: string;
  status: "PUBLISHED" | "DRAFT";
  created_at: string;
  updated_at: string;
  cover_url?: string;
  top_img_url?: string;
  categories: string[];
  tags: string[];
  summaries?: string[];
  home_sort: number;
  pin_sort: number;
  copyright: boolean;
  is_reprint: boolean;
  is_primary_color_manual: boolean;
  view_count: number;
  word_count: number;
  reading_time: number;
}

export interface ArticleData {
  version: string;
  export_at: string;
  articles: ArticleItem[];
  meta: Record<string, unknown>;
}

export function rewriteImageHost(text: string, imageHost?: string): string {
  return imageHost ? text.split(OLD_IMAGE_HOST).join(imageHost) : text;
}

function optionalString(value: unknown, imageHost?: string): string | undefined {
  const s = typeof value === "string" ? value.trim() : "";
  return s ? rewriteImageHost(s, imageHost) : undefined;
}

export function buildArticleData(posts: SourcePost[], options: BuildOptions): { data: ArticleData; warnings: string[] } {
  const warnings: string[] = [];
  const titles = new Map<string, string[]>();

  const articles = posts.map(post => {
    const { data, body } = parseFrontMatter(post.raw);
    const title = typeof data.title === "string" && data.title.trim() ? data.title.trim() : post.file;
    titles.set(title, [...(titles.get(title) ?? []), post.file]);

    const createdAt = parseHexoDate(data.date) ?? (post.gitCreatedAt ? new Date(post.gitCreatedAt) : null);
    if (!createdAt) throw new Error(`${post.file} 缺少发布时间：front matter 没有 date，git 里也查不到首次提交`);

    const converted = convertHexoTags(rewriteImageHost(body, options.imageHost));
    warnings.push(...converted.warnings.map(w => `${post.file}: ${w}`));

    const excerpt = optionalString(data.excerpt);
    return {
      title,
      content_md: converted.markdown,
      content_html: renderMarkdown(converted.markdown),
      status: post.draft ? "DRAFT" : "PUBLISHED",
      created_at: createdAt.toISOString(),
      updated_at: createdAt.toISOString(),
      cover_url: optionalString(data.cover, options.imageHost),
      top_img_url: optionalString(data.top_img, options.imageHost),
      categories: toStringList(data.categories),
      tags: toStringList(data.tags),
      summaries: excerpt ? [excerpt] : undefined,
      home_sort: 0,
      pin_sort: 0,
      copyright: true,
      is_reprint: false,
      is_primary_color_manual: false,
      view_count: 0,
      word_count: 0,
      reading_time: 0,
    } satisfies ArticleItem;
  });

  const duplicates = [...titles.entries()].filter(([, files]) => files.length > 1);
  if (duplicates.length > 0) {
    const detail = duplicates.map(([t, files]) => `「${t}」: ${files.join(", ")}`).join("\n");
    throw new Error(`存在重复的标题，上游导入器会跳过后者，请先改标题：\n${detail}`);
  }

  return {
    data: { version: "1.0", export_at: new Date().toISOString(), articles, meta: { source: "hexo-blog" } },
    warnings,
  };
}
```

> `word_count` / `reading_time` 填 0：先确认上游导入时会不会自动计算（看 `ImportArticles` 里创建文章时是否调用了字数统计）。不会的话，导入后在后台打开文章保存一次即可，在 Task M8 的验收里检查。

**Step 4：运行，确认通过**

Run: `cd tools/hexo-migrate && npm test`
Expected: 全部 `ok`。

**Step 5：提交**

```bash
git add tools/hexo-migrate/src/articles.ts tools/hexo-migrate/src/articles.test.ts
git commit -m "feat(migrate): Hexo 文章映射为 anheyu 导入格式"
```

---

### Task M5：短文与相册映射

**Files:**
- Test: `tools/hexo-migrate/src/essays.test.ts`、`tools/hexo-migrate/src/albums.test.ts`
- Create: `tools/hexo-migrate/src/essays.ts`、`tools/hexo-migrate/src/albums.ts`

`essay.yml` 实际用到的字段：`content`、`date`、`image[]`、`video[]`、`address`、`from`、`link`、`aplayer{server,id}`。
- `link` 指向 `localhost` 的丢弃并告警（有一条是 `https://localhost:4000/music/...`）
- `aplayer` 是 QQ 音乐 ID，无法自动转网易云：`music_id` 留空并告警，迁移后在后台手动补

`album.yml`：每个分类 `class_name`/`description`，`album_list[]` 每项 `date`/`content`/`address`/`image[]`；每张图片导入为一条相册记录。

**Step 1：写失败的测试**

`src/essays.test.ts`：

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildEssayPayloads } from "./essays.ts";

const yml = `
- title: 即刻短文
  essay_list:
    - content: 沙茶面一般般
      date: 2025/07/01
      image:
        - http://img.thpvip.cn/a.jpg
      address: 厦门
      from: super-thp
    - content: 喵桑
      date: 2025/06/02
      link: https://localhost:4000/music/?id=1
    - content: 歌曲推荐
      date: 2022/09/25
      aplayer:
        server: tencent
        id: 001FGQba3i10mw
    - content: 海滩
      date: 2025/07/02
      video:
        - http://img.thpvip.cn/b.mp4
`;

test("映射字段并按 +08:00 解析日期", () => {
  const { payloads } = buildEssayPayloads(yml, {});
  assert.equal(payloads.length, 4);
  assert.deepEqual(payloads[0], {
    content: "沙茶面一般般",
    published_at: "2025-06-30T16:00:00.000Z",
    images: ["http://img.thpvip.cn/a.jpg"],
    videos: [],
    address: "厦门",
    source: "super-thp",
    link_url: "",
    link_title: "",
    music_id: "",
    music_name: "",
    music_artist: "",
    music_cover: "",
    is_published: true,
  });
  assert.deepEqual(payloads[3].videos, ["http://img.thpvip.cn/b.mp4"]);
});

test("丢弃 localhost 链接、QQ 音乐只告警", () => {
  const { payloads, warnings } = buildEssayPayloads(yml, {});
  assert.equal(payloads[1].link_url, "");
  assert.equal(payloads[2].music_id, "");
  assert.equal(warnings.length, 2);
  assert.match(warnings.join("\n"), /localhost/);
  assert.match(warnings.join("\n"), /001FGQba3i10mw/);
});

test("图片域名改写", () => {
  const { payloads } = buildEssayPayloads(yml, { imageHost: "https://img.thpvip.cn" });
  assert.deepEqual(payloads[0].images, ["https://img.thpvip.cn/a.jpg"]);
});
```

`src/albums.test.ts`：

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { buildAlbumPlan } from "./albums.ts";

const yml = `
- class_name: 世界各地夕阳与风景
  description: 分享各地的夕阳与风景🌇。
  album_list:
    - date: 2025/07/01 12:00:00
      content: 厦门行
      address: 厦门
      image:
        - http://img.thpvip.cn/1.jpg
        - http://img.thpvip.cn/2.jpg
- class_name: 喵桑的日常
  album_list:
    - date: 2025/10/17 23:01:00
      content: 喵
      image:
        - http://img.thpvip.cn/3.jpg
`;

test("按分类分组，每张图一条记录", () => {
  const plan = buildAlbumPlan(yml, {});
  assert.deepEqual(
    plan.categories.map(c => c.name),
    ["世界各地夕阳与风景", "喵桑的日常"]
  );
  assert.equal(plan.categories[0].description, "分享各地的夕阳与风景🌇。");
  assert.equal(plan.categories[0].items.length, 2);
  assert.deepEqual(plan.categories[0].items[1], {
    image_url: "http://img.thpvip.cn/2.jpg",
    big_image_url: "http://img.thpvip.cn/2.jpg",
    title: "厦门行",
    description: "",
    location: "厦门",
    display_order: 1,
    published_at: "2025-07-01T04:00:00.000Z",
    created_at: "2025-07-01T04:00:00.000Z",
    updated_at: "2025-07-01T04:00:00.000Z",
  });
  assert.equal(plan.categories[1].items[0].display_order, 0);
});
```

**Step 2：运行，确认失败**

Run: `cd tools/hexo-migrate && npm test`
Expected: FAIL，找不到 `./essays.ts`、`./albums.ts`。

**Step 3：实现**

`src/essays.ts`：

```ts
import yaml from "js-yaml";
import { parseHexoDate, toStringList } from "./frontmatter.ts";
import { rewriteImageHost, type BuildOptions } from "./articles.ts";

/** 与阶段 1 后端 model.EssayRequest 对齐 */
export interface EssayPayload {
  content: string;
  published_at: string;
  images: string[];
  videos: string[];
  address: string;
  source: string;
  link_url: string;
  link_title: string;
  music_id: string;
  music_name: string;
  music_artist: string;
  music_cover: string;
  is_published: boolean;
}

type Raw = Record<string, unknown>;

const str = (v: unknown) => (typeof v === "string" ? v.trim() : "");

export function buildEssayPayloads(ymlText: string, options: BuildOptions): { payloads: EssayPayload[]; warnings: string[] } {
  const doc = yaml.load(ymlText, { schema: yaml.JSON_SCHEMA }) as Raw[] | null;
  const list = ((doc?.[0]?.essay_list as Raw[] | undefined) ?? []);
  const warnings: string[] = [];

  const payloads = list.map((item, i) => {
    const label = `essay.yml 第 ${i + 1} 条（${str(item.content).slice(0, 12)}）`;
    const publishedAt = parseHexoDate(item.date);
    if (!publishedAt) throw new Error(`${label} 的 date 无法解析：${String(item.date)}`);

    let link = str(item.link);
    if (link && /^https?:\/\/(localhost|127\.0\.0\.1)/.test(link)) {
      warnings.push(`${label}: 丢弃本地链接 ${link}`);
      link = "";
    }
    const aplayer = item.aplayer as Raw | undefined;
    if (aplayer) {
      warnings.push(`${label}: 原音乐是 ${str(aplayer.server)} 的歌曲 ${str(aplayer.id)}，需要在后台手动填网易云歌曲 ID`);
    }

    return {
      content: str(item.content),
      published_at: publishedAt.toISOString(),
      images: toStringList(item.image).map(u => rewriteImageHost(u, options.imageHost)),
      videos: toStringList(item.video).map(u => rewriteImageHost(u, options.imageHost)),
      address: str(item.address),
      source: str(item.from),
      link_url: link,
      link_title: "",
      music_id: "",
      music_name: "",
      music_artist: "",
      music_cover: "",
      is_published: true,
    };
  });
  return { payloads, warnings };
}
```

`src/albums.ts`：

```ts
import yaml from "js-yaml";
import { parseHexoDate, toStringList } from "./frontmatter.ts";
import { rewriteImageHost, type BuildOptions } from "./articles.ts";

/** 与上游 pkg/service/album 的 ExportAlbumItem 对齐（只填需要的字段，category_id 在 apply 时补） */
export interface AlbumItem {
  image_url: string;
  big_image_url: string;
  title: string;
  description: string;
  location: string;
  display_order: number;
  published_at: string;
  created_at: string;
  updated_at: string;
}

export interface AlbumCategoryPlan {
  name: string;
  description: string;
  items: AlbumItem[];
}

type Raw = Record<string, unknown>;
const str = (v: unknown) => (typeof v === "string" ? v.trim() : "");

export function buildAlbumPlan(ymlText: string, options: BuildOptions): { categories: AlbumCategoryPlan[] } {
  const doc = (yaml.load(ymlText, { schema: yaml.JSON_SCHEMA }) as Raw[] | null) ?? [];
  const categories = doc.map(cat => {
    const items: AlbumItem[] = [];
    for (const entry of (cat.album_list as Raw[] | undefined) ?? []) {
      const at = parseHexoDate(entry.date);
      if (!at) throw new Error(`album.yml「${str(cat.class_name)}」中的日期无法解析：${String(entry.date)}`);
      for (const url of toStringList(entry.image)) {
        const imageUrl = rewriteImageHost(url, options.imageHost);
        items.push({
          image_url: imageUrl,
          big_image_url: imageUrl,
          title: str(entry.content),
          description: "",
          location: str(entry.address),
          display_order: items.length,
          published_at: at.toISOString(),
          created_at: at.toISOString(),
          updated_at: at.toISOString(),
        });
      }
    }
    return { name: str(cat.class_name), description: str(cat.description), items };
  });
  return { categories };
}
```

**Step 4：运行，确认通过**

Run: `cd tools/hexo-migrate && npm test`
Expected: 全部 `ok`。

**Step 5：提交**

```bash
git add tools/hexo-migrate/src/essays.ts tools/hexo-migrate/src/essays.test.ts tools/hexo-migrate/src/albums.ts tools/hexo-migrate/src/albums.test.ts
git commit -m "feat(migrate): 短文与相册映射"
```

---

### Task M6：`build` 命令（读文件、查 git 时间、写输出和报告）

**Files:**
- Create: `tools/hexo-migrate/src/source.ts`
- Create: `tools/hexo-migrate/src/cli.ts`

这一层是文件系统和子进程的胶水代码，不写单测，用真实数据跑一遍验证。

**Step 1：读取 hexo 数据** `src/source.ts`

```ts
import { execFileSync } from "node:child_process";
import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import type { SourcePost } from "./articles.ts";

function walkMarkdown(dir: string): string[] {
  return readdirSync(dir).flatMap(name => {
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) return walkMarkdown(full);
    return name.endsWith(".md") ? [full] : [];
  });
}

/** 文件在 git 里第一次被添加的时间（ISO），查不到返回 null */
function gitCreatedAt(repoRoot: string, file: string): string | null {
  try {
    const out = execFileSync("git", ["log", "--diff-filter=A", "--follow", "--format=%aI", "--", file], {
      cwd: repoRoot,
      encoding: "utf8",
    });
    const lines = out.trim().split("\n").filter(Boolean);
    return lines.length > 0 ? lines[lines.length - 1] : null;
  } catch {
    return null;
  }
}

export function loadPosts(hexoSourceDir: string): SourcePost[] {
  const repoRoot = path.resolve(hexoSourceDir, "..");
  const collect = (sub: string, draft: boolean) =>
    walkMarkdown(path.join(hexoSourceDir, sub)).map(full => ({
      file: path.relative(hexoSourceDir, full).split(path.sep).join("/"),
      raw: readFileSync(full, "utf8"),
      draft,
      gitCreatedAt: gitCreatedAt(repoRoot, full),
    }));
  return [...collect("_posts", false), ...collect("_drafts", true)];
}

export function loadDataFile(hexoSourceDir: string, name: string): string {
  return readFileSync(path.join(hexoSourceDir, "_data", name), "utf8");
}
```

**Step 2：命令入口** `src/cli.ts`（先只实现 `build`，`apply` 在 Task M7 补上）

```ts
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";
import { parseArgs } from "node:util";
import { buildArticleData, type ArticleData } from "./articles.ts";
import { buildAlbumPlan, type AlbumCategoryPlan } from "./albums.ts";
import { buildEssayPayloads, type EssayPayload } from "./essays.ts";
import { loadDataFile, loadPosts } from "./source.ts";

const { positionals, values } = parseArgs({
  allowPositionals: true,
  options: {
    hexo: { type: "string", default: "E:/workspace/personal/code/hexo-blog/source" },
    out: { type: "string", default: "out" },
    "image-host": { type: "string" },
  },
});

function build() {
  const options = { imageHost: values["image-host"] };
  const hexo = values.hexo!;
  const posts = loadPosts(hexo);
  const articles = buildArticleData(posts, options);
  const essays = buildEssayPayloads(loadDataFile(hexo, "essay.yml"), options);
  const albums = buildAlbumPlan(loadDataFile(hexo, "album.yml"), options);

  const out = values.out!;
  mkdirSync(out, { recursive: true });
  writeFileSync(path.join(out, "articles.json"), JSON.stringify(articles.data, null, 2));
  writeFileSync(path.join(out, "essays.json"), JSON.stringify(essays.payloads, null, 2));
  writeFileSync(path.join(out, "albums.json"), JSON.stringify(albums.categories, null, 2));

  const published = articles.data.articles.filter(a => a.status === "PUBLISHED").length;
  const drafts = articles.data.articles.length - published;
  const imageCount = albums.categories.reduce((n, c) => n + c.items.length, 0);
  const warnings = [...articles.warnings, ...essays.warnings];
  const report = [
    "# 迁移数据报告",
    "",
    `- 文章：${published} 篇已发布，${drafts} 篇草稿`,
    `- 短文：${essays.payloads.length} 条`,
    `- 相册：${albums.categories.length} 个分类，${imageCount} 张图片`,
    `- 图片域名改写：${options.imageHost ?? "未改写"}`,
    "",
    `## 警告（${warnings.length}）`,
    "",
    ...warnings.map(w => `- ${w}`),
    "",
  ].join("\n");
  writeFileSync(path.join(out, "report.md"), report);
  console.log(report);
}

export function readOut<T>(name: string): T {
  return JSON.parse(readFileSync(path.join(values.out!, name), "utf8")) as T;
}
export type { ArticleData, AlbumCategoryPlan, EssayPayload };

const command = positionals[0];
if (command === "build") {
  build();
} else {
  console.error("用法: tsx src/cli.ts build [--hexo <source目录>] [--out out] [--image-host https://img.thpvip.cn]");
  process.exit(1);
}
```

**Step 3：用真实数据运行**

Run: `cd tools/hexo-migrate && npm run build-data`
Expected（数字以实际为准，和这里不一致就停下来查）：

```
- 文章：21 篇已发布，5 篇草稿
- 短文：<essay.yml 的条数> 条
- 相册：3 个分类，<图片数> 张图片
- 图片域名改写：未改写
## 警告（2）
- essay.yml 第 x 条（…）: 丢弃本地链接 https://localhost:4000/...
- essay.yml 第 x 条（歌曲推荐）: 原音乐是 tencent 的歌曲 001FGQba3i10mw，需要在后台手动填网易云歌曲 ID
```

如果 Task M0 还没做，会报「存在重复的标题」——这是预期的兜底，先完成 M0。

**Step 4：抽查输出**

- `out/articles.json` 里找「Java基础」：`content_html` 含 `class="tabs"`（3 处）、`<mark>`、`color: #f04a63`；`content_md` 里不再有 `{%`
- 找「nacos适配各数据库版本资源」：`content_html` 含 `btn-anzhiyu`
- 原来缺 `date` 的 4 篇：`created_at` 是 2025–2026 年的合理时间，不是今天

**Step 5：提交**

```bash
git add tools/hexo-migrate/src/source.ts tools/hexo-migrate/src/cli.ts
git commit -m "feat(migrate): build 命令生成导入数据与报告"
```

---

### Task M7：`apply` 命令（调用接口导入）

**Files:**
- Test: `tools/hexo-migrate/src/api.test.ts`
- Create: `tools/hexo-migrate/src/api.ts`
- Create: `tools/hexo-migrate/src/apply.ts`
- Modify: `tools/hexo-migrate/src/cli.ts`（加 `apply` 分支）

**凭证约定：** 管理员邮箱和密码只从环境变量 `ANHEYU_EMAIL`、`ANHEYU_PASSWORD` 读取，由用户在自己的终端里设置；工具不打印、不落盘。目标地址用 `--base`（默认 `http://localhost:8091`）。

**Step 1：写失败的测试**（只测纯逻辑：响应解包和短文去重键）

```ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { essayKey, unwrap } from "./api.ts";

test("unwrap 成功时返回 data", () => {
  assert.deepEqual(unwrap({ code: 200, message: "ok", data: { a: 1 } }, "x"), { a: 1 });
});

test("unwrap 失败时抛出带上下文的错误", () => {
  assert.throws(() => unwrap({ code: 401, message: "未授权", data: null }, "登录"), /登录失败：401 未授权/);
});

test("短文去重键：同一时刻 + 同内容视为同一条，时间格式不同也能对上", () => {
  assert.equal(
    essayKey({ published_at: "2025-06-30T16:00:00.000Z", content: " 海边 " }),
    essayKey({ published_at: "2025-07-01T00:00:00+08:00", content: "海边" })
  );
  assert.notEqual(
    essayKey({ published_at: "2025-06-30T16:00:00Z", content: "海边" }),
    essayKey({ published_at: "2025-06-30T16:00:00Z", content: "山里" })
  );
});
```

**Step 2：运行，确认失败**

Run: `cd tools/hexo-migrate && npm test`
Expected: FAIL，找不到 `./api.ts`。

**Step 3：实现接口客户端** `src/api.ts`

```ts
export interface ApiResponse<T> {
  code: number;
  message: string;
  data: T;
}

export function unwrap<T>(resp: ApiResponse<T>, action: string): T {
  if (resp.code !== 200) throw new Error(`${action}失败：${resp.code} ${resp.message}`);
  return resp.data;
}

export function essayKey(e: { published_at: string; content: string }): string {
  return `${new Date(e.published_at).getTime()}|${e.content.trim()}`;
}

export class AnheyuClient {
  private token = "";

  constructor(private readonly base: string) {}

  async login(email: string, password: string): Promise<void> {
    const data = await this.request<{ accessToken: string }>("POST", "/api/auth/login", { email, password }, "登录");
    this.token = data.accessToken;
  }

  async request<T>(method: string, path: string, body: unknown, action: string): Promise<T> {
    const res = await fetch(`${this.base}${path}`, {
      method,
      headers: {
        "Content-Type": "application/json",
        ...(this.token ? { Authorization: `Bearer ${this.token}` } : {}),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    return unwrap((await res.json()) as ApiResponse<T>, action);
  }

  async upload<T>(path: string, fileName: string, json: unknown, fields: Record<string, string>, action: string): Promise<T> {
    const form = new FormData();
    form.append("file", new Blob([JSON.stringify(json)], { type: "application/json" }), fileName);
    for (const [k, v] of Object.entries(fields)) form.append(k, v);
    const res = await fetch(`${this.base}${path}`, {
      method: "POST",
      headers: { Authorization: `Bearer ${this.token}` },
      body: form,
    });
    return unwrap((await res.json()) as ApiResponse<T>, action);
  }
}
```

**Step 4：实现导入流程** `src/apply.ts`

```ts
import { AnheyuClient, essayKey } from "./api.ts";
import type { ArticleData } from "./articles.ts";
import type { AlbumCategoryPlan } from "./albums.ts";
import type { EssayPayload } from "./essays.ts";

interface ImportResult {
  total_count: number;
  success_count: number;
  skipped_count: number;
  failed_count: number;
  errors: string[] | null;
}

function printResult(label: string, r: ImportResult) {
  console.log(`${label}：共 ${r.total_count}，成功 ${r.success_count}，跳过 ${r.skipped_count}，失败 ${r.failed_count}`);
  for (const e of r.errors ?? []) console.log(`  - ${e}`);
}

export async function applyAll(
  client: AnheyuClient,
  input: { articles: ArticleData; essays: EssayPayload[]; albums: AlbumCategoryPlan[] }
): Promise<void> {
  // 1. 文章（上游按标题去重）
  const articleResult = await client.upload<ImportResult>(
    "/api/articles/import",
    "articles.json",
    input.articles,
    { create_categories: "true", create_tags: "true", skip_existing: "true", default_status: "DRAFT" },
    "导入文章"
  );
  printResult("文章", articleResult);

  // 2. 短文（按 发布时间+内容 去重）
  const existing = await client.request<{ list: EssayPayload[] }>("GET", "/api/essays?page=1&pageSize=100", undefined, "查询短文");
  const seen = new Set(existing.list.map(essayKey));
  let created = 0;
  for (const essay of input.essays) {
    if (seen.has(essayKey(essay))) continue;
    await client.request("POST", "/api/essays", essay, `创建短文「${essay.content.slice(0, 12)}」`);
    created++;
  }
  console.log(`短文：共 ${input.essays.length}，新建 ${created}，跳过 ${input.essays.length - created}`);

  // 3. 相册：先确保分类存在，再按分类导入（上游按图片地址去重）
  const categories = await client.request<{ id: number; name: string }[]>("GET", "/api/album-categories", undefined, "查询相册分类");
  for (const [order, cat] of input.albums.entries()) {
    let found = categories.find(c => c.name === cat.name);
    if (!found) {
      found = await client.request<{ id: number; name: string }>(
        "POST",
        "/api/album-categories",
        { name: cat.name, description: cat.description, displayOrder: order },
        `创建相册分类「${cat.name}」`
      );
    }
    const albums = cat.items.map(item => ({ ...item, category_id: found!.id }));
    const result = await client.upload<ImportResult>(
      "/api/albums/import",
      "albums.json",
      { version: "1.0", export_at: new Date().toISOString(), albums, meta: { source: "hexo-blog" } },
      { skip_existing: "true" },
      `导入相册「${cat.name}」`
    );
    printResult(`相册「${cat.name}」`, result);
  }
}
```

> 两个需要在执行时核对的点（核对后按实际情况调整，不要猜）：
> 1. `GET /api/album-categories` 的 `data` 是数组还是 `{list: [...]}`：先用 curl 带 token 调一次看。
> 2. `GET /api/essays` 一页最多 100 条（service 的 `maxPageSize`），现有短文超过 100 条时要翻页；迁移场景下不会超过。

**Step 5：在 `src/cli.ts` 加 `apply` 分支**

在 `parseArgs` 的 `options` 里加：

```ts
    base: { type: "string", default: "http://localhost:8091" },
```

文件顶部 import 加：

```ts
import { AnheyuClient } from "./api.ts";
import { applyAll } from "./apply.ts";
```

把底部的命令分发改成：

```ts
async function apply() {
  const email = process.env.ANHEYU_EMAIL;
  const password = process.env.ANHEYU_PASSWORD;
  if (!email || !password) {
    console.error("请先在终端里设置环境变量 ANHEYU_EMAIL 和 ANHEYU_PASSWORD（管理员账号）");
    process.exit(1);
  }
  const client = new AnheyuClient(values.base!);
  await client.login(email, password);
  await applyAll(client, {
    articles: readOut<ArticleData>("articles.json"),
    essays: readOut<EssayPayload[]>("essays.json"),
    albums: readOut<AlbumCategoryPlan[]>("albums.json"),
  });
}

const command = positionals[0];
if (command === "build") {
  build();
} else if (command === "apply") {
  apply().catch(err => {
    console.error(err instanceof Error ? err.message : err);
    process.exit(1);
  });
} else {
  console.error("用法: tsx src/cli.ts build|apply [--hexo <dir>] [--out out] [--image-host <url>] [--base <url>]");
  process.exit(1);
}
```

**Step 6：运行单测**

Run: `cd tools/hexo-migrate && npm test`
Expected: 全部 `ok`。

**Step 7：提交**

```bash
git add tools/hexo-migrate/src/api.ts tools/hexo-migrate/src/api.test.ts tools/hexo-migrate/src/apply.ts tools/hexo-migrate/src/cli.ts
git commit -m "feat(migrate): apply 命令通过接口导入文章、短文、相册"
```

---

### Task M8：本地导入并验收

**Step 1：备份本地数据库**（导入出问题可以回滚）

```bash
cd /e/workspace/code/blog/anheyu-app
docker compose -f docker-compose.local.yml exec -T postgres pg_dump -U anheyu -d anheyu -Fc > /tmp/anheyu-before-migrate.dump
ls -la /tmp/anheyu-before-migrate.dump
```

**Step 2：由用户运行导入**（Claude 不接触密码）。请用户在 PowerShell 里执行：

```powershell
cd E:\workspace\code\blog\anheyu-app-frontend\tools\hexo-migrate
$env:ANHEYU_EMAIL = '你的管理员邮箱'
$env:ANHEYU_PASSWORD = Read-Host '管理员密码'
npm run apply
```

Expected：文章「共 26，成功 26，跳过 0，失败 0」；短文全部新建；每个相册分类成功数等于图片数。把输出发给 Claude。

**Step 3：再运行一次 `npm run apply`，确认幂等**

Expected：文章全部跳过，短文新建 0，相册全部跳过。

**Step 4：浏览器抽查**（截图留证）

| 页面 | 检查 |
|---|---|
| 首页、`/archives` | 21 篇已发布文章，按原日期排序 |
| 「Java基础」 | 3 组标签页能切换；高亮文字、红色文字正常 |
| 「MySQL索引详情」 | 高亮正常 |
| 「nacos适配各数据库版本资源」、「破解文件资源」 | 下载按钮能点 |
| 「JUC并发编程」 | 代码块高亮、复制按钮正常 |
| 原来缺 date 的 4 篇 | 日期不是今天 |
| 后台文章管理 | 5 篇草稿；分类、标签已自动创建；字数统计不为 0（为 0 时打开该文章保存一次，记下这个现象） |
| `/essay` | 条数正确、图片和视频正常 |
| `/album` | 3 个分类，图片正常 |

**Step 5：后台手动补音乐**：编辑「歌曲推荐」那条短文，填网易云歌曲 ID（由用户找对应的歌），点「获取歌曲信息」，保存。

**Step 6：回滚演练（可选，推荐）**：确认 Step 1 的备份能恢复

```bash
docker compose -f docker-compose.local.yml exec -T postgres pg_restore -U anheyu -d anheyu --clean --if-exists < /tmp/anheyu-before-migrate.dump
```

恢复后再跑一次 `npm run apply`，结果应与 Step 2 相同。

---

## 阶段 3 验收（停下来向用户汇报）

1. `npm test` 结果（通过数）
2. `out/report.md` 全文
3. 两次 `apply` 的输出（证明幂等）
4. Step 4 的截图
5. 遗留事项：需要手动处理的短文音乐、字数统计是否需要逐篇重新保存
