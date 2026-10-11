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
		"全部为空":           {},
		"只有空白":           {Content: "   ", Images: []string{"  "}},
		"文字超长":           {Content: strings.Repeat("字", 2001)},
		"图片不是 http":      {Images: []string{"ftp://a/b.jpg"}},
		"图片太多":           {Images: strings.Split(strings.TrimSuffix(strings.Repeat("https://a/b.jpg,", 10), ","), ",")},
		"视频太多":           {Videos: []string{"https://a/1.mp4", "https://a/2.mp4", "https://a/3.mp4", "https://a/4.mp4", "https://a/5.mp4"}},
		"地点超长":           {Content: "x", Address: strings.Repeat("地", 51)},
		"外链是 javascript": {Content: "x", LinkURL: "javascript:alert(1)"},
		"有外链标题没外链":       {Content: "x", LinkTitle: "标题"},
		"歌曲 ID 不是数字":     {Content: "x", MusicID: "abc"},
		"有歌名没歌曲 ID":      {Content: "x", MusicName: "晴天"},
		"封面不是 URL":       {Content: "x", MusicID: "123", MusicCover: "cover.jpg"},
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
