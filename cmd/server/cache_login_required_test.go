package server

import (
	"net/http"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

func TestManualSearchRebuildFiltersLoginRequiredContent(t *testing.T) {
	indexer := &cacheTestIndexer{}
	useCacheTestIndexer(t, indexer)
	article := &model.Article{ID: "published", Status: "PUBLISHED", ReviewStatus: "NONE", ContentHTML: `<p>public</p><div class="login-required-content-editor-preview">private-canary</div>`}
	repo := &cacheArticleRepo{pages: [][]*model.Article{{article}}}
	response := cacheManageRequest(newCacheManagement(nil, nil, repo, nil), "index")
	if response.Code != http.StatusOK || len(indexer.indexed) != 1 {
		t.Fatal("manual index rebuild failed")
	}
	if strings.Contains(indexer.indexed[0].ContentHTML, "private-canary") {
		t.Fatal("manual rebuild reintroduced protected text into public search")
	}
	if !strings.Contains(article.ContentHTML, "private-canary") {
		t.Fatal("rebuild changed source content")
	}
}
