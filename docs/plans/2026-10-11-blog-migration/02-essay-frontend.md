# 阶段 2：即刻短文前台与后台管理

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 前台 `/essay` 页面、首页「即刻」滚动条、后台 `/admin/essays` 管理页。

**Architecture:** 接口层 `src/lib/api/essay.ts` + 查询层 `src/hooks/queries/use-essays.ts`（TanStack Query），纯函数放 `src/lib/essay.ts`。前台页照 `recentcomments` 的写法（`BannerCard` + 列表），后台页照 `doc-series` 的写法（HeroUI `Table` + `AdminDialog` 表单 + `ConfirmDialog`）。图片放大用项目已有的 Fancybox；音乐卡片直接用 `PostContent` 渲染 `renderMusicPlayerHtml()` 生成的 HTML，复用文章页的播放器逻辑。

**仓库：** `E:\workspace\code\blog\anheyu-app-frontend`，分支 `custom`。

**前置条件：** 阶段 1 完成；本地后端（8091）和前台 dev server（3000）在运行。

**常用命令：**

| 用途 | 命令 |
|---|---|
| 单测 | `npx vitest run <文件>` |
| 类型检查 | `npx tsc --noEmit` |
| Lint | `npx eslint <文件...>` |

---

### Task F0：记录基线

上游代码本身可能就有类型或 lint 报错。先记下基线，后面只要求**不新增**错误。

**Step 1：** Run: `npx tsc --noEmit 2>&1 | tail -3`
记录输出（"无输出" 或错误条数）。

**Step 2：** Run: `npx vitest run 2>&1 | tail -6`
记录通过/失败的测试文件数和用例数。

**Step 3：** 把两项基线写进本阶段汇报的草稿里，不提交。

---

### Task F1：类型、接口、查询 hooks

**Files:**
- Create: `src/types/essay.ts`
- Create: `src/lib/api/essay.ts`
- Create: `src/hooks/queries/use-essays.ts`

这三个文件是对后端接口的薄封装，没有逻辑分支，不单独写测试；在 F3、F4、F6 的组件测试里通过 mock 覆盖。

**Step 1：类型** `src/types/essay.ts`

```ts
/**
 * 即刻短文类型（[custom] 二开新增）
 * 与后端 model.EssayResponse / model.EssayRequest 对齐
 */

export interface Essay {
  id: number;
  created_at: string;
  updated_at: string;
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

/** 创建/更新请求体；published_at 不传时，创建取当前时间、更新保持原值 */
export interface EssayPayload {
  content: string;
  published_at?: string;
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

export interface EssayListResponse {
  list: Essay[];
  total: number;
  page: number;
  pageSize: number;
}

export interface EssayListParams {
  page?: number;
  pageSize?: number;
}
```

**Step 2：接口** `src/lib/api/essay.ts`

```ts
/**
 * 即刻短文 API（[custom] 二开新增）
 * 公开：GET /api/public/essays
 * 后台：/api/essays（需要管理员登录）
 */

import { apiClient } from "./client";
import type { Essay, EssayListParams, EssayListResponse, EssayPayload } from "@/types/essay";

function listQuery({ page = 1, pageSize = 20 }: EssayListParams): string {
  return new URLSearchParams({ page: String(page), pageSize: String(pageSize) }).toString();
}

export const essayApi = {
  async getPublicList(params: EssayListParams = {}): Promise<EssayListResponse> {
    const response = await apiClient.get<EssayListResponse>(`/api/public/essays?${listQuery(params)}`);
    if (response.code === 200 && response.data) return response.data;
    throw new Error(response.message || "获取短文列表失败");
  },

  async getAdminList(params: EssayListParams = {}): Promise<EssayListResponse> {
    const response = await apiClient.get<EssayListResponse>(`/api/essays?${listQuery(params)}`);
    if (response.code === 200 && response.data) return response.data;
    throw new Error(response.message || "获取短文列表失败");
  },

  async create(data: EssayPayload): Promise<Essay> {
    const response = await apiClient.post<Essay>("/api/essays", data);
    if (response.code === 200 && response.data) return response.data;
    throw new Error(response.message || "创建短文失败");
  },

  async update(id: number, data: EssayPayload): Promise<Essay> {
    const response = await apiClient.put<Essay>(`/api/essays/${id}`, data);
    if (response.code === 200 && response.data) return response.data;
    throw new Error(response.message || "更新短文失败");
  },

  async delete(id: number): Promise<void> {
    const response = await apiClient.delete(`/api/essays/${id}`);
    if (response.code !== 200) throw new Error(response.message || "删除短文失败");
  },
};
```

> 确认 `apiClient` 在后端返回 4xx 时的行为：打开 `src/lib/api/client.ts` 看 `get/post` 是抛异常还是返回 `{code, message}`。两种情况上面的写法都能把后端的中文错误消息带出来（抛异常时由调用方的 `catch` 显示 `getErrorMessage(error)`），如果发现消息丢失，在 F5 的表单 `catch` 里改用 `getErrorMessage`（`src/lib/api/client.ts` 已导出）。

**Step 3：查询 hooks** `src/hooks/queries/use-essays.ts`

```ts
/**
 * 即刻短文 Query Hooks（[custom] 二开新增）
 */

import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { essayApi } from "@/lib/api/essay";
import type { EssayListParams, EssayPayload } from "@/types/essay";

export const essayKeys = {
  all: ["essays"] as const,
  public: (params: EssayListParams) => [...essayKeys.all, "public", params] as const,
  publicInfinite: (pageSize: number) => [...essayKeys.all, "public-infinite", pageSize] as const,
  admin: (params: EssayListParams) => [...essayKeys.all, "admin", params] as const,
};

/** 前台：固定一页（首页滚动条用） */
export function usePublicEssays(params: EssayListParams = {}) {
  return useQuery({
    queryKey: essayKeys.public(params),
    queryFn: () => essayApi.getPublicList(params),
    staleTime: 1000 * 60,
  });
}

/** 前台：/essay 页面「加载更多」 */
export function useInfinitePublicEssays(pageSize = 20) {
  return useInfiniteQuery({
    queryKey: essayKeys.publicInfinite(pageSize),
    queryFn: ({ pageParam }) => essayApi.getPublicList({ page: pageParam, pageSize }),
    initialPageParam: 1,
    getNextPageParam: (lastPage, allPages) =>
      allPages.length * pageSize < lastPage.total ? allPages.length + 1 : undefined,
  });
}

/** 后台：分页列表 */
export function useAdminEssays(params: EssayListParams = {}) {
  return useQuery({
    queryKey: essayKeys.admin(params),
    queryFn: () => essayApi.getAdminList(params),
    placeholderData: keepPreviousData,
  });
}

function useInvalidateEssays() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: essayKeys.all, refetchType: "all" });
}

export function useCreateEssay() {
  const invalidate = useInvalidateEssays();
  return useMutation({ mutationFn: (data: EssayPayload) => essayApi.create(data), onSuccess: invalidate });
}

export function useUpdateEssay() {
  const invalidate = useInvalidateEssays();
  return useMutation({
    mutationFn: ({ id, data }: { id: number; data: EssayPayload }) => essayApi.update(id, data),
    onSuccess: invalidate,
  });
}

export function useDeleteEssay() {
  const invalidate = useInvalidateEssays();
  return useMutation({ mutationFn: (id: number) => essayApi.delete(id), onSuccess: invalidate });
}
```

**Step 4：类型检查与 lint**

Run: `npx tsc --noEmit 2>&1 | tail -3` → 与 F0 基线相同，没有新增错误。
Run: `npx eslint src/types/essay.ts src/lib/api/essay.ts src/hooks/queries/use-essays.ts` → 无输出。

**Step 5：提交**

```bash
git add src/types/essay.ts src/lib/api/essay.ts src/hooks/queries/use-essays.ts
git commit -m "feat(essay): 新增即刻短文类型、接口与查询 hooks"
```

---

### Task F2：纯函数工具

**Files:**
- Test: `src/lib/essay.test.ts`
- Create: `src/lib/essay.ts`

**Step 1：写失败的测试**

```ts
import { describe, expect, it } from "vitest";
import {
  EMPTY_ESSAY_PAYLOAD,
  essayImageColumns,
  essaySummary,
  essayToPayload,
  fromDateTimeLocal,
  parseStringList,
  stringifyStringList,
  toDateTimeLocal,
} from "./essay";
import type { Essay } from "@/types/essay";

const base: Essay = {
  id: 1,
  created_at: "2025-07-01T04:00:00Z",
  updated_at: "2025-07-01T04:00:00Z",
  content: "",
  published_at: "2025-07-01T04:00:00Z",
  images: [],
  videos: [],
  address: "",
  source: "",
  link_url: "",
  link_title: "",
  music_id: "",
  music_name: "",
  music_artist: "",
  music_cover: "",
  is_published: true,
};

describe("essaySummary", () => {
  it("取第一行非空文字", () => {
    expect(essaySummary({ ...base, content: "\n  第一行  \n第二行" })).toBe("第一行");
  });

  it("超过 60 个字截断并加省略号", () => {
    const text = "字".repeat(61);
    expect(essaySummary({ ...base, content: text })).toBe(`${"字".repeat(60)}…`);
  });

  it("没有文字时按媒体类型给占位", () => {
    expect(essaySummary({ ...base, images: ["https://a/b.jpg"] })).toBe("[图片]");
    expect(essaySummary({ ...base, videos: ["https://a/b.mp4"] })).toBe("[视频]");
    expect(essaySummary({ ...base, music_id: "123" })).toBe("[音乐]");
    expect(essaySummary({ ...base, link_url: "https://a" })).toBe("[链接]");
  });
});

describe("string list", () => {
  it("解析 FormStringList 的 JSON 值并去掉空白项", () => {
    expect(parseStringList('[" https://a ", "", 1, "https://b"]')).toEqual(["https://a", "https://b"]);
  });

  it("非法 JSON 返回空数组", () => {
    expect(parseStringList("not json")).toEqual([]);
    expect(parseStringList("")).toEqual([]);
  });

  it("序列化后可以再解析回来", () => {
    expect(parseStringList(stringifyStringList(["https://a"]))).toEqual(["https://a"]);
  });
});

describe("datetime-local 转换", () => {
  it("往返转换保持同一时刻（与本机时区无关）", () => {
    const iso = "2025-07-01T04:00:00.000Z";
    expect(fromDateTimeLocal(toDateTimeLocal(iso))).toBe(iso);
  });

  it("非法输入", () => {
    expect(toDateTimeLocal("bad")).toBe("");
    expect(fromDateTimeLocal("")).toBeUndefined();
  });
});

describe("essayImageColumns", () => {
  it.each([
    [0, 1],
    [1, 1],
    [2, 2],
    [3, 3],
    [4, 2],
    [9, 3],
  ])("%i 张图 → %i 列", (count, cols) => {
    expect(essayImageColumns(count)).toBe(cols);
  });
});

describe("essayToPayload", () => {
  it("只保留请求体字段", () => {
    const payload = essayToPayload({ ...base, content: "x", is_published: false });
    expect(payload).toEqual({ ...EMPTY_ESSAY_PAYLOAD, content: "x", is_published: false, published_at: base.published_at });
    expect(payload).not.toHaveProperty("id");
  });
});
```

**Step 2：运行，确认失败**

Run: `npx vitest run src/lib/essay.test.ts`
Expected: FAIL，`Failed to resolve import "./essay"`。

**Step 3：实现** `src/lib/essay.ts`

```ts
/**
 * 即刻短文纯函数（[custom] 二开新增）
 */

import type { Essay, EssayPayload } from "@/types/essay";

export const ESSAY_SUMMARY_MAX = 60;

export const EMPTY_ESSAY_PAYLOAD: EssayPayload = {
  content: "",
  images: [],
  videos: [],
  address: "",
  source: "",
  link_url: "",
  link_title: "",
  music_id: "",
  music_name: "",
  music_artist: "",
  music_cover: "",
  is_published: true,
};

type SummarySource = Pick<Essay, "content" | "images" | "videos" | "music_id" | "link_url">;

/** 首页滚动条用的一行摘要 */
export function essaySummary(essay: SummarySource): string {
  const firstLine = essay.content
    .split(/\r?\n/)
    .map(line => line.trim())
    .find(Boolean);
  if (firstLine) {
    const chars = Array.from(firstLine);
    return chars.length > ESSAY_SUMMARY_MAX ? `${chars.slice(0, ESSAY_SUMMARY_MAX).join("")}…` : firstLine;
  }
  if (essay.images.length > 0) return "[图片]";
  if (essay.videos.length > 0) return "[视频]";
  if (essay.music_id) return "[音乐]";
  if (essay.link_url) return "[链接]";
  return "";
}

/** FormStringList 的值是 JSON 字符串数组 */
export function parseStringList(raw: string): string[] {
  try {
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    return value
      .filter((item): item is string => typeof item === "string")
      .map(item => item.trim())
      .filter(Boolean);
  } catch {
    return [];
  }
}

export function stringifyStringList(list: string[]): string {
  return JSON.stringify(list);
}

/** ISO 时间 → <input type="datetime-local"> 的值（本地时间，精确到分钟） */
export function toDateTimeLocal(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** <input type="datetime-local"> 的值 → ISO 时间 */
export function fromDateTimeLocal(local: string): string | undefined {
  if (!local) return undefined;
  const d = new Date(local);
  return Number.isNaN(d.getTime()) ? undefined : d.toISOString();
}

/** 图片宫格列数：1 张单列大图，2/4 张两列，其余三列 */
export function essayImageColumns(count: number): 1 | 2 | 3 {
  if (count <= 1) return 1;
  if (count === 2 || count === 4) return 2;
  return 3;
}

/** 编辑已有短文时，把响应转成请求体 */
export function essayToPayload(essay: Essay): EssayPayload {
  return {
    content: essay.content,
    published_at: essay.published_at,
    images: essay.images,
    videos: essay.videos,
    address: essay.address,
    source: essay.source,
    link_url: essay.link_url,
    link_title: essay.link_title,
    music_id: essay.music_id,
    music_name: essay.music_name,
    music_artist: essay.music_artist,
    music_cover: essay.music_cover,
    is_published: essay.is_published,
  };
}
```

**Step 4：运行，确认通过**

Run: `npx vitest run src/lib/essay.test.ts`
Expected: 全部通过。

**Step 5：提交**

```bash
git add src/lib/essay.ts src/lib/essay.test.ts
git commit -m "feat(essay): 新增即刻短文纯函数工具"
```

---

### Task F3：首页「即刻」滚动条

**Files:**
- Test: `src/components/home/HomeTop/EssayBar.test.tsx`
- Create: `src/components/home/HomeTop/EssayBar.tsx`（样式用上游遗留的 `EssayBar.module.css`，不改它）
- Modify: `src/components/home/HomePageContent.tsx`（挂载点）

**Step 1：写失败的测试**

```tsx
import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { EssayBar } from "./EssayBar";

const { usePublicEssaysMock } = vi.hoisted(() => ({ usePublicEssaysMock: vi.fn() }));

vi.mock("@/hooks/queries/use-essays", () => ({ usePublicEssays: usePublicEssaysMock }));
vi.mock("@iconify/react", () => ({ Icon: () => <span data-testid="icon" /> }));

function essay(id: number, content: string) {
  return {
    id,
    content,
    images: [],
    videos: [],
    music_id: "",
    link_url: "",
  };
}

describe("EssayBar", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    usePublicEssaysMock.mockReset();
  });

  it("没有短文时不渲染", () => {
    usePublicEssaysMock.mockReturnValue({ data: { list: [], total: 0 } });
    const { container } = render(<EssayBar />);
    expect(container).toBeEmptyDOMElement();
  });

  it("渲染摘要并链接到 /essay", () => {
    usePublicEssaysMock.mockReturnValue({ data: { list: [essay(1, "第一条"), essay(2, "第二条")], total: 2 } });
    render(<EssayBar />);
    expect(screen.getByText("第一条")).toBeInTheDocument();
    for (const link of screen.getAllByRole("link")) {
      expect(link).toHaveAttribute("href", "/essay");
    }
  });

  it("每 3 秒滚动到下一条", () => {
    usePublicEssaysMock.mockReturnValue({ data: { list: [essay(1, "第一条"), essay(2, "第二条")], total: 2 } });
    render(<EssayBar />);
    const carousel = screen.getByTestId("essay-carousel");
    expect(carousel.style.transform).toBe("translateY(-0px)");
    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(carousel.style.transform).toBe("translateY(-30px)");
  });

  it("只取最新 5 条", () => {
    usePublicEssaysMock.mockReturnValue({ data: { list: [], total: 0 } });
    render(<EssayBar />);
    expect(usePublicEssaysMock).toHaveBeenCalledWith({ page: 1, pageSize: 5 });
  });
});
```

**Step 2：运行，确认失败**

Run: `npx vitest run src/components/home/HomeTop/EssayBar.test.tsx`
Expected: FAIL，找不到 `./EssayBar`。

**Step 3：实现** `src/components/home/HomeTop/EssayBar.tsx`

```tsx
/**
 * 首页「即刻」滚动条（[custom] 二开新增，样式沿用上游遗留的 EssayBar.module.css）
 */
"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { Icon } from "@iconify/react";
import { usePublicEssays } from "@/hooks/queries/use-essays";
import { essaySummary } from "@/lib/essay";
import styles from "./EssayBar.module.css";

const BAR_SIZE = 5;
const ITEM_HEIGHT = 30; // 与 .essayItem 的 height 保持一致
const INTERVAL_MS = 3000;

export function EssayBar() {
  const { data } = usePublicEssays({ page: 1, pageSize: BAR_SIZE });
  const items = useMemo(
    () => (data?.list ?? []).map(essay => ({ id: essay.id, text: essaySummary(essay) })).filter(item => item.text),
    [data]
  );
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (items.length < 2) return;
    const timer = setInterval(() => setIndex(i => (i + 1) % items.length), INTERVAL_MS);
    return () => clearInterval(timer);
  }, [items.length]);

  if (items.length === 0) return null;

  return (
    <div className={styles.essayBarContainer}>
      <Link href="/essay" className={styles.essayLogo} aria-label="即刻短文">
        <Icon icon="ri:quill-pen-line" className={styles.essayLogoIcon} />
      </Link>
      <div className={styles.essayCarouselWrapper}>
        <Link href="/essay" className={styles.essayCarouselLink}>
          <div
            data-testid="essay-carousel"
            className={styles.essayCarousel}
            style={{ transform: `translateY(-${(index % items.length) * ITEM_HEIGHT}px)` }}
          >
            {items.map(item => (
              <div key={item.id} className={styles.essayItem}>
                {item.text}
              </div>
            ))}
          </div>
        </Link>
      </div>
      <Link href="/essay" className={styles.essayMoreBtn} aria-label="查看全部短文">
        <Icon icon="ri:arrow-right-circle-line" width={24} />
      </Link>
    </div>
  );
}
```

**Step 4：运行，确认通过**

Run: `npx vitest run src/components/home/HomeTop/EssayBar.test.tsx`
Expected: 4 个用例通过。

**Step 5：挂载到首页**

`src/components/home/HomePageContent.tsx`：

在 import 区加：

```tsx
import { EssayBar } from "@/components/home/HomeTop/EssayBar"; // [custom]
```

在 `{/* 首页顶部区域 */}` 那个 `motion.div` **之前**插入：

```tsx
      {/* [custom] 即刻短文滚动条 */}
      <motion.div className="pt-4" variants={itemVariants}>
        <EssayBar />
      </motion.div>
```

**Step 6：浏览器验证**

先用 curl 造两条数据（需要管理员 token，由用户在浏览器后台登录后，用 F12 → Application → Local Storage 找到 token；**或者**直接等 Task F5 后台页做完再回来验证这一步）。

打开 http://localhost:3000 ，截图确认：
- 有短文时，导航栏下方出现「即刻」条，文字每 3 秒向上滚动一条，点击跳到 `/essay`
- 没有短文时不出现，页面布局和改动前一致
- 窄屏（`resize_window` 设 375×812）下不溢出

间距不对时只调整挂载处 `motion.div` 的 Tailwind 类，不改 `EssayBar.module.css`。

**Step 7：lint 并提交**

Run: `npx eslint src/components/home/HomeTop/EssayBar.tsx src/components/home/HomeTop/EssayBar.test.tsx src/components/home/HomePageContent.tsx` → 无输出。

```bash
git add src/components/home/HomeTop/EssayBar.tsx src/components/home/HomeTop/EssayBar.test.tsx src/components/home/HomePageContent.tsx
git commit -m "feat(essay): 首页新增即刻短文滚动条"
```

---

### Task F4：前台 `/essay` 页面

**Files:**
- Test: `src/app/(frontend)/essay/_components/EssayCard.test.tsx`
- Create: `src/app/(frontend)/essay/_components/EssayCard.tsx`
- Create: `src/app/(frontend)/essay/_components/EssayPageClient.tsx`
- Create: `src/app/(frontend)/essay/page.tsx`

**Step 1：写失败的测试（卡片）**

```tsx
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EssayCard } from "./EssayCard";
import type { Essay } from "@/types/essay";

vi.mock("@fancyapps/ui", () => ({ Fancybox: { bind: vi.fn(), unbind: vi.fn() } }));
vi.mock("@fancyapps/ui/dist/fancybox/fancybox.css", () => ({}));
vi.mock("@iconify/react", () => ({ Icon: () => <span /> }));
vi.mock("@/components/post/PostContent", () => ({
  PostContent: ({ content }: { content: string }) => (
    <div data-testid="music-player" data-has-music={content.includes('data-music-id="186016"')} />
  ),
}));

const base: Essay = {
  id: 7,
  created_at: "2025-07-01T04:00:00Z",
  updated_at: "2025-07-01T04:00:00Z",
  content: "沙茶面一般般\n姜母鸭还阔以",
  published_at: "2025-07-01T04:00:00Z",
  images: ["https://img.example.com/1.jpg", "https://img.example.com/2.jpg"],
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
};

describe("EssayCard", () => {
  afterEach(cleanup);

  it("渲染作者、文字、图片、地点和来源", () => {
    render(<EssayCard essay={base} ownerName="Super-Thp" />);
    expect(screen.getByText("Super-Thp")).toBeInTheDocument();
    expect(screen.getByText(/沙茶面一般般/)).toBeInTheDocument();
    expect(screen.getAllByRole("img", { name: /短文配图/ })).toHaveLength(2);
    expect(screen.getByText("厦门")).toBeInTheDocument();
    expect(screen.getByText("来自 super-thp")).toBeInTheDocument();
  });

  it("外链卡片在新窗口打开", () => {
    render(<EssayCard essay={{ ...base, link_url: "https://example.com", link_title: "示例" }} ownerName="x" />);
    const link = screen.getByRole("link", { name: /示例/ });
    expect(link).toHaveAttribute("href", "https://example.com");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("有歌曲 ID 才渲染音乐卡片", () => {
    const { rerender } = render(<EssayCard essay={base} ownerName="x" />);
    expect(screen.queryByTestId("music-player")).not.toBeInTheDocument();
    rerender(<EssayCard essay={{ ...base, music_id: "186016", music_name: "晴天" }} ownerName="x" />);
    expect(screen.getByTestId("music-player")).toHaveAttribute("data-has-music", "true");
  });

  it("视频带播放控件", () => {
    const { container } = render(<EssayCard essay={{ ...base, videos: ["https://img.example.com/a.mp4"] }} ownerName="x" />);
    const video = container.querySelector("video");
    expect(video).toHaveAttribute("src", "https://img.example.com/a.mp4");
    expect(video).toHaveAttribute("controls");
  });
});
```

**Step 2：运行，确认失败**

Run: `npx vitest run "src/app/(frontend)/essay/_components/EssayCard.test.tsx"`
Expected: FAIL，找不到 `./EssayCard`。

**Step 3：实现卡片** `src/app/(frontend)/essay/_components/EssayCard.tsx`

```tsx
/**
 * 即刻短文卡片（[custom] 二开新增）
 */
"use client";

import { useEffect, useRef } from "react";
import { Fancybox } from "@fancyapps/ui";
import "@fancyapps/ui/dist/fancybox/fancybox.css";
import { Icon } from "@iconify/react";
import { PostContent } from "@/components/post/PostContent";
import { renderMusicPlayerHtml } from "@/lib/marked-extensions";
import { essayImageColumns } from "@/lib/essay";
import { cn } from "@/lib/utils";
import { formatDateTimeParts } from "@/utils/date";
import type { Essay } from "@/types/essay";

const GRID_CLASS = { 1: "grid-cols-1", 2: "grid-cols-2", 3: "grid-cols-3" } as const;

interface EssayCardProps {
  essay: Essay;
  ownerName: string;
  ownerAvatar?: string;
}

export function EssayCard({ essay, ownerName, ownerAvatar }: EssayCardProps) {
  const galleryRef = useRef<HTMLDivElement>(null);
  const hasImages = essay.images.length > 0;

  useEffect(() => {
    const el = galleryRef.current;
    if (!el || !hasImages) return;
    Fancybox.bind(el, "[data-fancybox]", { groupAll: true });
    return () => Fancybox.unbind(el);
  }, [hasImages]);

  const published = formatDateTimeParts(essay.published_at);
  const columns = essayImageColumns(essay.images.length);

  return (
    <article className="rounded-xl border border-border bg-card p-5 shadow-sm transition-shadow duration-300 hover:shadow-md">
      <header className="flex items-center gap-3">
        {ownerAvatar ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={ownerAvatar} alt={`${ownerName}头像`} className="h-10 w-10 rounded-full border border-border object-cover" />
        ) : (
          <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 font-bold text-primary">
            {ownerName.charAt(0)}
          </div>
        )}
        <div className="min-w-0">
          <div className="font-semibold text-foreground">{ownerName}</div>
          <time dateTime={essay.published_at} className="text-xs text-muted-foreground tabular-nums">
            {published.date} {published.time}
          </time>
        </div>
      </header>

      {essay.content && (
        <p className="mt-3 whitespace-pre-wrap break-words text-[15px] leading-7 text-foreground">{essay.content}</p>
      )}

      {hasImages && (
        <div ref={galleryRef} className={cn("mt-3 grid gap-2", GRID_CLASS[columns])}>
          {essay.images.map((src, i) => (
            <a
              key={`${src}-${i}`}
              href={src}
              data-fancybox={`essay-${essay.id}`}
              className={cn("block overflow-hidden rounded-lg bg-muted", columns === 1 ? "max-h-[420px]" : "aspect-square")}
            >
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={src}
                alt={`短文配图 ${i + 1}`}
                loading="lazy"
                className="h-full w-full object-cover transition-transform duration-300 hover:scale-105"
              />
            </a>
          ))}
        </div>
      )}

      {essay.videos.map((src, i) => (
        <video key={`${src}-${i}`} src={src} controls preload="metadata" className="mt-3 w-full rounded-lg bg-black" />
      ))}

      {essay.link_url && (
        <a
          href={essay.link_url}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-3 flex items-center gap-3 rounded-lg border border-border bg-muted/40 px-4 py-3 text-sm transition-colors hover:border-primary hover:text-primary"
        >
          <Icon icon="ri:link" width={18} className="shrink-0" />
          <span className="truncate">{essay.link_title || essay.link_url}</span>
        </a>
      )}

      {essay.music_id && (
        <div className="mt-3">
          <PostContent
            content={renderMusicPlayerHtml({
              neteaseId: essay.music_id,
              name: essay.music_name,
              artist: essay.music_artist,
              pic: essay.music_cover,
              instanceKey: essay.id,
            })}
          />
        </div>
      )}

      {(essay.address || essay.source) && (
        <footer className="mt-3 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {essay.address && (
            <span className="inline-flex items-center gap-1 rounded-full bg-muted px-2.5 py-1">
              <Icon icon="ri:map-pin-line" width={14} />
              {essay.address}
            </span>
          )}
          {essay.source && <span className="rounded-full bg-muted px-2.5 py-1">来自 {essay.source}</span>}
        </footer>
      )}
    </article>
  );
}
```

**Step 4：运行，确认通过**

Run: `npx vitest run "src/app/(frontend)/essay/_components/EssayCard.test.tsx"`
Expected: 4 个用例通过。

**Step 5：页面** `src/app/(frontend)/essay/_components/EssayPageClient.tsx`

```tsx
/**
 * 即刻短文页面（[custom] 二开新增）
 */
"use client";

import { Button } from "@heroui/react";
import { BannerCard } from "@/components/common/BannerCard";
import { Spinner } from "@/components/ui";
import { useInfinitePublicEssays } from "@/hooks/queries/use-essays";
import { useSiteConfigStore } from "@/store/site-config-store";
import { EssayCard } from "./EssayCard";

const PAGE_SIZE = 20;

const BANNER = {
  tips: "即刻短文",
  title: "咸鱼的日常生活。",
  description: "随时随地，分享生活",
};

export function EssayPageClient() {
  const siteConfig = useSiteConfigStore(state => state.siteConfig);
  const ownerName = siteConfig?.frontDesk?.siteOwner?.name || siteConfig?.APP_NAME || "";
  const ownerAvatar = siteConfig?.USER_AVATAR || undefined;

  const { data, isPending, isError, hasNextPage, fetchNextPage, isFetchingNextPage } =
    useInfinitePublicEssays(PAGE_SIZE);
  const essays = data?.pages.flatMap(page => page.list) ?? [];

  return (
    <div className="mx-auto w-full max-w-[1400px] space-y-6 px-6 py-8">
      <BannerCard tips={BANNER.tips} title={BANNER.title} description={BANNER.description} height={300} />

      {isPending && (
        <div className="flex items-center justify-center py-10">
          <Spinner />
        </div>
      )}

      {isError && <p className="text-muted-foreground">短文加载失败，请稍后再试。</p>}

      {!isPending && !isError && essays.length === 0 && <p className="text-muted-foreground">还没有短文</p>}

      <div className="columns-1 gap-4 md:columns-2 [&>*]:mb-4 [&>*]:break-inside-avoid">
        {essays.map(essay => (
          <EssayCard key={essay.id} essay={essay} ownerName={ownerName} ownerAvatar={ownerAvatar} />
        ))}
      </div>

      {hasNextPage && (
        <div className="flex justify-center">
          <Button variant="flat" onPress={() => fetchNextPage()} isLoading={isFetchingNextPage}>
            加载更多
          </Button>
        </div>
      )}
    </div>
  );
}
```

`src/app/(frontend)/essay/page.tsx`：

```tsx
import type { Metadata } from "next";
import { buildPageMetadata } from "@/lib/seo";
import { EssayPageClient } from "./_components/EssayPageClient";

export async function generateMetadata(): Promise<Metadata> {
  return buildPageMetadata({
    title: "即刻短文",
    description: "随时随地，分享生活。",
    path: "/essay",
  });
}

export default function EssayPage() {
  return <EssayPageClient />;
}
```

**Step 6：类型检查、lint、浏览器验证**

Run: `npx tsc --noEmit 2>&1 | tail -3` → 不比 F0 基线多。
Run: `npx eslint "src/app/(frontend)/essay"` → 无输出。

打开 http://localhost:3000/essay 截图确认：顶部横幅、空列表提示「还没有短文」、桌面两列 / 手机一列。有数据后（F5 完成后回来复查）：点图片能放大、视频能播放、音乐卡片显示歌名并能播放、外链新窗口打开。

**Step 7：提交**

```bash
git add "src/app/(frontend)/essay"
git commit -m "feat(essay): 新增前台即刻短文页面"
```

---

### Task F5：后台 `/admin/essays` 管理页

**Files:**
- Modify: `src/components/admin/article-editor/extensions/music-block.tsx`（挂载点：`fetchMusicDetail` 加 `export`）
- Modify: `src/config/admin-menu.ts`（挂载点：加菜单项）
- Test: `src/components/admin/essays/EssayFormModal.test.tsx`
- Create: `src/components/admin/essays/EssayFormModal.tsx`
- Create: `src/components/admin/essays/EssayTableColumns.tsx`
- Create: `src/app/admin/essays/page.tsx`

页面比 `doc-series` 简单：没有筛选和批量选择，只有「新增 / 编辑 / 删除 / 分页」。

**Step 1：两处挂载点**

`music-block.tsx` 第 50 行左右：

```ts
// [custom] 导出供即刻短文后台复用
export async function fetchMusicDetail(neteaseId: string): Promise<{ name: string; artist: string; pic: string } | null> {
```

`src/config/admin-menu.ts`，在 `content` 组的 `albums` 项后面加：

```ts
      // [custom] 即刻短文
      {
        id: "essays",
        label: "即刻短文",
        href: "/admin/essays",
        icon: "ri:quill-pen-line",
        roles: ["admin"],
      },
```

**Step 2：写失败的测试（表单）**

只测表单的关键行为：必填校验、提交的请求体、编辑时回填。

```tsx
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import EssayFormModal from "./EssayFormModal";
import type { Essay } from "@/types/essay";

const { createMock, updateMock, toastMock } = vi.hoisted(() => ({
  createMock: vi.fn().mockResolvedValue({}),
  updateMock: vi.fn().mockResolvedValue({}),
  toastMock: vi.fn(),
}));

vi.mock("@/hooks/queries/use-essays", () => ({
  useCreateEssay: () => ({ mutateAsync: createMock, isPending: false }),
  useUpdateEssay: () => ({ mutateAsync: updateMock, isPending: false }),
}));
vi.mock("@/components/admin/AdminDialog", () => ({
  AdminDialog: ({ isOpen, children }: { isOpen: boolean; children: React.ReactNode }) =>
    isOpen ? <div>{children}</div> : null,
}));
vi.mock("@/components/ui/form-string-list", () => ({
  FormStringList: ({ label }: { label: string }) => <div>{label}</div>,
}));
vi.mock("@/components/ui/form-image-upload", () => ({ FormImageUpload: () => <div>upload</div> }));
vi.mock("@/components/admin/article-editor/extensions/music-block", () => ({ fetchMusicDetail: vi.fn() }));
vi.mock("@heroui/react", async () => {
  const actual = await vi.importActual<typeof import("@heroui/react")>("@heroui/react");
  return { ...actual, addToast: toastMock };
});

describe("EssayFormModal", () => {
  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("文字和媒体都为空时提示并不提交", async () => {
    render(<EssayFormModal isOpen onClose={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "发布" }));
    await waitFor(() => expect(toastMock).toHaveBeenCalledWith(expect.objectContaining({ color: "warning" })));
    expect(createMock).not.toHaveBeenCalled();
  });

  it("新增时提交去掉首尾空白的请求体", async () => {
    const onClose = vi.fn();
    render(<EssayFormModal isOpen onClose={onClose} />);
    fireEvent.change(screen.getByLabelText("文字内容"), { target: { value: "  海边  " } });
    fireEvent.click(screen.getByRole("button", { name: "发布" }));
    await waitFor(() => expect(createMock).toHaveBeenCalledTimes(1));
    expect(createMock.mock.calls[0][0]).toMatchObject({ content: "海边", images: [], is_published: true });
    expect(onClose).toHaveBeenCalled();
  });

  it("编辑时回填并调用更新", async () => {
    const editItem = {
      id: 3,
      content: "旧内容",
      published_at: "2025-07-01T04:00:00.000Z",
      images: [],
      videos: [],
      address: "厦门",
      source: "",
      link_url: "",
      link_title: "",
      music_id: "",
      music_name: "",
      music_artist: "",
      music_cover: "",
      is_published: false,
    } as unknown as Essay;
    render(<EssayFormModal isOpen onClose={vi.fn()} editItem={editItem} />);
    expect(screen.getByLabelText("文字内容")).toHaveValue("旧内容");
    fireEvent.click(screen.getByRole("button", { name: "保存修改" }));
    await waitFor(() => expect(updateMock).toHaveBeenCalledTimes(1));
    expect(updateMock.mock.calls[0][0]).toMatchObject({
      id: 3,
      data: { content: "旧内容", address: "厦门", is_published: false, published_at: "2025-07-01T04:00:00.000Z" },
    });
  });
});
```

> HeroUI 的 `Textarea` 通过 `label` 生成可访问名称，`getByLabelText("文字内容")` 能找到真实的 `<textarea>`。如果 HeroUI 在 jsdom 里渲染异常（例如缺少 `ResizeObserver`），参照仓库里其他 admin 组件测试的写法补 polyfill 或 mock，**不要删用例**；无法解决时停下来报告。

**Step 3：运行，确认失败**

Run: `npx vitest run src/components/admin/essays/EssayFormModal.test.tsx`
Expected: FAIL，找不到 `./EssayFormModal`。

**Step 4：实现表单** `src/components/admin/essays/EssayFormModal.tsx`

```tsx
/**
 * 即刻短文新增/编辑弹窗（[custom] 二开新增）
 * 与 DocSeriesFormModal 相同：外层控制开关，内层在打开时挂载，初始值直接来自 props
 */
"use client";

import { useCallback, useState } from "react";
import { Button, Input, ModalBody, ModalFooter, Switch, Textarea, addToast } from "@heroui/react";
import { Pencil, Feather } from "lucide-react";
import { AdminDialog } from "@/components/admin/AdminDialog";
import { fetchMusicDetail } from "@/components/admin/article-editor/extensions/music-block";
import { FormImageUpload } from "@/components/ui/form-image-upload";
import { FormStringList } from "@/components/ui/form-string-list";
import { useCreateEssay, useUpdateEssay } from "@/hooks/queries/use-essays";
import {
  EMPTY_ESSAY_PAYLOAD,
  essayToPayload,
  fromDateTimeLocal,
  parseStringList,
  stringifyStringList,
  toDateTimeLocal,
} from "@/lib/essay";
import type { Essay, EssayPayload } from "@/types/essay";

interface EssayFormModalProps {
  isOpen: boolean;
  onClose: () => void;
  editItem?: Essay | null;
}

export default function EssayFormModal({ isOpen, onClose, editItem }: EssayFormModalProps) {
  const isEdit = !!editItem;
  return (
    <AdminDialog
      isOpen={isOpen}
      onClose={onClose}
      size="2xl"
      scrollBehavior="inside"
      header={{
        title: isEdit ? "编辑短文" : "发布短文",
        description: "文字、图片、视频至少填写一项",
        icon: isEdit ? Pencil : Feather,
      }}
    >
      {isOpen && <EssayFormContent editItem={editItem} onClose={onClose} />}
    </AdminDialog>
  );
}

function EssayFormContent({ editItem, onClose }: { editItem?: Essay | null; onClose: () => void }) {
  const isEdit = !!editItem;
  const initial = editItem ? essayToPayload(editItem) : EMPTY_ESSAY_PAYLOAD;

  const [form, setForm] = useState<EssayPayload>(initial);
  const [imagesRaw, setImagesRaw] = useState(stringifyStringList(initial.images));
  const [videosRaw, setVideosRaw] = useState(stringifyStringList(initial.videos));
  const [publishedLocal, setPublishedLocal] = useState(initial.published_at ? toDateTimeLocal(initial.published_at) : "");
  const [isFetchingMusic, setIsFetchingMusic] = useState(false);

  const createEssay = useCreateEssay();
  const updateEssay = useUpdateEssay();

  const setField = useCallback(<K extends keyof EssayPayload>(key: K, value: EssayPayload[K]) => {
    setForm(prev => ({ ...prev, [key]: value }));
  }, []);

  const appendUploadedImage = useCallback((url: string) => {
    if (!url) return;
    setImagesRaw(prev => stringifyStringList([...parseStringList(prev), url]));
  }, []);

  const handleFetchMusic = useCallback(async () => {
    const id = form.music_id.trim();
    if (!/^\d+$/.test(id)) {
      addToast({ title: "请先填写数字格式的网易云歌曲 ID", color: "warning", timeout: 3000 });
      return;
    }
    setIsFetchingMusic(true);
    try {
      const detail = await fetchMusicDetail(id);
      if (!detail) {
        addToast({ title: "没有找到这首歌，请检查 ID 或手动填写", color: "warning", timeout: 3000 });
        return;
      }
      setForm(prev => ({ ...prev, music_name: detail.name, music_artist: detail.artist, music_cover: detail.pic }));
    } finally {
      setIsFetchingMusic(false);
    }
  }, [form.music_id]);

  const handleSubmit = useCallback(async () => {
    const payload: EssayPayload = {
      ...form,
      content: form.content.trim(),
      images: parseStringList(imagesRaw),
      videos: parseStringList(videosRaw),
      address: form.address.trim(),
      source: form.source.trim(),
      link_url: form.link_url.trim(),
      link_title: form.link_title.trim(),
      music_id: form.music_id.trim(),
      music_name: form.music_name.trim(),
      music_artist: form.music_artist.trim(),
      music_cover: form.music_cover.trim(),
      published_at: fromDateTimeLocal(publishedLocal) ?? (isEdit ? initial.published_at : undefined),
    };

    if (!payload.content && payload.images.length === 0 && payload.videos.length === 0) {
      addToast({ title: "文字、图片、视频至少填写一项", color: "warning", timeout: 3000 });
      return;
    }

    try {
      if (isEdit && editItem) {
        await updateEssay.mutateAsync({ id: editItem.id, data: payload });
        addToast({ title: "已保存", color: "success", timeout: 2000 });
      } else {
        await createEssay.mutateAsync(payload);
        addToast({ title: "已发布", color: "success", timeout: 2000 });
      }
      onClose();
    } catch (error) {
      addToast({ title: error instanceof Error ? error.message : "操作失败", color: "danger", timeout: 4000 });
    }
  }, [form, imagesRaw, videosRaw, publishedLocal, isEdit, editItem, initial.published_at, createEssay, updateEssay, onClose]);

  const isSubmitting = createEssay.isPending || updateEssay.isPending;

  return (
    <>
      <ModalBody className="gap-4">
        <Textarea
          label="文字内容"
          placeholder="此刻的想法……"
          value={form.content}
          onValueChange={v => setField("content", v)}
          minRows={4}
          maxRows={10}
          maxLength={2000}
        />

        <FormStringList
          label="图片地址"
          description="最多 9 张；也可以用下方按钮上传"
          value={imagesRaw}
          onValueChange={setImagesRaw}
          placeholder="https://"
          addButtonText="添加图片地址"
          maxItems={9}
        />
        <FormImageUpload label="上传图片" value="" onValueChange={appendUploadedImage} hidePreview />

        <FormStringList
          label="视频地址"
          description="最多 4 个"
          value={videosRaw}
          onValueChange={setVideosRaw}
          placeholder="https://"
          addButtonText="添加视频地址"
          maxItems={4}
        />

        <div className="grid gap-4 sm:grid-cols-2">
          <Input label="地点" value={form.address} onValueChange={v => setField("address", v)} maxLength={50} />
          <Input label="来源" value={form.source} onValueChange={v => setField("source", v)} maxLength={50} />
          <Input label="外链地址" value={form.link_url} onValueChange={v => setField("link_url", v)} placeholder="https://" />
          <Input label="外链标题" value={form.link_title} onValueChange={v => setField("link_title", v)} maxLength={100} />
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label="网易云歌曲 ID"
            value={form.music_id}
            onValueChange={v => setField("music_id", v)}
            endContent={
              <Button size="sm" variant="flat" onPress={handleFetchMusic} isLoading={isFetchingMusic}>
                获取歌曲信息
              </Button>
            }
          />
          <Input label="歌名" value={form.music_name} onValueChange={v => setField("music_name", v)} maxLength={100} />
          <Input label="歌手" value={form.music_artist} onValueChange={v => setField("music_artist", v)} maxLength={100} />
          <Input label="封面地址" value={form.music_cover} onValueChange={v => setField("music_cover", v)} />
        </div>

        <div className="grid items-center gap-4 sm:grid-cols-2">
          <Input
            type="datetime-local"
            label="发布时间"
            description="留空则使用当前时间"
            value={publishedLocal}
            onValueChange={setPublishedLocal}
          />
          <Switch isSelected={form.is_published} onValueChange={v => setField("is_published", v)}>
            公开
          </Switch>
        </div>
      </ModalBody>

      <ModalFooter>
        <Button variant="flat" onPress={onClose}>
          取消
        </Button>
        <Button color="primary" onPress={handleSubmit} isLoading={isSubmitting}>
          {isEdit ? "保存修改" : "发布"}
        </Button>
      </ModalFooter>
    </>
  );
}
```

**Step 5：运行，确认通过**

Run: `npx vitest run src/components/admin/essays/EssayFormModal.test.tsx`
Expected: 3 个用例通过。

**Step 6：表格列** `src/components/admin/essays/EssayTableColumns.tsx`

```tsx
/**
 * 即刻短文表格列（[custom] 二开新增）
 */
"use client";

import { useCallback } from "react";
import { Button, Chip, Tooltip } from "@heroui/react";
import { Edit, Trash2 } from "lucide-react";
import { essaySummary } from "@/lib/essay";
import { formatDateTimeParts } from "@/utils/date";
import type { Essay } from "@/types/essay";

export const ESSAY_TABLE_COLUMNS = [
  { key: "content", label: "内容" },
  { key: "media", label: "附件" },
  { key: "status", label: "状态" },
  { key: "published", label: "发布时间" },
  { key: "actions", label: "操作" },
];

export function useEssayRenderCell(onAction: (item: Essay, key: "edit" | "delete") => void) {
  return useCallback(
    (item: Essay, columnKey: React.Key) => {
      switch (columnKey) {
        case "content":
          return <p className="max-w-[360px] truncate text-sm">{essaySummary(item) || "-"}</p>;
        case "media": {
          const parts = [
            item.images.length > 0 && `${item.images.length} 图`,
            item.videos.length > 0 && `${item.videos.length} 视频`,
            item.music_id && "音乐",
            item.link_url && "链接",
          ].filter(Boolean);
          return <span className="text-xs text-muted-foreground">{parts.join(" · ") || "-"}</span>;
        }
        case "status":
          return (
            <Chip size="sm" variant="flat" color={item.is_published ? "success" : "default"}>
              {item.is_published ? "公开" : "隐藏"}
            </Chip>
          );
        case "published": {
          const { date, time } = formatDateTimeParts(item.published_at);
          return (
            <div className="text-xs tabular-nums text-muted-foreground">
              <div>{date}</div>
              <div className="text-muted-foreground/60">{time}</div>
            </div>
          );
        }
        case "actions":
          return (
            <div className="flex items-center justify-center gap-1">
              <Tooltip content="编辑" size="sm">
                <Button isIconOnly size="sm" variant="light" radius="full" aria-label="编辑" onPress={() => onAction(item, "edit")}>
                  <Edit className="h-3.5 w-3.5" />
                </Button>
              </Tooltip>
              <Tooltip content="删除" size="sm" color="danger">
                <Button
                  isIconOnly
                  size="sm"
                  variant="light"
                  radius="full"
                  color="danger"
                  aria-label="删除"
                  onPress={() => onAction(item, "delete")}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </Tooltip>
            </div>
          );
        default:
          return null;
      }
    },
    [onAction]
  );
}
```

**Step 7：页面** `src/app/admin/essays/page.tsx`

```tsx
/**
 * 即刻短文管理（[custom] 二开新增）
 */
"use client";

import { useCallback, useState } from "react";
import {
  Button,
  Pagination,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableColumn,
  TableHeader,
  TableRow,
  addToast,
  useDisclosure,
} from "@heroui/react";
import { Feather, Plus, ShieldAlert } from "lucide-react";
import { ConfirmDialog } from "@/components/admin/ConfirmDialog";
import { TableEmptyState } from "@/components/admin/TableEmptyState";
import EssayFormModal from "@/components/admin/essays/EssayFormModal";
import { ESSAY_TABLE_COLUMNS, useEssayRenderCell } from "@/components/admin/essays/EssayTableColumns";
import { useAdminEssays, useDeleteEssay } from "@/hooks/queries/use-essays";
import type { Essay } from "@/types/essay";

const PAGE_SIZE = 10;

export default function EssaysAdminPage() {
  const [page, setPage] = useState(1);
  const [editItem, setEditItem] = useState<Essay | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<Essay | null>(null);
  const formModal = useDisclosure();
  const deleteModal = useDisclosure();

  const { data, isLoading, isFetching } = useAdminEssays({ page, pageSize: PAGE_SIZE });
  const deleteEssay = useDeleteEssay();
  const essays = data?.list ?? [];
  const totalPages = Math.max(1, Math.ceil((data?.total ?? 0) / PAGE_SIZE));

  const openNew = useCallback(() => {
    setEditItem(null);
    formModal.onOpen();
  }, [formModal]);

  const handleAction = useCallback(
    (item: Essay, key: "edit" | "delete") => {
      if (key === "edit") {
        setEditItem(item);
        formModal.onOpen();
      } else {
        setDeleteTarget(item);
        deleteModal.onOpen();
      }
    },
    [formModal, deleteModal]
  );
  const renderCell = useEssayRenderCell(handleAction);

  const handleDeleteConfirm = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      await deleteEssay.mutateAsync(deleteTarget.id);
      addToast({ title: "已删除", color: "success", timeout: 2000 });
    } catch (error) {
      addToast({ title: error instanceof Error ? error.message : "删除失败", color: "danger", timeout: 3000 });
    }
    deleteModal.onClose();
    setDeleteTarget(null);
  }, [deleteTarget, deleteEssay, deleteModal]);

  if (isLoading) {
    return (
      <div className="flex h-full items-center justify-center">
        <Spinner label="加载中..." />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-foreground">即刻短文</h1>
          <p className="mt-1 text-xs text-muted-foreground">共 {data?.total ?? 0} 条，前台 /essay 和首页滚动条只显示公开的</p>
        </div>
        <Button size="sm" color="primary" startContent={<Plus className="h-3.5 w-3.5" />} onPress={openNew}>
          发布短文
        </Button>
      </div>

      <Table
        aria-label="即刻短文列表"
        bottomContent={
          totalPages > 1 && (
            <div className="flex justify-center">
              <Pagination isCompact showControls color="primary" page={page} total={totalPages} onChange={setPage} />
            </div>
          )
        }
      >
        <TableHeader columns={ESSAY_TABLE_COLUMNS}>
          {column => (
            <TableColumn key={column.key} align={column.key === "actions" ? "center" : "start"}>
              {column.label}
            </TableColumn>
          )}
        </TableHeader>
        <TableBody
          items={essays}
          isLoading={isFetching}
          loadingContent={<Spinner size="sm" />}
          emptyContent={
            <TableEmptyState
              icon={Feather}
              hasFilter={false}
              filterEmptyText=""
              emptyText="还没有短文"
              emptyHint="点击「发布短文」写第一条"
              action={{ label: "发布短文", onPress: openNew }}
            />
          }
        >
          {item => <TableRow key={item.id}>{columnKey => <TableCell>{renderCell(item, columnKey)}</TableCell>}</TableRow>}
        </TableBody>
      </Table>

      <EssayFormModal isOpen={formModal.isOpen} onClose={formModal.onClose} editItem={editItem} />

      <ConfirmDialog
        isOpen={deleteModal.isOpen}
        onOpenChange={deleteModal.onOpenChange}
        title="删除短文"
        description="确定要删除这条短文吗？此操作不可撤销。"
        confirmText="删除"
        confirmColor="danger"
        icon={<ShieldAlert className="h-5 w-5 text-danger" />}
        iconBg="bg-danger-50"
        loading={deleteEssay.isPending}
        onConfirm={handleDeleteConfirm}
      />
    </div>
  );
}
```

> `TableEmptyState`、`ConfirmDialog` 的属性以 `src/app/admin/doc-series/page.tsx` 里的用法为准；`tsc` 报属性不匹配时照那里改。

**Step 8：类型检查、lint、全量单测**

Run: `npx tsc --noEmit 2>&1 | tail -3` → 不比 F0 基线多。
Run: `npx eslint src/app/admin/essays src/components/admin/essays src/config/admin-menu.ts src/components/admin/article-editor/extensions/music-block.tsx` → 无输出。
Run: `npx vitest run 2>&1 | tail -6` → 通过数 = F0 基线 + 本阶段新增，失败数不比基线多。

**Step 9：浏览器验证（需要用户已注册管理员并登录）**

1. 侧边栏「内容管理」出现「即刻短文」，点进去是空列表
2. 发布一条只有文字的短文 → 列表出现，状态「公开」
3. 发布一条带 2 张图片地址、地点「厦门」、来源的短文
4. 发布一条带网易云歌曲 ID 的短文，点「获取歌曲信息」自动填好歌名/歌手/封面
5. 发布一条「公开」关闭的短文
6. 提交空表单 → 黄色提示，不发请求；外链填 `javascript:alert(1)` → 红色提示「外链地址必须以 http:// 或 https:// 开头…」（后端校验生效）
7. 编辑、删除各一次
8. 回到首页和 `/essay` 复查 Task F3 Step 6 和 Task F4 Step 6 中「有数据后」的检查项；隐藏的那条两处都不出现

每一项截图留证。

**Step 10：提交**

```bash
git add src/app/admin/essays src/components/admin/essays src/config/admin-menu.ts src/components/admin/article-editor/extensions/music-block.tsx
git commit -m "feat(essay): 新增即刻短文后台管理页"
```

---

## 阶段 2 验收（停下来向用户汇报）

1. F0 基线 vs 现在：`tsc` 错误数、`vitest` 通过/失败数
2. Task F5 Step 9 的截图（后台列表、表单、首页滚动条、`/essay` 页面桌面版和手机版）
3. 本阶段提交列表：`git log --oneline main..custom`
4. 对上游文件的改动：`git diff main..custom --stat -- src/config src/components/home/HomePageContent.tsx src/components/admin/article-editor` 只涉及 3 个挂载点文件
