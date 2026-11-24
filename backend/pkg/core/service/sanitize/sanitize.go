// Package sanitize 用 bluemonday 清洗用户输入的富文本，防止 XSS。
//
// admin 后台允许的标签：标题/段落/列表/链接/图片/强调，剥离 script/style/iframe/on* 事件等。
// C 端不再二次清洗（保存时已 sanitize；前端只 v-html 输出可信内容）。
package sanitize

import (
	"strings"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	richOnce sync.Once
	richPol  *bluemonday.Policy
)

func richPolicy() *bluemonday.Policy {
	richOnce.Do(func() {
		p := bluemonday.UGCPolicy()
		// 额外允许：标题级别、段落、br、图片
		p.AllowElements("h1", "h2", "h3", "h4", "h5", "h6", "p", "br", "hr", "blockquote", "pre", "code", "ul", "ol", "li", "strong", "em", "b", "i", "u", "img", "a", "span", "div")
		p.AllowAttrs("href").OnElements("a")
		p.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")
		p.AllowAttrs("class").Globally()
		// 限制 a 的协议白名单，避免 javascript:
		p.AllowURLSchemes("http", "https", "mailto")
		richPol = p
	})
	return richPol
}

// RichHTML 清洗富文本 HTML：剥离 script/style/iframe/on* 事件、危险协议链接
func RichHTML(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimSpace(richPolicy().Sanitize(s))
}

// PlainText 仅保留纯文本（用于 title / summary 等不允许任何 HTML 的字段）
func PlainText(s string) string {
	return strings.TrimSpace(bluemonday.StripTagsPolicy().Sanitize(s))
}
