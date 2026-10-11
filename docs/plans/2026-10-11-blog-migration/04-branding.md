# 阶段 4：品牌化

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 前台看不到任何原作者信息（安知鱼 / 安和鱼 / AnHeYu / anheyu.com），站点身份换成 Super-Thp 的。

**Architecture:** 以后台配置为主：一份 `tools/brand/settings.json` 通过后台「配置导入」一次写入，本地和新服务器复用。代码只动两处：控制台欢迎语（新文件 + 改 1 行 import）、文章页 `<meta name="author">` 的默认值（新的纯函数 + 两个页面各改 2 行）。初始化时写入的默认数据（友链、页面、默认文章）在后台删除或改写。

**仓库：** 代码改动在前台仓库；`tools/brand/` 放后端仓库（配置和后端设置项一一对应）。

**与设计文档的偏差：** 控制台欢迎语不直接改上游的 `src/utils/console-printer.ts`，而是新建 `src/utils/console-printer-custom.ts`，只把 `src/providers/console-printer.tsx` 的 import 指过去——对上游文件的改动从几十行降到 1 行。

---

### Task R1：控制台欢迎语

**Files:**
- Test: `src/utils/console-printer-custom.test.ts`（前台仓库）
- Create: `src/utils/console-printer-custom.ts`
- Modify: `src/providers/console-printer.tsx`（挂载点，1 行）

**Step 1：写失败的测试**

```ts
import { afterEach, describe, expect, it, vi } from "vitest";
import { buildConsoleLines } from "./console-printer-custom";

describe("buildConsoleLines", () => {
  afterEach(() => vi.restoreAllMocks());

  it("使用站点名和副标题", () => {
    const lines = buildConsoleLines("Super-Thp的个人空间", "记录学习与生活");
    const text = lines.map(l => l.message).join("\n");
    expect(text).toContain("欢迎来到 Super-Thp的个人空间");
    expect(text).toContain("记录学习与生活");
  });

  it("不包含原作者品牌", () => {
    const text = buildConsoleLines("站点", "").map(l => l.message).join("\n");
    expect(text).not.toMatch(/安知鱼|安和鱼|AnHeYu|anheyu/i);
  });

  it("站点名为空时使用通用文案", () => {
    expect(buildConsoleLines("", "")[0].message).toContain("欢迎来到本站");
  });
});
```

**Step 2：运行，确认失败**

Run: `npx vitest run src/utils/console-printer-custom.test.ts`
Expected: FAIL，找不到模块。

**Step 3：实现** `src/utils/console-printer-custom.ts`

```ts
/**
 * 控制台欢迎语（[custom] 二开替换上游 console-printer.ts，去掉原作者品牌）
 */
import { useSiteConfigStore } from "@/store/site-config-store";

interface ConsoleLine {
  message: string;
  styles: string[];
}

const BADGE = "color:white; background-color:#425AEF; padding:2px 6px; border-radius:3px";

export function buildConsoleLines(siteName: string, subTitle: string): ConsoleLine[] {
  const name = siteName.trim() || "本站";
  const lines: ConsoleLine[] = [
    { message: `%c 欢迎来到 ${name} %c ${subTitle.trim()}`, styles: [BADGE, ""] },
    { message: "%c HELLO %c 你已打开控制台，欢迎交流。", styles: [BADGE, ""] },
  ];
  return lines;
}

let hasPrinted = false;

export function printConsoleWelcome() {
  if (hasPrinted || typeof window === "undefined") return;
  hasPrinted = true;

  setTimeout(() => {
    const config = useSiteConfigStore.getState().siteConfig;
    for (const line of buildConsoleLines(config?.APP_NAME || "", config?.SUB_TITLE || "")) {
      console.log(line.message, ...line.styles);
    }
  }, 1000);
}
```

> 确认 `useSiteConfigStore.getState().siteConfig` 的类型里有 `APP_NAME`、`SUB_TITLE`（`src/types/site-config.ts`），字段名不同就照实际改。

**Step 4：运行，确认通过**

Run: `npx vitest run src/utils/console-printer-custom.test.ts`
Expected: 3 个用例通过。

**Step 5：挂载**：`src/providers/console-printer.tsx` 的 import 改为

```ts
import { printConsoleWelcome } from "@/utils/console-printer-custom"; // [custom] 替换上游 console-printer
```

**Step 6：验证**：打开 http://localhost:3000 ，F12 控制台只看到新的两行，没有 ANHEYU 字符画和「Powered by AnHeYu」。（可以用 `read_console_messages` 读取。）

**Step 7：提交**

```bash
git add src/utils/console-printer-custom.ts src/utils/console-printer-custom.test.ts src/providers/console-printer.tsx
git commit -m "feat(brand): 控制台欢迎语改为读取站点配置"
```

---

### Task R2：文章页 `<meta name="author">` 默认值

**Files:**
- Test: `src/lib/seo-author.test.ts`（前台仓库）
- Create: `src/lib/seo-author.ts`
- Modify: `src/app/(frontend)/posts/[id]/page.tsx`、`src/app/(frontend)/doc/[id]/page.tsx`（`generateMetadata` 里各 2 处）

**Step 1：写失败的测试**

```ts
import { describe, expect, it } from "vitest";
import { resolveArticleAuthor } from "./seo-author";

describe("resolveArticleAuthor", () => {
  const config = { APP_NAME: "Super-Thp的个人空间", frontDesk: { siteOwner: { name: "Super-Thp" } } };

  it("优先用文章自己的版权作者", () => {
    expect(resolveArticleAuthor("别人", config as never)).toBe("别人");
  });

  it("没有版权作者时用站点拥有者", () => {
    expect(resolveArticleAuthor("", config as never)).toBe("Super-Thp");
    expect(resolveArticleAuthor(undefined, config as never)).toBe("Super-Thp");
  });

  it("没有站点拥有者时用站点名，都没有返回空字符串", () => {
    expect(resolveArticleAuthor(null, { APP_NAME: "站点" } as never)).toBe("站点");
    expect(resolveArticleAuthor(null, null)).toBe("");
  });
});
```

**Step 2：运行，确认失败**

Run: `npx vitest run src/lib/seo-author.test.ts` → FAIL，找不到模块。

**Step 3：实现** `src/lib/seo-author.ts`

```ts
/**
 * 文章作者元信息（[custom] 二开新增，替换上游写死的「安知鱼」默认值）
 */
import type { SiteConfigData } from "@/types/site-config";

export function resolveArticleAuthor(copyrightAuthor: string | null | undefined, siteConfig: SiteConfigData | null): string {
  return (
    copyrightAuthor?.trim() ||
    siteConfig?.frontDesk?.siteOwner?.name?.trim() ||
    siteConfig?.APP_NAME?.trim() ||
    ""
  );
}
```

> `SiteConfigData` 的导出位置以 `src/lib/seo.ts` 顶部的 import 为准。

**Step 4：运行，确认通过**

Run: `npx vitest run src/lib/seo-author.test.ts` → 3 个用例通过。

**Step 5：两个页面的 `generateMetadata`**

`src/app/(frontend)/posts/[id]/page.tsx` 和 `src/app/(frontend)/doc/[id]/page.tsx` 各做同样的改动：

import 区加：

```ts
import { resolveArticleAuthor } from "@/lib/seo-author"; // [custom]
```

（两个文件都已从 `@/lib/seo` 引入或可引入 `fetchSiteConfigForSeo`，doc 页没有的话补上。）

在 `return buildPageMetadata({` 之前加一行，并替换 `authors` 那一行、把 `siteConfig` 传进去避免重复请求：

```ts
  const siteConfig = await fetchSiteConfigForSeo(); // [custom]
  return buildPageMetadata({
    // ...原有字段保持不变...
    authors: [resolveArticleAuthor(article.copyright_author, siteConfig)].filter(Boolean), // [custom]
    siteConfig, // [custom]
  });
```

**Step 6：验证**

Run: `npx tsc --noEmit 2>&1 | tail -3` → 不比阶段 2 多。
Run: `curl -s http://localhost:3000/posts/<任意一篇迁移文章的 id> | grep -o '<meta name="author"[^>]*>'`
Expected: `content="Super-Thp"`（Task R3 导入配置之后；之前是站点名）。

**Step 7：提交**

```bash
git add src/lib/seo-author.ts src/lib/seo-author.test.ts "src/app/(frontend)/posts/[id]/page.tsx" "src/app/(frontend)/doc/[id]/page.tsx"
git commit -m "feat(brand): 文章作者元信息默认使用站点拥有者"
```

---

### Task R3：品牌配置文件

**Files:**
- Create: `tools/brand/settings.json`（后端仓库）
- Create: `tools/brand/README.md`

**Step 1：先和用户确认待定项**（标 `待确认` 的值，Claude 不自行编造个人信息）：

| 配置 | 建议值 | 来源 |
|---|---|---|
| `APP_NAME` | Super-Thp的个人空间 | Hexo `title` |
| `SUB_TITLE` | 待确认（建议「记录学习与生活」） | Hexo 没有 |
| `SITE_DESCRIPTION` | 待确认（建议「Super-Thp 的个人博客：Java 后端、面试知识与实践踩坑」） | — |
| `SITE_KEYWORDS` | 待确认（建议「Super-Thp,Java,后端,面试,MySQL,JVM,博客」） | — |
| `USER_AVATAR` | `http://img.thpvip.cn/hexo-blog/%E5%A4%B4%E5%83%8F.jpg` | Hexo `avatar.img`（阶段 5 图床方案定了再改 https） |
| `ICP_NUMBER` | 豫ICP备2024093387号-1 | Hexo 页脚 |
| `frontDesk.siteOwner.name` / `footer.owner.name` | Super-Thp | Hexo `author` |
| `frontDesk.siteOwner.email` / `comment.blogger_email` | 待确认（是否公开邮箱由用户决定，可留空） | — |
| `footer.owner.since` | 2020 | Hexo 页脚 |
| `footer.runtime.*` | 上线时间 `04/01/2025 00:00:00`，上下班图片和文案沿用 Hexo | Hexo 页脚 |
| Logo、favicon | 待确认（没有就暂用头像） | — |

**Step 2：写 `tools/brand/settings.json`**（确认后把 `待确认` 替换成实际值；配置导入接口接受 `{"settings": {...}}` 格式，只覆盖列出的键）

```json
{
  "settings": {
    "APP_NAME": "Super-Thp的个人空间",
    "SUB_TITLE": "待确认",
    "SITE_DESCRIPTION": "待确认",
    "SITE_KEYWORDS": "待确认",
    "ABOUT_LINK": "/about",
    "ICP_NUMBER": "豫ICP备2024093387号-1",
    "POLICE_RECORD_NUMBER": "",
    "USER_AVATAR": "http://img.thpvip.cn/hexo-blog/%E5%A4%B4%E5%83%8F.jpg",
    "LOGO_URL": "http://img.thpvip.cn/hexo-blog/%E5%A4%B4%E5%83%8F.jpg",
    "LOGO_URL_192x192": "http://img.thpvip.cn/hexo-blog/%E5%A4%B4%E5%83%8F.jpg",
    "LOGO_URL_512x512": "http://img.thpvip.cn/hexo-blog/%E5%A4%B4%E5%83%8F.jpg",
    "frontDesk.siteOwner.name": "Super-Thp",
    "frontDesk.siteOwner.email": "",
    "comment.blogger_email": "",
    "footer.owner.name": "Super-Thp",
    "footer.owner.since": "2020",
    "footer.custom_text": "",
    "footer.runtime.enable": "true",
    "footer.runtime.launch_time": "04/01/2025 00:00:00",
    "footer.runtime.work_img": "http://img.thpvip.cn/hexo-blog/super-thp-上班摸鱼中.svg",
    "footer.runtime.work_description": "距离月入25k也就还差一个大佬带我~",
    "footer.runtime.offduty_img": "http://img.thpvip.cn/hexo-blog/super-thp-下班啦.svg",
    "footer.runtime.offduty_description": "下班了就该开开心心的玩耍，嘿嘿~",
    "footer.badge.enable": "false",
    "footer.socialBar.centerImg": "",
    "footer.socialBar.left": "[]",
    "footer.socialBar.right": "[]",
    "footer.project.list": "[]",
    "footer.bar.authorLink": "/about",
    "footer.bar.linkList": "[]",
    "header.menu": "[{\"title\":\"文章\",\"items\":[{\"title\":\"全部文章\",\"path\":\"/archives\",\"icon\":\"fa6-solid:book\",\"isExternal\":false},{\"title\":\"分类\",\"path\":\"/categories\",\"icon\":\"fa6-solid:shapes\",\"isExternal\":false},{\"title\":\"标签\",\"path\":\"/tags\",\"icon\":\"fa6-solid:tags\",\"isExternal\":false}]},{\"title\":\"我的\",\"items\":[{\"title\":\"相册集\",\"path\":\"/album\",\"icon\":\"fa6-solid:images\",\"isExternal\":false},{\"title\":\"即刻短文\",\"path\":\"/essay\",\"icon\":\"fa6-solid:feather\",\"isExternal\":false}]},{\"title\":\"关于\",\"items\":[{\"title\":\"关于本人\",\"path\":\"/about\",\"icon\":\"fa6-solid:paper-plane\",\"isExternal\":false},{\"title\":\"随便逛逛\",\"path\":\"/random-post\",\"icon\":\"fa6-solid:shoe-prints\",\"isExternal\":false}]}]",
    "header.nav.menu": "[]",
    "HOME_TOP": "{\"title\":\"Super-Thp\",\"subTitle\":\"的个人空间。\",\"siteText\":\"THPVIP.CN\",\"category\":[{\"name\":\"核心原理\",\"path\":\"/categories/核心原理与面试/\",\"background\":\"linear-gradient(to right,#358bff,#15c6ff)\",\"icon\":\"fa6-solid:book\",\"isExternal\":false},{\"name\":\"运维踩坑\",\"path\":\"/categories/运维与实战踩坑/\",\"background\":\"linear-gradient(to right,#f65,#ffbf37)\",\"icon\":\"fa6-solid:fire\",\"isExternal\":false},{\"name\":\"AI 工具\",\"path\":\"/categories/AI与效率工具/\",\"background\":\"linear-gradient(to right,#18e7ae,#1eebeb)\",\"icon\":\"fa6-solid:robot\",\"isExternal\":false}],\"banner\":{\"tips\":\"\",\"title\":\"\",\"image\":\"\",\"link\":\"\",\"isExternal\":false}}"
  }
}
```

> `HOME_TOP.banner.title` 留空即不显示右侧推荐位（`HomeTop.tsx` 里 `hasBanner` 依赖 `banner.title`）。分类路径用迁移后实际的分类名，导入后在 `/categories` 页核对。

**Step 3：`tools/brand/README.md`**

```markdown
# 品牌配置

`settings.json` 是本站的品牌配置（站点名、头像、页脚、菜单、首页顶部），本地和服务器复用同一份。

导入：后台「系统设置」→ 配置导入，上传 `settings.json`。只覆盖文件里列出的键，其他配置不受影响。

不放在这里的：`SITE_URL`（每个环境不同：本地 http://localhost:3000，线上 https://thpvip.cn），在后台单独设置。
```

**Step 4：JSON 校验**

Run: `node -e "const s=require('./tools/brand/settings.json').settings; for (const k of ['header.menu','HOME_TOP']) JSON.parse(s[k]); console.log(Object.keys(s).length, 'keys ok')"`
Expected: `33 keys ok`（数字以实际为准），没有 JSON 解析错误；且 `grep -c 待确认 tools/brand/settings.json` 输出 `0`。

**Step 5：提交**

```bash
git add tools/brand/
git commit -m "chore(brand): 新增站点品牌配置"
```

---

### Task R4：导入配置并清理默认数据（在浏览器里和用户一起做）

**Step 1：导入**：后台「系统设置」找到配置导入（找不到入口时用 `POST /api/config/import`，multipart 字段名以 `pkg/handler/config/import_export_handler.go` 为准），上传 `tools/brand/settings.json`。刷新前台确认站点名、头像、菜单、页脚已变。

**Step 2：清理初始化数据**（逐项截图）：

| 位置 | 操作 |
|---|---|
| 友链管理 | 删除两条「安知鱼」友链 |
| 页面管理 | 隐私政策、版权页：把 `/update` 链接和「安和鱼应用说明」相关文字删掉或改写 |
| 文章管理 | 删除首个账号注册时自动生成的默认文章 |
| 关于页 | 参照 hexo-blog 的 `source/_data/about.yml` 重新填写（这一步由用户决定内容） |
| 系统设置其余分区 | 和用户一起过一遍：首页技能模块（`CREATIVITY`）、侧边栏作者卡片、音乐馆歌单等，凡是写着安知鱼信息的都改掉或关闭 |

**Step 3：自动检查残留**：前台跑着的情况下运行

```bash
for p in / /archives /categories /tags /essay /album /about /random-post /link; do
  curl -sL "http://localhost:3000$p" | grep -oiE '安知鱼|安和鱼|anheyu\.com|AnHeYu' | sort | uniq -c | sed "s#^#$p #"
done
```

Expected：没有任何输出。有输出就定位来源（配置 → 后台改；数据 → 后台删；代码兜底值 → 说明原因后再决定是否改代码），处理后重跑直到无输出。另外随机打开 3 篇文章页重复检查。

---

## 阶段 4 验收（停下来向用户汇报）

1. R1、R2 的测试结果和 `tsc` 结果
2. 控制台截图、文章页 `meta author` 的 curl 输出
3. Task R4 Step 3 的检查输出（应为空）
4. 首页、文章页、`/essay`、页脚的截图
5. 本阶段两个仓库的提交列表
