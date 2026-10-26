package articlecontent

import (
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

func TestPublicCopyRemovesWholeRestrictedSubtrees(t *testing.T) {
	for _, source := range []string{
		`<p>visible</p><div class='extra login-required-content-editor-preview'><div><p>private-canary</p></div><!-- private-canary --><img src="private-canary"></div><p>after</p>`,
		`<p>visible</p><section class="login-required-content"><div class="login-required-content-editor-preview">private-canary</div></section><p>after</p>`,
		`<p>visible</p><div class="login-required-content-editor-preview"><p>private-canary`,
		`<p>visible</p><table><div class="login-required-content-editor-preview"><tr><td>private-canary</td></tr></div></table><p>after</p>`,
		`<p>visible</p><div class="login-required-content-editor-preview"/><p>private-canary</p>`,
		`<p>visible</p><DIV CLASS="login&#45;required-content-editor-preview"><p>private-canary</p></DIV><p>after</p>`,
		`<p>visible</p><svg><title><div class="login-required-content-editor-preview">private-canary</div></title></svg>`,
		`<p>visible</p><svg><style><div class="login-required-content-editor-preview">private-canary</div></style></svg>`,
	} {
		t.Run(source, func(t *testing.T) {
			public := PublicCopy(&model.Article{ContentHTML: source, ContentMd: "private-canary", Summaries: []string{"private-canary"}})
			if strings.Contains(public.ContentHTML, "private-canary") || public.ContentMd != "" || len(public.Summaries) != 0 {
				t.Fatal("restricted text or attributes escaped public filtering")
			}
			if !strings.Contains(public.ContentHTML, `data-login-required-locked="true"`) {
				t.Fatal("missing login placeholder")
			}
		})
	}
}

func TestPublicCopyFailsClosedForMarkdownOnlyRestriction(t *testing.T) {
	public := PublicCopy(&model.Article{ContentMd: ":::login-required title=test\nprivate-canary\n:::", ContentHTML: "<p>private-canary</p>"})
	if strings.Contains(public.ContentHTML, "private-canary") || public.ContentMd != "" {
		t.Fatal("missing HTML markers exposed Markdown-only restricted content")
	}
}

func TestPublicCopyLeavesOrdinaryArticleUntouched(t *testing.T) {
	article := &model.Article{ContentHTML: "<p>ordinary &amp; unchanged</p>", ContentMd: "ordinary", Summaries: []string{"summary"}}
	public := PublicCopy(article)
	if public.ContentHTML != article.ContentHTML || public.ContentMd != article.ContentMd || public.Summaries[0] != "summary" {
		t.Fatal("ordinary article was changed")
	}
}
