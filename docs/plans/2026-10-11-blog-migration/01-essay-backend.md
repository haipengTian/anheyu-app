# 阶段 1：即刻短文后端

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在后端新增 `essays` 表和 `/api/essays`（管理）、`/api/public/essays`（公开）接口。

**Architecture:** 照搬 `doc_series` 模块分层：`ent/schema/essay.go` → `pkg/domain/model/essay.go` + `pkg/domain/repository/essay_repo.go` → `internal/infra/persistence/ent/essay_repo.go` → `pkg/service/essay` → `pkg/handler/essay` → `internal/infra/router/essay_router.go`。路由用独立函数注册，不改上游的 `Router` 结构体。

**仓库：** `E:\workspace\code\blog\anheyu-app`，分支 `custom`。

**与设计文档的两处偏差（已有理由）：**
- **不用软删除**：上游的 `SoftDeleteMixin` 只拦截删除，查询时每处都要手写 `DeletedAtIsNil()`，对短文来说收益很小。后台删除前有确认弹窗。
- **ID 用数字，不用 `idgen` 公共 ID**：用 `idgen` 要在上游的 `pkg/idgen/idgen.go` 里新增实体类型常量，且编号可能和上游 PRO 版冲突。相册模块也是直接用数字 ID。
- **字段 `from` 改名为 `source`**：`from` 是 SQL 保留字。
- **新增 `music_name`、`music_artist`、`music_cover`**：前台显示音乐卡片时不会再去查歌曲信息（文章编辑器也是在编辑时存下这些信息），所以要一起存。

**校验规则（service 层，唯一权威）：**

| 字段 | 规则 |
|---|---|
| `content`、`images`、`videos` | 三者至少一项非空；`content` ≤ 2000 字 |
| `images` | ≤ 9 个；每个都是 http(s) URL，≤ 500 字节；空白项自动去掉 |
| `videos` | ≤ 4 个；规则同上 |
| `address`、`source` | ≤ 50 字 |
| `link_url` | 可空；非空时必须是 http(s) URL，≤ 500 字节 |
| `link_title` | ≤ 100 字；非空时 `link_url` 必须非空 |
| `music_id` | 可空；非空时只能是数字，≤ 20 位 |
| `music_name`、`music_artist` | ≤ 100 字；`music_cover` 同 URL 规则；三者非空时 `music_id` 必须非空 |
| `published_at` | 创建时不传 = 当前时间；更新时不传 = 保持原值 |
| `is_published` | 创建时不传 = `true`；更新时不传 = 保持原值 |

---

### Task B1：Ent schema 与代码生成

**Files:**
- Create: `ent/schema/essay.go`
- Generate: `ent/` 下的生成代码（`go generate ./ent`）

**Step 1：写 schema**

```go
/*
 * @Description: 即刻短文表（[custom] 二开新增）
 */
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Essay holds the schema definition for the Essay entity.
type Essay struct {
	ent.Schema
}

// Annotations of the Essay.
func (Essay) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.WithComments(true),
		schema.Comment("即刻短文表"),
	}
}

// Fields of the Essay.
// 长度限制统一由 service 层按「字数」校验，这里不用 MaxLen（ent 的 MaxLen 按字节计）。
func (Essay) Fields() []ent.Field {
	return []ent.Field{
		field.Uint("id"),
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
		field.Text("content").Default("").Comment("文字内容"),
		field.Time("published_at").Default(time.Now).Comment("发布时间，可手动指定"),
		field.JSON("images", []string{}).Optional().Comment("图片 URL 列表"),
		field.JSON("videos", []string{}).Optional().Comment("视频 URL 列表"),
		field.String("address").Default("").Comment("地点"),
		field.String("source").Default("").Comment("来源"),
		field.String("link_url").Default("").Comment("外链地址"),
		field.String("link_title").Default("").Comment("外链标题"),
		field.String("music_id").Default("").Comment("网易云歌曲 ID"),
		field.String("music_name").Default("").Comment("歌名"),
		field.String("music_artist").Default("").Comment("歌手"),
		field.String("music_cover").Default("").Comment("封面 URL"),
		field.Bool("is_published").Default(true).Comment("是否公开"),
	}
}

// Indexes of the Essay.
func (Essay) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("is_published", "published_at"),
	}
}
```

**Step 2：生成代码**

Run: `go generate ./ent`
Expected: 无报错；`git status --short ent/` 里出现新目录 `ent/essay/`、新文件 `ent/essay*.go`，以及 `ent/client.go`、`ent/mutation.go`、`ent/migrate/schema.go` 等共享文件的修改。

**Step 3：确认生成的代码能编译、上游测试不受影响**

Run: `go build ./... && go test ./ent/... ./internal/infra/persistence/...`
Expected: 编译通过，测试全部 `ok`。

**Step 4：提交**

```bash
git add ent/
git commit -m "feat(essay): 新增即刻短文 ent schema 并生成代码"
```

---

### Task B2：领域模型与仓储接口

**Files:**
- Create: `pkg/domain/model/essay.go`
- Create: `pkg/domain/repository/essay_repo.go`

**Step 1：领域模型和 DTO**

`pkg/domain/model/essay.go`：

```go
/*
 * @Description: 即刻短文领域模型（[custom] 二开新增）
 */
package model

import "time"

// Essay 是即刻短文的核心领域模型
type Essay struct {
	ID          uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Content     string
	PublishedAt time.Time
	Images      []string
	Videos      []string
	Address     string
	Source      string
	LinkURL     string
	LinkTitle   string
	MusicID     string
	MusicName   string
	MusicArtist string
	MusicCover  string
	IsPublished bool
}

// EssayRequest 是创建/更新即刻短文的请求体。
// 更新时整体覆盖，只有 PublishedAt / IsPublished 为 nil 时保留原值。
type EssayRequest struct {
	Content     string     `json:"content"`
	PublishedAt *time.Time `json:"published_at"`
	Images      []string   `json:"images"`
	Videos      []string   `json:"videos"`
	Address     string     `json:"address"`
	Source      string     `json:"source"`
	LinkURL     string     `json:"link_url"`
	LinkTitle   string     `json:"link_title"`
	MusicID     string     `json:"music_id"`
	MusicName   string     `json:"music_name"`
	MusicArtist string     `json:"music_artist"`
	MusicCover  string     `json:"music_cover"`
	IsPublished *bool      `json:"is_published"`
}

// EssayResponse 是即刻短文的 API 响应结构
type EssayResponse struct {
	ID          uint      `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Content     string    `json:"content"`
	PublishedAt time.Time `json:"published_at"`
	Images      []string  `json:"images"`
	Videos      []string  `json:"videos"`
	Address     string    `json:"address"`
	Source      string    `json:"source"`
	LinkURL     string    `json:"link_url"`
	LinkTitle   string    `json:"link_title"`
	MusicID     string    `json:"music_id"`
	MusicName   string    `json:"music_name"`
	MusicArtist string    `json:"music_artist"`
	MusicCover  string    `json:"music_cover"`
	IsPublished bool      `json:"is_published"`
}

// EssayListResponse 是即刻短文列表的 API 响应结构
type EssayListResponse struct {
	List     []EssayResponse `json:"list"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"pageSize"`
}

// ListEssaysOptions 定义了查询短文列表的选项（Page/PageSize 由 service 规整后传入）
type ListEssaysOptions struct {
	Page          int
	PageSize      int
	OnlyPublished bool
}
```

**Step 2：仓储接口**

`pkg/domain/repository/essay_repo.go`：

```go
/*
 * @Description: 即刻短文仓储接口（[custom] 二开新增）
 */
package repository

import (
	"context"
	"errors"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// ErrEssayNotFound 表示短文不存在
var ErrEssayNotFound = errors.New("短文不存在")

// EssayRepository 定义了即刻短文的数据操作契约。
// 列表按 published_at 倒序、id 倒序返回。
type EssayRepository interface {
	Create(ctx context.Context, essay *model.Essay) (*model.Essay, error)
	Update(ctx context.Context, essay *model.Essay) (*model.Essay, error)
	Delete(ctx context.Context, id uint) error
	FindByID(ctx context.Context, id uint) (*model.Essay, error)
	List(ctx context.Context, opts model.ListEssaysOptions) ([]*model.Essay, int64, error)
}
```

**Step 3：编译**

Run: `go build ./pkg/...`
Expected: 通过。

**Step 4：提交**

```bash
git add pkg/domain/model/essay.go pkg/domain/repository/essay_repo.go
git commit -m "feat(essay): 新增即刻短文领域模型与仓储接口"
```

---

### Task B3：Ent 仓储实现

**Files:**
- Test: `internal/infra/persistence/ent/essay_repo_test.go`
- Create: `internal/infra/persistence/ent/essay_repo.go`

**Step 1：写失败的测试**

用内存 SQLite（和上游 `article_repo_test.go` 同样的方式）。每个测试用独立的库名，避免 `cache=shared` 串数据。

```go
package ent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent/enttest"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

func newEssayTestRepo(t *testing.T) repository.EssayRepository {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client := enttest.Open(t, "sqlite3", "file:essay_repo_"+name+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })
	return NewEssayRepo(client)
}

func sampleEssay(content string, publishedAt time.Time, published bool) *model.Essay {
	return &model.Essay{
		Content:     content,
		PublishedAt: publishedAt,
		Images:      []string{"https://img.example.com/a.jpg"},
		Videos:      []string{},
		Address:     "厦门",
		Source:      "super-thp",
		IsPublished: published,
	}
}

func TestEssayRepoCreateAndFind(t *testing.T) {
	repo := newEssayTestRepo(t)
	ctx := context.Background()
	at := time.Date(2025, 7, 1, 12, 0, 0, 0, time.UTC)

	created, err := repo.Create(ctx, sampleEssay("海边", at, true))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected generated id")
	}

	got, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Content != "海边" || got.Address != "厦门" || got.Source != "super-thp" {
		t.Fatalf("unexpected essay: %+v", got)
	}
	if len(got.Images) != 1 || got.Images[0] != "https://img.example.com/a.jpg" {
		t.Fatalf("images not round-tripped: %v", got.Images)
	}
	if got.Videos == nil {
		t.Fatal("videos should be an empty slice, not nil")
	}
	if !got.PublishedAt.Equal(at) {
		t.Fatalf("published_at = %v, want %v", got.PublishedAt, at)
	}
}

func TestEssayRepoListFiltersAndOrders(t *testing.T) {
	repo := newEssayTestRepo(t)
	ctx := context.Background()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, published := range []bool{true, false, true, true} {
		content := fmt.Sprintf("第%d条", i+1)
		if _, err := repo.Create(ctx, sampleEssay(content, base.AddDate(0, 0, i), published)); err != nil {
			t.Fatalf("Create %s: %v", content, err)
		}
	}

	items, total, err := repo.List(ctx, model.ListEssaysOptions{Page: 1, PageSize: 2, OnlyPublished: true})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(items) != 2 || items[0].Content != "第4条" || items[1].Content != "第3条" {
		t.Fatalf("unexpected order/page: %+v", items)
	}

	all, totalAll, err := repo.List(ctx, model.ListEssaysOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if totalAll != 4 || len(all) != 4 {
		t.Fatalf("admin list should include drafts, got total=%d len=%d", totalAll, len(all))
	}
}

func TestEssayRepoUpdateAndDelete(t *testing.T) {
	repo := newEssayTestRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, sampleEssay("原文", time.Now(), true))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	created.Content = "改过"
	created.Images = []string{}
	created.IsPublished = false
	updated, err := repo.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Content != "改过" || updated.IsPublished || len(updated.Images) != 0 {
		t.Fatalf("update not applied: %+v", updated)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.FindByID(ctx, created.ID); !errors.Is(err, repository.ErrEssayNotFound) {
		t.Fatalf("FindByID after delete: err = %v, want ErrEssayNotFound", err)
	}
}

func TestEssayRepoMissingReturnsNotFound(t *testing.T) {
	repo := newEssayTestRepo(t)
	ctx := context.Background()

	if _, err := repo.Update(ctx, &model.Essay{ID: 999, PublishedAt: time.Now()}); !errors.Is(err, repository.ErrEssayNotFound) {
		t.Fatalf("Update missing: err = %v", err)
	}
	if err := repo.Delete(ctx, 999); !errors.Is(err, repository.ErrEssayNotFound) {
		t.Fatalf("Delete missing: err = %v", err)
	}
}
```

**Step 2：运行，确认失败**

Run: `go test ./internal/infra/persistence/ent/ -run TestEssayRepo -v`
Expected: 编译失败，`undefined: NewEssayRepo`。

**Step 3：实现**

`internal/infra/persistence/ent/essay_repo.go`：

```go
/*
 * @Description: 即刻短文仓储实现（[custom] 二开新增）
 */
package ent

import (
	"context"

	"github.com/anzhiyu-c/anheyu-app/ent"
	"github.com/anzhiyu-c/anheyu-app/ent/essay"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

type essayRepo struct {
	db *ent.Client
}

// NewEssayRepo 是 essayRepo 的构造函数。
func NewEssayRepo(db *ent.Client) repository.EssayRepository {
	return &essayRepo{db: db}
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func (r *essayRepo) toModel(e *ent.Essay) *model.Essay {
	return &model.Essay{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Content:     e.Content,
		PublishedAt: e.PublishedAt,
		Images:      nonNilStrings(e.Images),
		Videos:      nonNilStrings(e.Videos),
		Address:     e.Address,
		Source:      e.Source,
		LinkURL:     e.LinkURL,
		LinkTitle:   e.LinkTitle,
		MusicID:     e.MusicID,
		MusicName:   e.MusicName,
		MusicArtist: e.MusicArtist,
		MusicCover:  e.MusicCover,
		IsPublished: e.IsPublished,
	}
}

func mapEssayErr(err error) error {
	if ent.IsNotFound(err) {
		return repository.ErrEssayNotFound
	}
	return err
}

// Create 创建一条短文
func (r *essayRepo) Create(ctx context.Context, m *model.Essay) (*model.Essay, error) {
	created, err := r.db.Essay.Create().
		SetContent(m.Content).
		SetPublishedAt(m.PublishedAt).
		SetImages(nonNilStrings(m.Images)).
		SetVideos(nonNilStrings(m.Videos)).
		SetAddress(m.Address).
		SetSource(m.Source).
		SetLinkURL(m.LinkURL).
		SetLinkTitle(m.LinkTitle).
		SetMusicID(m.MusicID).
		SetMusicName(m.MusicName).
		SetMusicArtist(m.MusicArtist).
		SetMusicCover(m.MusicCover).
		SetIsPublished(m.IsPublished).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return r.toModel(created), nil
}

// Update 整体覆盖一条短文
func (r *essayRepo) Update(ctx context.Context, m *model.Essay) (*model.Essay, error) {
	updated, err := r.db.Essay.UpdateOneID(m.ID).
		SetContent(m.Content).
		SetPublishedAt(m.PublishedAt).
		SetImages(nonNilStrings(m.Images)).
		SetVideos(nonNilStrings(m.Videos)).
		SetAddress(m.Address).
		SetSource(m.Source).
		SetLinkURL(m.LinkURL).
		SetLinkTitle(m.LinkTitle).
		SetMusicID(m.MusicID).
		SetMusicName(m.MusicName).
		SetMusicArtist(m.MusicArtist).
		SetMusicCover(m.MusicCover).
		SetIsPublished(m.IsPublished).
		Save(ctx)
	if err != nil {
		return nil, mapEssayErr(err)
	}
	return r.toModel(updated), nil
}

// Delete 删除一条短文
func (r *essayRepo) Delete(ctx context.Context, id uint) error {
	return mapEssayErr(r.db.Essay.DeleteOneID(id).Exec(ctx))
}

// FindByID 按 ID 查询
func (r *essayRepo) FindByID(ctx context.Context, id uint) (*model.Essay, error) {
	e, err := r.db.Essay.Get(ctx, id)
	if err != nil {
		return nil, mapEssayErr(err)
	}
	return r.toModel(e), nil
}

// List 分页查询，按发布时间倒序
func (r *essayRepo) List(ctx context.Context, opts model.ListEssaysOptions) ([]*model.Essay, int64, error) {
	query := r.db.Essay.Query()
	if opts.OnlyPublished {
		query = query.Where(essay.IsPublished(true))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	entities, err := query.
		Order(ent.Desc(essay.FieldPublishedAt), ent.Desc(essay.FieldID)).
		Offset((opts.Page - 1) * opts.PageSize).
		Limit(opts.PageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}

	items := make([]*model.Essay, len(entities))
	for i, e := range entities {
		items[i] = r.toModel(e)
	}
	return items, int64(total), nil
}
```

> 生成代码里的方法名以 Task B1 实际生成的为准（例如 `link_url` 生成的是 `SetLinkURL` 还是 `SetLinkUrl`）。编译报找不到方法时，打开 `ent/essay_create.go` 对照后**改这里，不改生成代码**。

**Step 4：运行，确认通过**

Run: `go test ./internal/infra/persistence/ent/ -run TestEssayRepo -v`
Expected: 4 个测试全部 `PASS`。

**Step 5：提交**

```bash
git add internal/infra/persistence/ent/essay_repo.go internal/infra/persistence/ent/essay_repo_test.go
git commit -m "feat(essay): 实现即刻短文 ent 仓储"
```

---

### Task B4：Service（校验与业务规则）

**Files:**
- Test: `pkg/service/essay/service_test.go`
- Create: `pkg/service/essay/service.go`

**Step 1：写失败的测试**

用内存假仓储（上游 `pkg/service/link/service_test.go` 也是这种写法），只测 service 自己的规则。

```go
package essay

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

type fakeEssayRepo struct {
	items  map[uint]model.Essay
	nextID uint
}

func newFakeEssayRepo() *fakeEssayRepo {
	return &fakeEssayRepo{items: map[uint]model.Essay{}, nextID: 1}
}

func (f *fakeEssayRepo) Create(_ context.Context, e *model.Essay) (*model.Essay, error) {
	c := *e
	c.ID = f.nextID
	f.nextID++
	f.items[c.ID] = c
	return &c, nil
}

func (f *fakeEssayRepo) Update(_ context.Context, e *model.Essay) (*model.Essay, error) {
	if _, ok := f.items[e.ID]; !ok {
		return nil, repository.ErrEssayNotFound
	}
	f.items[e.ID] = *e
	c := *e
	return &c, nil
}

func (f *fakeEssayRepo) Delete(_ context.Context, id uint) error {
	if _, ok := f.items[id]; !ok {
		return repository.ErrEssayNotFound
	}
	delete(f.items, id)
	return nil
}

func (f *fakeEssayRepo) FindByID(_ context.Context, id uint) (*model.Essay, error) {
	e, ok := f.items[id]
	if !ok {
		return nil, repository.ErrEssayNotFound
	}
	return &e, nil
}

func (f *fakeEssayRepo) List(_ context.Context, opts model.ListEssaysOptions) ([]*model.Essay, int64, error) {
	var all []*model.Essay
	for _, e := range f.items {
		if opts.OnlyPublished && !e.IsPublished {
			continue
		}
		c := e
		all = append(all, &c)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].PublishedAt.After(all[j].PublishedAt) })
	start := min((opts.Page-1)*opts.PageSize, len(all))
	end := min(start+opts.PageSize, len(all))
	return all[start:end], int64(len(all)), nil
}

var fixedNow = time.Date(2026, 10, 11, 20, 0, 0, 0, time.UTC)

func newTestService() (*Service, *fakeEssayRepo) {
	repo := newFakeEssayRepo()
	svc := NewService(repo)
	svc.now = func() time.Time { return fixedNow }
	return svc, repo
}

func boolPtr(b bool) *bool { return &b }

func TestCreateRejectsInvalidInput(t *testing.T) {
	cases := map[string]model.EssayRequest{
		"全部为空":            {},
		"只有空白":            {Content: "   ", Images: []string{"  "}},
		"文字超长":            {Content: strings.Repeat("字", 2001)},
		"图片不是 http":       {Images: []string{"ftp://a/b.jpg"}},
		"图片太多":            {Images: strings.Split(strings.TrimSuffix(strings.Repeat("https://a/b.jpg,", 10), ","), ",")},
		"视频太多":            {Videos: []string{"https://a/1.mp4", "https://a/2.mp4", "https://a/3.mp4", "https://a/4.mp4", "https://a/5.mp4"}},
		"地点超长":            {Content: "x", Address: strings.Repeat("地", 51)},
		"外链是 javascript": {Content: "x", LinkURL: "javascript:alert(1)"},
		"有外链标题没外链":        {Content: "x", LinkTitle: "标题"},
		"歌曲 ID 不是数字":      {Content: "x", MusicID: "abc"},
		"有歌名没歌曲 ID":       {Content: "x", MusicName: "晴天"},
		"封面不是 URL":         {Content: "x", MusicID: "123", MusicCover: "cover.jpg"},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			svc, _ := newTestService()
			_, err := svc.Create(context.Background(), &req)
			var vErr *ValidationError
			if !errors.As(err, &vErr) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
		})
	}
}

func TestCreateNormalizesAndDefaults(t *testing.T) {
	svc, _ := newTestService()

	got, err := svc.Create(context.Background(), &model.EssayRequest{
		Content: "  海边  ",
		Images:  []string{" https://a/b.jpg ", "", "  "},
		Address: " 厦门 ",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.Content != "海边" || got.Address != "厦门" {
		t.Fatalf("not trimmed: %+v", got)
	}
	if len(got.Images) != 1 || got.Images[0] != "https://a/b.jpg" {
		t.Fatalf("images = %v", got.Images)
	}
	if got.Videos == nil {
		t.Fatal("videos should default to empty slice")
	}
	if !got.IsPublished {
		t.Fatal("is_published should default to true")
	}
	if !got.PublishedAt.Equal(fixedNow) {
		t.Fatalf("published_at = %v, want now", got.PublishedAt)
	}
}

func TestCreateKeepsGivenPublishedAtAndVisibility(t *testing.T) {
	svc, _ := newTestService()
	at := time.Date(2025, 7, 1, 4, 0, 0, 0, time.UTC)

	got, err := svc.Create(context.Background(), &model.EssayRequest{Content: "旧短文", PublishedAt: &at, IsPublished: boolPtr(false)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !got.PublishedAt.Equal(at) || got.IsPublished {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestUpdateKeepsOmittedPublishedAtAndVisibility(t *testing.T) {
	svc, _ := newTestService()
	at := time.Date(2025, 7, 1, 4, 0, 0, 0, time.UTC)
	created, err := svc.Create(context.Background(), &model.EssayRequest{Content: "原文", PublishedAt: &at, IsPublished: boolPtr(false)})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(context.Background(), created.ID, &model.EssayRequest{Content: "改过"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Content != "改过" || !updated.PublishedAt.Equal(at) || updated.IsPublished {
		t.Fatalf("unexpected: %+v", updated)
	}
}

func TestMissingEssayReturnsNotFound(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	if _, err := svc.Get(ctx, 42); !errors.Is(err, ErrEssayNotFound) {
		t.Fatalf("Get: err = %v", err)
	}
	if _, err := svc.Update(ctx, 42, &model.EssayRequest{Content: "x"}); !errors.Is(err, ErrEssayNotFound) {
		t.Fatalf("Update: err = %v", err)
	}
	if err := svc.Delete(ctx, 42); !errors.Is(err, ErrEssayNotFound) {
		t.Fatalf("Delete: err = %v", err)
	}
}

func TestListPublicHidesDraftsAndNormalizesPaging(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	for i, published := range []bool{true, false, true} {
		at := fixedNow.Add(time.Duration(-i) * time.Hour)
		if _, err := svc.Create(ctx, &model.EssayRequest{Content: "x", PublishedAt: &at, IsPublished: boolPtr(published)}); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	public, err := svc.ListPublic(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	if public.Total != 2 || len(public.List) != 2 {
		t.Fatalf("public total=%d len=%d, want 2", public.Total, len(public.List))
	}
	if public.Page != 1 || public.PageSize != defaultPageSize {
		t.Fatalf("paging not normalized: page=%d size=%d", public.Page, public.PageSize)
	}

	admin, err := svc.ListAdmin(ctx, 1, 1000)
	if err != nil {
		t.Fatalf("ListAdmin: %v", err)
	}
	if admin.Total != 3 || admin.PageSize != maxPageSize {
		t.Fatalf("admin total=%d size=%d", admin.Total, admin.PageSize)
	}
}
```

**Step 2：运行，确认失败**

Run: `go test ./pkg/service/essay/ -v`
Expected: 编译失败，`undefined: Service` 等。

**Step 3：实现**

`pkg/service/essay/service.go`：

```go
/*
 * @Description: 即刻短文服务（[custom] 二开新增）
 */
package essay

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
)

const (
	maxContentRunes  = 2000
	maxImages        = 9
	maxVideos        = 4
	maxURLBytes      = 500
	maxShortRunes    = 50
	maxTitleRunes    = 100
	maxMusicIDLength = 20
	defaultPageSize  = 20
	maxPageSize      = 100
)

// ErrEssayNotFound 表示短文不存在（handler 映射为 404）
var ErrEssayNotFound = repository.ErrEssayNotFound

// ValidationError 表示请求参数不合法（handler 映射为 400），消息可直接展示给用户
type ValidationError struct {
	msg string
}

func (e *ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

// Service 封装了即刻短文的业务逻辑
type Service struct {
	repo repository.EssayRepository
	now  func() time.Time
}

// NewService 是 Service 的构造函数
func NewService(repo repository.EssayRepository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Create 创建短文
func (s *Service) Create(ctx context.Context, req *model.EssayRequest) (*model.EssayResponse, error) {
	e, err := s.buildEssay(req, nil)
	if err != nil {
		return nil, err
	}
	created, err := s.repo.Create(ctx, e)
	if err != nil {
		return nil, fmt.Errorf("创建短文失败: %w", err)
	}
	return toResponse(created), nil
}

// Update 整体更新短文；PublishedAt / IsPublished 不传时保留原值
func (s *Service) Update(ctx context.Context, id uint, req *model.EssayRequest) (*model.EssayResponse, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	e, err := s.buildEssay(req, existing)
	if err != nil {
		return nil, err
	}
	e.ID = id
	updated, err := s.repo.Update(ctx, e)
	if err != nil {
		return nil, err
	}
	return toResponse(updated), nil
}

// Delete 删除短文
func (s *Service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

// Get 获取单条短文（后台用，含未公开的）
func (s *Service) Get(ctx context.Context, id uint) (*model.EssayResponse, error) {
	e, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toResponse(e), nil
}

// ListAdmin 后台列表，包含未公开的短文
func (s *Service) ListAdmin(ctx context.Context, page, pageSize int) (*model.EssayListResponse, error) {
	return s.list(ctx, page, pageSize, false)
}

// ListPublic 前台列表，只返回公开的短文
func (s *Service) ListPublic(ctx context.Context, page, pageSize int) (*model.EssayListResponse, error) {
	return s.list(ctx, page, pageSize, true)
}

func (s *Service) list(ctx context.Context, page, pageSize int, onlyPublished bool) (*model.EssayListResponse, error) {
	page, pageSize = normalizePaging(page, pageSize)
	items, total, err := s.repo.List(ctx, model.ListEssaysOptions{Page: page, PageSize: pageSize, OnlyPublished: onlyPublished})
	if err != nil {
		return nil, fmt.Errorf("查询短文列表失败: %w", err)
	}
	list := make([]model.EssayResponse, len(items))
	for i, e := range items {
		list[i] = *toResponse(e)
	}
	return &model.EssayListResponse{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// buildEssay 校验并规整请求；existing 非 nil 表示更新
func (s *Service) buildEssay(req *model.EssayRequest, existing *model.Essay) (*model.Essay, error) {
	e := &model.Essay{
		Content:     strings.TrimSpace(req.Content),
		Address:     strings.TrimSpace(req.Address),
		Source:      strings.TrimSpace(req.Source),
		LinkURL:     strings.TrimSpace(req.LinkURL),
		LinkTitle:   strings.TrimSpace(req.LinkTitle),
		MusicID:     strings.TrimSpace(req.MusicID),
		MusicName:   strings.TrimSpace(req.MusicName),
		MusicArtist: strings.TrimSpace(req.MusicArtist),
		MusicCover:  strings.TrimSpace(req.MusicCover),
	}

	var err error
	if e.Images, err = cleanURLList(req.Images, "图片", maxImages); err != nil {
		return nil, err
	}
	if e.Videos, err = cleanURLList(req.Videos, "视频", maxVideos); err != nil {
		return nil, err
	}
	if e.Content == "" && len(e.Images) == 0 && len(e.Videos) == 0 {
		return nil, invalid("文字、图片、视频至少填写一项")
	}
	if err := checkRunes("文字内容", e.Content, maxContentRunes); err != nil {
		return nil, err
	}
	if err := checkRunes("地点", e.Address, maxShortRunes); err != nil {
		return nil, err
	}
	if err := checkRunes("来源", e.Source, maxShortRunes); err != nil {
		return nil, err
	}
	if err := checkOptionalURL("外链地址", e.LinkURL); err != nil {
		return nil, err
	}
	if err := checkRunes("外链标题", e.LinkTitle, maxTitleRunes); err != nil {
		return nil, err
	}
	if e.LinkTitle != "" && e.LinkURL == "" {
		return nil, invalid("填写外链标题时必须填写外链地址")
	}
	if err := checkMusic(e); err != nil {
		return nil, err
	}

	switch {
	case req.PublishedAt != nil:
		e.PublishedAt = *req.PublishedAt
	case existing != nil:
		e.PublishedAt = existing.PublishedAt
	default:
		e.PublishedAt = s.now()
	}
	switch {
	case req.IsPublished != nil:
		e.IsPublished = *req.IsPublished
	case existing != nil:
		e.IsPublished = existing.IsPublished
	default:
		e.IsPublished = true
	}
	return e, nil
}

func checkMusic(e *model.Essay) error {
	if e.MusicID == "" {
		if e.MusicName != "" || e.MusicArtist != "" || e.MusicCover != "" {
			return invalid("填写歌曲信息时必须填写网易云歌曲 ID")
		}
		return nil
	}
	if len(e.MusicID) > maxMusicIDLength || strings.Trim(e.MusicID, "0123456789") != "" {
		return invalid("网易云歌曲 ID 只能是数字，且不超过 %d 位", maxMusicIDLength)
	}
	if err := checkRunes("歌名", e.MusicName, maxTitleRunes); err != nil {
		return err
	}
	if err := checkRunes("歌手", e.MusicArtist, maxTitleRunes); err != nil {
		return err
	}
	return checkOptionalURL("歌曲封面", e.MusicCover)
}

func cleanURLList(raw []string, label string, limit int) ([]string, error) {
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if !isHTTPURL(item) {
			return nil, invalid("%s地址必须以 http:// 或 https:// 开头，且不超过 %d 个字符：%s", label, maxURLBytes, item)
		}
		out = append(out, item)
	}
	if len(out) > limit {
		return nil, invalid("%s最多 %d 个", label, limit)
	}
	return out, nil
}

func checkOptionalURL(label, value string) error {
	if value == "" || isHTTPURL(value) {
		return nil
	}
	return invalid("%s必须以 http:// 或 https:// 开头，且不超过 %d 个字符", label, maxURLBytes)
}

func checkRunes(label, value string, limit int) error {
	if utf8.RuneCountInString(value) > limit {
		return invalid("%s不能超过 %d 个字", label, limit)
	}
	return nil
}

func isHTTPURL(raw string) bool {
	if len(raw) > maxURLBytes {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func toResponse(e *model.Essay) *model.EssayResponse {
	return &model.EssayResponse{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		Content:     e.Content,
		PublishedAt: e.PublishedAt,
		Images:      e.Images,
		Videos:      e.Videos,
		Address:     e.Address,
		Source:      e.Source,
		LinkURL:     e.LinkURL,
		LinkTitle:   e.LinkTitle,
		MusicID:     e.MusicID,
		MusicName:   e.MusicName,
		MusicArtist: e.MusicArtist,
		MusicCover:  e.MusicCover,
		IsPublished: e.IsPublished,
	}
}

// IsNotFound 供 handler 判断 404
func IsNotFound(err error) bool { return errors.Is(err, ErrEssayNotFound) }
```

**Step 4：运行，确认通过**

Run: `go test ./pkg/service/essay/ -v`
Expected: 全部 `PASS`（`TestCreateRejectsInvalidInput` 的 12 个子用例都通过）。

**Step 5：静态检查并提交**

Run: `go vet ./pkg/service/essay/`
Expected: 无输出。

```bash
git add pkg/service/essay/
git commit -m "feat(essay): 新增即刻短文 service 及校验规则"
```

---

### Task B5：HTTP Handler

**Files:**
- Test: `pkg/handler/essay/handler_test.go`
- Create: `pkg/handler/essay/handler.go`

**Step 1：写失败的测试**

handler + service + 真实 ent 仓储（内存 SQLite）一起测，覆盖状态码映射。鉴权中间件不在这里测（Task B6 用 curl 验证）。

```go
package essay

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/ent/enttest"
	ent_impl "github.com/anzhiyu-c/anheyu-app/internal/infra/persistence/ent"
	essay_service "github.com/anzhiyu-c/anheyu-app/pkg/service/essay"
	"github.com/gin-gonic/gin"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

type apiResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client := enttest.Open(t, "sqlite3", "file:essay_handler_"+name+"?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	h := NewHandler(essay_service.NewService(ent_impl.NewEssayRepo(client)))
	r := gin.New()
	r.GET("/api/public/essays", h.ListPublic)
	r.GET("/api/essays", h.ListAdmin)
	r.GET("/api/essays/:id", h.Get)
	r.POST("/api/essays", h.Create)
	r.PUT("/api/essays/:id", h.Update)
	r.DELETE("/api/essays/:id", h.Delete)
	return r
}

func do(t *testing.T, r *gin.Engine, method, path, body string) (int, apiResp) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp apiResp
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json %q: %v", w.Body.String(), err)
	}
	return w.Code, resp
}

func TestCreateValidationReturns400(t *testing.T) {
	r := newTestRouter(t)
	code, resp := do(t, r, http.MethodPost, "/api/essays", `{"content":"  "}`)
	if code != http.StatusBadRequest || !strings.Contains(resp.Message, "至少填写一项") {
		t.Fatalf("code=%d msg=%q", code, resp.Message)
	}
}

func TestCreateThenGet(t *testing.T) {
	r := newTestRouter(t)
	code, resp := do(t, r, http.MethodPost, "/api/essays", `{"content":"海边","images":["https://a/b.jpg"]}`)
	if code != http.StatusOK {
		t.Fatalf("create code=%d msg=%q", code, resp.Message)
	}
	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(resp.Data, &created); err != nil || created.ID == 0 {
		t.Fatalf("bad create data %s: %v", resp.Data, err)
	}

	code, resp = do(t, r, http.MethodGet, "/api/essays/1", "")
	if code != http.StatusOK || !strings.Contains(string(resp.Data), "海边") {
		t.Fatalf("get code=%d data=%s", code, resp.Data)
	}
}

func TestPublicListHidesDrafts(t *testing.T) {
	r := newTestRouter(t)
	do(t, r, http.MethodPost, "/api/essays", `{"content":"公开"}`)
	do(t, r, http.MethodPost, "/api/essays", `{"content":"草稿","is_published":false}`)

	code, resp := do(t, r, http.MethodGet, "/api/public/essays?page=1&pageSize=10", "")
	if code != http.StatusOK {
		t.Fatalf("code=%d", code)
	}
	var list struct {
		Total int64 `json:"total"`
	}
	_ = json.Unmarshal(resp.Data, &list)
	if list.Total != 1 || strings.Contains(string(resp.Data), "草稿") {
		t.Fatalf("public list leaked drafts: %s", resp.Data)
	}
}

func TestStatusCodeMapping(t *testing.T) {
	r := newTestRouter(t)
	if code, _ := do(t, r, http.MethodPut, "/api/essays/999", `{"content":"x"}`); code != http.StatusNotFound {
		t.Fatalf("update missing: code=%d", code)
	}
	if code, _ := do(t, r, http.MethodDelete, "/api/essays/abc", ""); code != http.StatusBadRequest {
		t.Fatalf("bad id: code=%d", code)
	}
	if code, _ := do(t, r, http.MethodPost, "/api/essays", `not-json`); code != http.StatusBadRequest {
		t.Fatalf("bad json: code=%d", code)
	}

	do(t, r, http.MethodPost, "/api/essays", `{"content":"x"}`)
	if code, _ := do(t, r, http.MethodDelete, "/api/essays/1", ""); code != http.StatusOK {
		t.Fatalf("delete: code=%d", code)
	}
	if code, _ := do(t, r, http.MethodGet, "/api/essays/1", ""); code != http.StatusNotFound {
		t.Fatalf("get deleted: code=%d", code)
	}
}
```

**Step 2：运行，确认失败**

Run: `go test ./pkg/handler/essay/ -v`
Expected: 编译失败，`undefined: NewHandler`。

**Step 3：实现**

`pkg/handler/essay/handler.go`。和上游不同的一点：500 错误**不把内部错误原文返回给前端**，只记日志。

```go
/*
 * @Description: 即刻短文 HTTP 处理器（[custom] 二开新增）
 */
package essay

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/response"
	essay_service "github.com/anzhiyu-c/anheyu-app/pkg/service/essay"
	"github.com/gin-gonic/gin"
)

// Handler 封装即刻短文的 HTTP 处理器
type Handler struct {
	svc *essay_service.Service
}

// NewHandler 是 Handler 的构造函数
func NewHandler(svc *essay_service.Service) *Handler {
	return &Handler{svc: svc}
}

// ListPublic 前台短文列表
// @Summary  获取公开的即刻短文列表
// @Tags     即刻短文
// @Produce  json
// @Param    page query int false "页码" default(1)
// @Param    pageSize query int false "每页数量" default(20)
// @Success  200 {object} response.Response{data=model.EssayListResponse}
// @Router   /public/essays [get]
func (h *Handler) ListPublic(c *gin.Context) {
	page, pageSize := pagingFromQuery(c)
	result, err := h.svc.ListPublic(c.Request.Context(), page, pageSize)
	if err != nil {
		h.fail(c, err, "获取短文列表")
		return
	}
	response.Success(c, result, "获取成功")
}

// ListAdmin 后台短文列表（含未公开）
// @Summary  获取全部即刻短文（后台）
// @Tags     即刻短文
// @Security BearerAuth
// @Produce  json
// @Success  200 {object} response.Response{data=model.EssayListResponse}
// @Router   /essays [get]
func (h *Handler) ListAdmin(c *gin.Context) {
	page, pageSize := pagingFromQuery(c)
	result, err := h.svc.ListAdmin(c.Request.Context(), page, pageSize)
	if err != nil {
		h.fail(c, err, "获取短文列表")
		return
	}
	response.Success(c, result, "获取成功")
}

// Get 获取单条短文
// @Summary  获取单条即刻短文（后台）
// @Tags     即刻短文
// @Security BearerAuth
// @Produce  json
// @Param    id path int true "短文ID"
// @Success  200 {object} response.Response{data=model.EssayResponse}
// @Router   /essays/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	result, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		h.fail(c, err, "获取短文")
		return
	}
	response.Success(c, result, "获取成功")
}

// Create 创建短文
// @Summary  创建即刻短文
// @Tags     即刻短文
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    essay body model.EssayRequest true "短文内容"
// @Success  200 {object} response.Response{data=model.EssayResponse}
// @Router   /essays [post]
func (h *Handler) Create(c *gin.Context) {
	var req model.EssayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式不正确")
		return
	}
	result, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		h.fail(c, err, "创建短文")
		return
	}
	response.Success(c, result, "创建成功")
}

// Update 更新短文
// @Summary  更新即刻短文
// @Tags     即刻短文
// @Security BearerAuth
// @Accept   json
// @Produce  json
// @Param    id path int true "短文ID"
// @Param    essay body model.EssayRequest true "短文内容"
// @Success  200 {object} response.Response{data=model.EssayResponse}
// @Router   /essays/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req model.EssayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "请求参数格式不正确")
		return
	}
	result, err := h.svc.Update(c.Request.Context(), id, &req)
	if err != nil {
		h.fail(c, err, "更新短文")
		return
	}
	response.Success(c, result, "更新成功")
}

// Delete 删除短文
// @Summary  删除即刻短文
// @Tags     即刻短文
// @Security BearerAuth
// @Produce  json
// @Param    id path int true "短文ID"
// @Success  200 {object} response.Response
// @Router   /essays/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		h.fail(c, err, "删除短文")
		return
	}
	response.Success(c, nil, "删除成功")
}

func (h *Handler) fail(c *gin.Context, err error, action string) {
	var vErr *essay_service.ValidationError
	switch {
	case errors.As(err, &vErr):
		response.Fail(c, http.StatusBadRequest, vErr.Error())
	case essay_service.IsNotFound(err):
		response.Fail(c, http.StatusNotFound, essay_service.ErrEssayNotFound.Error())
	default:
		log.Printf("[Essay] %s失败: %v", action, err)
		response.Fail(c, http.StatusInternalServerError, action+"失败，请稍后重试")
	}
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Fail(c, http.StatusBadRequest, "短文 ID 不合法")
		return 0, false
	}
	return uint(id), true
}

func pagingFromQuery(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	return page, pageSize
}
```

**Step 4：运行，确认通过**

Run: `go test ./pkg/handler/essay/ -v`
Expected: 4 个测试全部 `PASS`。

**Step 5：提交**

```bash
git add pkg/handler/essay/
git commit -m "feat(essay): 新增即刻短文 HTTP 处理器"
```

---

### Task B6：注册路由（挂载到上游）

**Files:**
- Create: `internal/infra/router/essay_router.go`
- Modify: `cmd/server/app.go`（import 区 + `appRouter.Setup(engine)` 之后，约第 712 行）

**Step 1：新增路由注册函数**

`internal/infra/router/essay_router.go`：

```go
/*
 * @Description: 即刻短文路由（[custom] 二开新增）
 * 独立函数注册，不改 Router 结构体和 NewRouter 签名，避免与上游合并冲突。
 */
package router

import (
	"github.com/anzhiyu-c/anheyu-app/internal/app/middleware"
	essay_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/essay"
	"github.com/gin-gonic/gin"
)

// RegisterEssayRoutes 注册即刻短文的公开接口和后台管理接口
func RegisterEssayRoutes(engine *gin.Engine, h *essay_handler.Handler, mw *middleware.Middleware) {
	api := engine.Group("/api")
	api.Use(NoCacheMiddleware())

	api.GET("/public/essays", h.ListPublic)

	admin := api.Group("/essays").Use(mw.JWTAuth(), mw.AdminAuth())
	{
		admin.GET("", h.ListAdmin)
		admin.GET("/:id", h.Get)
		admin.POST("", h.Create)
		admin.PUT("/:id", h.Update)
		admin.DELETE("/:id", h.Delete)
	}
}
```

**Step 2：在 `cmd/server/app.go` 挂载**

import 区加两行（和其他 handler/service 的 import 放在一起）：

```go
	essay_handler "github.com/anzhiyu-c/anheyu-app/pkg/handler/essay" // [custom]
	essay_service "github.com/anzhiyu-c/anheyu-app/pkg/service/essay" // [custom]
```

在 `appRouter.Setup(engine)` 这一行**之后**插入：

```go
	// [custom] 即刻短文：独立注册，减少与上游的合并冲突
	essayHandler := essay_handler.NewHandler(essay_service.NewService(ent_impl.NewEssayRepo(entClient)))
	router.RegisterEssayRoutes(engine, essayHandler, mw)
```

**Step 3：编译 + 全量测试**

Run: `go build ./... && go test ./...`
Expected: 编译通过；测试全部 `ok`（基线 34 个包 + 新增的 3 个包），没有 `FAIL`。

**Step 4：端到端验证（真实 PostgreSQL）**

重启后端：停掉正在运行的 `scripts/dev-local.sh`，重新运行 `bash scripts/dev-local.sh`，等日志出现 `应用程序启动成功，正在监听端口: 8091`（如果启动时 panic 报路由冲突，停下来报告）。

依次运行并核对：

```bash
curl -s http://localhost:8091/api/public/essays
```
Expected: `{"code":200,"message":"获取成功","data":{"list":[],"total":0,"page":1,"pageSize":20}}`

```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8091/api/essays
```
Expected: `401`（没带 token，鉴权中间件生效）

```bash
curl -s http://localhost:3000/api/public/essays
```
Expected: 与第一条相同（经过 Next.js 代理）

```bash
docker compose -f docker-compose.local.yml exec -T postgres psql -U anheyu -d anheyu -c '\d essays'
```
Expected: 列出 `essays` 表，含 `content`、`published_at`、`images`、`is_published` 等列，以及 `is_published, published_at` 联合索引。

**Step 5：提交**

```bash
git add internal/infra/router/essay_router.go cmd/server/app.go
git commit -m "feat(essay): 注册即刻短文路由"
```

---

## 阶段 1 验收（停下来向用户汇报）

汇报内容：
1. `go test ./...` 的汇总（通过的包数、0 失败）
2. Task B6 Step 4 四条命令的实际输出
3. 本阶段的提交列表：`git log --oneline main..custom`
4. 对上游文件的改动只有 `cmd/server/app.go`（`git diff main..custom --stat -- cmd/`）和 ent 生成代码
