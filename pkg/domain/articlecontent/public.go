// Package articlecontent builds article content that is safe for anonymous readers.
package articlecontent

import (
	"io"
	"regexp"
	"strings"

	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const lockedHTML = `<div class="login-required-content-editor-preview" data-login-required-locked="true"><div class="login-required-content-header"><span class="login-required-title">登录后可查看</span></div><div class="login-required-content-body"><p>此内容需要登录后才能查看</p><a href="/login" data-login-action="check-email">登录查看</a></div></div>`

var loginMarkdown = regexp.MustCompile(`(?m)^\s*:{3,}\s*login-required(?:\s|$)`)

// PublicCopy never mutates the original, which is also used by the editor and history.
// Legacy summaries may have been generated from private text. Suppress them along
// with the raw Markdown when an article contains a login-required block.
func PublicCopy(article *model.Article) *model.Article {
	if article == nil {
		return nil
	}
	public := *article
	content, restricted := publicHTML(article.ContentHTML)
	if restricted || loginMarkdown.MatchString(article.ContentMd) || strings.Contains(article.ContentMd, "login-required-content-editor-preview") {
		public.ContentMd = ""
		public.Summaries = []string{}
		if !restricted {
			// A Markdown-only restriction with missing rendered markers must fail closed.
			content = lockedHTML
		}
	}
	public.ContentHTML = content
	return &public
}

func publicHTML(source string) (string, bool) {
	// Filter original token boundaries before the HTML tree builder can move
	// malformed table children outside a protected div (HTML foster parenting).
	tokens := html.NewTokenizer(strings.NewReader(source))
	var out strings.Builder
	restricted := false
	checkDOM := false
	protectedTag, depth := "", 0
	for {
		kind := tokens.Next()
		if kind == html.ErrorToken {
			if tokens.Err() != io.EOF {
				return lockedHTML, true
			}
			filtered := out.String()
			if checkDOM && hasUnfilteredRestriction(filtered) {
				return lockedHTML, true
			}
			if !restricted {
				return source, false
			}
			return filtered, true
		}
		raw := string(tokens.Raw())
		if kind == html.StartTagToken || kind == html.SelfClosingTagToken {
			token := tokens.Token()
			switch token.Data {
			case "script", "style", "title", "textarea", "xmp", "iframe", "noembed", "noframes", "noscript", "plaintext":
				checkDOM = true
			}
			if depth > 0 {
				if token.Data == protectedTag && !isVoidElement(token.Data) {
					depth++
				}
				continue
			}
			if isLoginRequired(token) {
				restricted = true
				out.WriteString(lockedHTML)
				if !isVoidElement(token.Data) {
					// HTML ignores a self-closing slash on non-void elements.
					protectedTag, depth = token.Data, 1
				}
				continue
			}
		} else if kind == html.EndTagToken && depth > 0 {
			if tokens.Token().Data == protectedTag {
				depth--
			}
			continue
		}
		if depth == 0 {
			out.WriteString(raw)
		}
	}
}

// Raw-text rules differ inside SVG/MathML. If a browser tree still contains a
// protected node after token filtering, hide the whole article rather than
// trusting a tree that may already have moved private descendants.
func hasUnfilteredRestriction(source string) bool {
	withoutPlaceholders := strings.ReplaceAll(source, lockedHTML, "")
	nodes, err := html.ParseFragment(strings.NewReader(withoutPlaceholders), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return true
	}
	var contains func(*html.Node) bool
	contains = func(node *html.Node) bool {
		if node.Type == html.ElementNode && isLoginRequired(html.Token{Attr: node.Attr}) {
			return true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if contains(child) {
				return true
			}
		}
		return false
	}
	for _, node := range nodes {
		if contains(node) {
			return true
		}
	}
	return false
}

func isLoginRequired(token html.Token) bool {
	for _, attr := range token.Attr {
		if attr.Key == "class" {
			for _, class := range strings.Fields(attr.Val) {
				if class == "login-required-content-editor-preview" || class == "login-required-content" {
					return true
				}
			}
		}
	}
	return false
}

func isVoidElement(tag string) bool {
	switch tag {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr", "keygen":
		return true
	default:
		return false
	}
}
