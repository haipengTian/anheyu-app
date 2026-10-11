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
