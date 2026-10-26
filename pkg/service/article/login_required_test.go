package article

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/repository"
	"github.com/anzhiyu-c/anheyu-app/pkg/service/utility"
	"time"
)

const restrictedTestHTML = `<p>public-before</p><div class="login-required-content-editor-preview" data-title="登录后可查看"><div class="login-required-content-header">需登录</div><div class="login-required-content-body"><div class="login-required-content-preview"><p>private-canary</p><div><pre>nested-secret</pre></div></div></div></div><p>public-after</p>`

type loginArticleRepo struct {
	repository.ArticleRepository
	article *model.Article
}

func (r loginArticleRepo) GetRandom(context.Context) (*model.Article, error) {
	return r.article, nil
}

func (r loginArticleRepo) GetByID(context.Context, string) (*model.Article, error) {
	return r.article, nil
}

func (r loginArticleRepo) GetBySlugOrID(context.Context, string) (*model.Article, error) {
	return r.article, nil
}

func (r loginArticleRepo) GetPrevArticle(context.Context, uint, time.Time) (*model.Article, error) {
	return nil, nil
}
func (r loginArticleRepo) GetNextArticle(context.Context, uint, time.Time) (*model.Article, error) {
	return nil, nil
}
func (r loginArticleRepo) FindRelatedArticles(context.Context, *model.Article, int) ([]*model.Article, error) {
	return nil, nil
}
func (r loginArticleRepo) ListPublic(context.Context, *model.ListPublicArticlesOptions) ([]*model.Article, int, error) {
	return []*model.Article{r.article}, 1, nil
}
func (r loginArticleRepo) ListHome(context.Context) ([]*model.Article, error) {
	return []*model.Article{r.article}, nil
}

type loginCache struct{ utility.CacheService }

func (loginCache) Get(context.Context, string) (string, error)      { return "", nil }
func (loginCache) Increment(context.Context, string) (int64, error) { return 0, nil }

type loginComments struct{ repository.CommentRepository }

func (loginComments) CountByTargetPaths(context.Context, []string) (map[string]int, error) {
	return nil, nil
}

func TestPublicArticleEndpointsUseSameRestrictedProjection(t *testing.T) {
	a := &model.Article{ID: "test", ContentHTML: restrictedTestHTML, ContentMd: "private-canary", Summaries: []string{"private-canary"}}
	svc := &serviceImpl{repo: loginArticleRepo{article: a}, cacheSvc: loginCache{}, commentRepo: loginComments{}}
	ctx := context.Background()
	detail, err := svc.GetPublicBySlugOrID(ctx, "test")
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListPublic(ctx, &model.ListPublicArticlesOptions{WithContent: true})
	if err != nil {
		t.Fatal(err)
	}
	home, err := svc.ListHome(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for name, response := range map[string]any{"detail/by-url/SSR": detail, "list with content": list, "home": home} {
		body, _ := json.Marshal(response)
		if strings.Contains(string(body), "private-canary") || strings.Contains(string(body), "nested-secret") {
			t.Errorf("%s leaked protected content", name)
		}
	}
	content, err := svc.GetLoginRequiredContent(ctx, "test")
	if err != nil || content != restrictedTestHTML {
		t.Fatal("authenticated read lost original HTML")
	}
}

func TestRandomPublicArticleHidesLoginRequiredContent(t *testing.T) {
	a := &model.Article{
		ID: "test", Status: "PUBLISHED", ContentHTML: restrictedTestHTML,
		ContentMd: "public-before\n:::login-required\nprivate-canary\n:::\npublic-after",
		Summaries: []string{"legacy summary containing private-canary"},
	}
	svc := &serviceImpl{repo: loginArticleRepo{article: a}}
	resp, err := svc.GetRandom(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(resp)
	for _, secret := range []string{"private-canary", "nested-secret"} {
		if strings.Contains(string(body), secret) {
			t.Errorf("anonymous response exposes %s", secret)
		}
	}
	for _, visible := range []string{"public-before", "public-after", "登录后可查看"} {
		if !strings.Contains(resp.ContentHTML, visible) {
			t.Errorf("public HTML lost %s", visible)
		}
	}
	if !strings.Contains(resp.ContentHTML, `data-login-required-locked="true"`) {
		t.Error("public HTML needs a locked placeholder")
	}
	if a.ContentHTML != restrictedTestHTML || !strings.Contains(a.ContentMd, "private-canary") {
		t.Error("public projection mutated stored article")
	}
	admin, err := svc.Get(context.Background(), a.ID)
	if err != nil || !strings.Contains(admin.ContentHTML, "private-canary") || !strings.Contains(admin.ContentMd, "private-canary") {
		t.Error("admin editing must retain the original content")
	}
}

func TestOrdinaryPublicArticleKeepsMarkdownAndSummary(t *testing.T) {
	a := &model.Article{ID: "test", ContentHTML: "<p>public</p>", ContentMd: "public", Summaries: []string{"summary"}}
	svc := &serviceImpl{repo: loginArticleRepo{article: a}}
	resp, err := svc.GetRandom(context.Background())
	if err != nil || resp.ContentHTML != a.ContentHTML || resp.ContentMd != a.ContentMd || len(resp.Summaries) != 1 {
		t.Fatalf("ordinary article changed: %#v, %v", resp, err)
	}
}
