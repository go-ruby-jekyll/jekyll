// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"html"
	"regexp"
	"strings"

	"github.com/go-kramdown/kramdown"
	"github.com/go-rouge/rouge"
)

// kramdownOptions maps the site configuration onto go-ruby-kramdown options,
// matching Jekyll's kramdown defaults (auto_ids on, hard_wrap off, smart quotes
// on).
func kramdownOptions(cfg Config) *kramdown.Options {
	o := kramdown.DefaultOptions()
	o.HardWrap = false // Jekyll sets kramdown hard_wrap: false
	// Jekyll does not use kramdown's built-in Rouge highlighter; it wraps code
	// blocks itself (see rougeifyCodeBlocks), whose whitespace differs from the
	// standalone kramdown gem. Disable the native highlighter so kramdown emits
	// plain <pre><code class="language-…"> that rougeifyCodeBlocks then rewrites.
	o.SyntaxHighlighter = ""
	if kd, ok := cfg["kramdown"].(map[string]any); ok {
		if v, ok := kd["auto_ids"].(bool); ok {
			o.AutoIds = v
		}
		if v, ok := kd["hard_wrap"].(bool); ok {
			o.HardWrap = v
		}
	}
	return &o
}

// convertMarkdown renders Markdown to HTML through go-ruby-kramdown and then
// upgrades fenced code blocks and inline code spans to the Jekyll/rouge form.
func convertMarkdown(src string, cfg Config) string {
	out := kramdown.ToHTML(src, kramdownOptions(cfg))
	out = rougeifyCodeBlocks(out)
	out = addInlineCodeClass(out)
	return out
}

// codeBlockRe matches kramdown's fenced-code output, with or without a language
// class:
//
//	<pre><code class="language-XXX">...</code></pre>
//	<pre><code>...</code></pre>
var codeBlockRe = regexp.MustCompile(`(?s)<pre><code(?: class="language-([^"]+)")?>(.*?)</code></pre>`)

// rougeifyCodeBlocks replaces kramdown's plain highlighted code blocks with the
// exact wrapper Jekyll emits when syntax_highlighter: rouge is configured. A
// block with no language is treated as "plaintext", like Jekyll.
func rougeifyCodeBlocks(htmlStr string) string {
	return codeBlockRe.ReplaceAllStringFunc(htmlStr, func(m string) string {
		sub := codeBlockRe.FindStringSubmatch(m)
		lang := sub[1]
		if lang == "" {
			lang = "plaintext"
		}
		code := html.UnescapeString(sub[2])
		return highlightBlock(code, lang)
	})
}

// inlineSentinel temporarily hides block <pre><code ...> openings while the
// bare inline <code> spans are given Jekyll's plaintext class.
const inlineSentinel = "\x00PRECODE"

// addInlineCodeClass tags inline code spans with Jekyll's rouge plaintext class,
// leaving block code (already rewritten by rougeifyCodeBlocks) untouched.
func addInlineCodeClass(htmlStr string) string {
	htmlStr = strings.ReplaceAll(htmlStr, "<pre class=\"highlight\"><code>", inlineSentinel)
	htmlStr = strings.ReplaceAll(htmlStr, "<code>", `<code class="language-plaintext highlighter-rouge">`)
	htmlStr = strings.ReplaceAll(htmlStr, inlineSentinel, "<pre class=\"highlight\"><code>")
	return htmlStr
}

// highlightBlock renders code in the named language through go-ruby-rouge and
// wraps it in Jekyll's div structure.
func highlightBlock(code, lang string) string {
	spans, err := rouge.Highlight(code, lang, "html")
	if err != nil || spans == "" {
		spans = html.EscapeString(code)
	}
	return "<div class=\"language-" + lang + " highlighter-rouge\"><div class=\"highlight\"><pre class=\"highlight\"><code>" +
		spans + "</code></pre></div></div>"
}

// highlightFigure renders the {% highlight %} tag body, matching Jekyll's figure
// wrapper (distinct from the fenced-code-block wrapper above).
func highlightFigure(code, lang string) string {
	spans, err := rouge.Highlight(code, lang, "html")
	if err != nil || spans == "" {
		spans = html.EscapeString(code)
	}
	return "<figure class=\"highlight\"><pre><code class=\"language-" + lang +
		"\" data-lang=\"" + lang + "\">" + spans + "</code></pre></figure>"
}
