package ent

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/anzhiyu-c/anheyu-app/ent/article"
	"github.com/anzhiyu-c/anheyu-app/ent/enttest"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/idgen"
)

func TestLoginReadingUsesPublicLookupAndListKeepsFilteringInputs(t *testing.T) {
	if err := idgen.InitSqidsEncoderWithSeed("login_required_repository_test"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:login_required_repository?mode=memory&cache=shared&_fk=1")
	defer client.Close()
	repo := NewArticleRepo(client, "sqlite3")
	for _, tc := range []struct {
		name                        string
		status                      article.Status
		review                      article.ReviewStatus
		takenDown, deleted, visible bool
	}{
		{"published-login", article.StatusPUBLISHED, article.ReviewStatusNONE, false, false, true},
		{"draft-login", article.StatusDRAFT, article.ReviewStatusNONE, false, false, false},
		{"scheduled-login", article.StatusSCHEDULED, article.ReviewStatusNONE, false, false, false},
		{"archived-login", article.StatusARCHIVED, article.ReviewStatusNONE, false, false, false},
		{"pending-login", article.StatusPUBLISHED, article.ReviewStatusPENDING, false, false, false},
		{"takedown-login", article.StatusPUBLISHED, article.ReviewStatusAPPROVED, true, false, false},
		{"deleted-login", article.StatusPUBLISHED, article.ReviewStatusNONE, false, true, false},
	} {
		create := client.Article.Create().SetTitle(tc.name).SetAbbrlink(tc.name).
			SetStatus(tc.status).SetReviewStatus(tc.review).SetIsTakedown(tc.takenDown).
			SetShowOnHome(true).SetContentMd(":::login-required\nprivate-canary\n:::").
			SetContentHTML(`<div class="login-required-content-editor-preview">private-canary</div>`).
			SetSummaries([]string{"legacy private-canary summary"})
		if tc.deleted {
			create.SetDeletedAt(time.Now())
		}
		create.SaveX(ctx)
		_, err := repo.GetBySlugOrID(ctx, tc.name)
		if (err == nil) != tc.visible {
			t.Errorf("%s lookup error = %v, visible = %v", tc.name, err, tc.visible)
		}
	}
	list, _, err := repo.ListPublic(ctx, &model.ListPublicArticlesOptions{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || !strings.Contains(list[0].ContentHTML, "login-required-content") || list[0].ContentMd == "" {
		t.Fatal("list query must retain source markers so the service can suppress legacy summaries")
	}
}
