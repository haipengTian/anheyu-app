package search

import (
	"context"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

type loginIndexRecorder struct {
	model.Searcher
	indexed *model.Article
}

func (r *loginIndexRecorder) IndexArticle(_ context.Context, article *model.Article) error {
	r.indexed = article
	return nil
}

func TestSearchIndexesOnlyPublicArticleContent(t *testing.T) {
	previous := AppSearcher
	t.Cleanup(func() { AppSearcher = previous })
	recorder := &loginIndexRecorder{}
	AppSearcher = recorder
	article := &model.Article{ID: "post", ContentHTML: `<p>public</p><div class="login-required-content-editor-preview"><p>private-canary</p></div>`}
	if err := NewSearchService().IndexArticle(context.Background(), article); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(recorder.indexed.ContentHTML, "private-canary") {
		t.Fatal("search index contains protected text")
	}
	if !strings.Contains(article.ContentHTML, "private-canary") {
		t.Fatal("indexing modified the original article")
	}
}
