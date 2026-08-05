// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"regexp"
	"strings"
	"time"

	"github.com/go-ruby-kramdown/kramdown"
)

// runGenerators runs the enabled plugin generators after the site has rendered
// (so post content and excerpts are available to them).
func (s *Site) runGenerators() {
	if s.pluginActive("jekyll-feed") {
		s.generateFeed()
	}
}

// jekyll-feed (https://github.com/jekyll/jekyll-feed) generates an Atom feed at
// /feed.xml. This file reproduces the plugin's `feed.xml` template output field
// for field, byte-for-byte, for the posts collection. The multi-feed variants
// (per-collection, per-category and per-tag feed files) are a named gap in the
// README; the standard `jekyll new` site produces only /feed.xml.

// generateFeed appends the posts Atom feed to the site's generated outputs,
// unless a source file already occupies its path.
func (s *Site) generateFeed() {
	path := "/feed.xml"
	if p := s.feedConfigStr("path"); p != "" {
		path = "/" + strings.TrimPrefix(p, "/")
	}
	if s.hasSourceFile(strings.TrimPrefix(path, "/")) {
		return
	}
	s.generated = append(s.generated, generatedFile{url: path, content: s.buildFeed(path)})
}

// feedConfig returns the `feed:` config map (empty when unset).
func (s *Site) feedConfig() map[string]any {
	if m, ok := s.Config["feed"].(map[string]any); ok {
		return m
	}
	return nil
}

func (s *Site) feedConfigStr(key string) string {
	if v, ok := s.feedConfig()[key].(string); ok {
		return v
	}
	return ""
}

// buildFeed renders the Atom feed document for the posts collection at feedURL.
func (s *Site) buildFeed(feedURL string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)

	lang, _ := s.Config["lang"].(string)
	b.WriteString(`<feed xmlns="http://www.w3.org/2005/Atom" `)
	if lang != "" {
		b.WriteString(`xml:lang="` + lang + `"`)
	}
	b.WriteString(">")

	b.WriteString(`<generator uri="https://jekyllrb.com/" version="` + jekyllVersion + `">Jekyll</generator>`)
	b.WriteString(`<link href="` + s.absoluteURL(feedURL) + `" rel="self" type="application/atom+xml" />`)
	b.WriteString(`<link href="` + s.absoluteURL("/") + `" rel="alternate" type="text/html" `)
	if lang != "" {
		b.WriteString(`hreflang="` + lang + `" `)
	}
	b.WriteString("/>")
	b.WriteString("<updated>" + xmlSchemaDate(s.Time, siteLocation(s.Config)) + "</updated>")
	b.WriteString("<id>" + xmlEscape(s.absoluteURL(feedURL)) + "</id>")

	if title := s.feedTitle(); title != "" {
		b.WriteString(`<title type="html">` + xmlEscape(smartifyText(title, s.Config)) + "</title>")
	}
	if desc := s.Config.str("description"); desc != "" {
		b.WriteString("<subtitle>" + xmlEscape(desc) + "</subtitle>")
	}
	s.writeFeedAuthor(&b)

	for _, p := range s.feedPosts() {
		s.writeFeedEntry(&b, s.docMap(p))
	}
	b.WriteString("</feed>")
	return b.String()
}

// feedTitle mirrors `site.title | default: site.name`.
func (s *Site) feedTitle() string {
	if t := s.Config.str("title"); t != "" {
		return t
	}
	return s.Config.str("name")
}

// writeFeedAuthor emits the feed-level <author> block when site.author is set,
// supporting both the scalar and the {name,email,uri} hash forms.
func (s *Site) writeFeedAuthor(b *strings.Builder) {
	author := s.Config["author"]
	if author == nil {
		return
	}
	name, email, uri := authorParts(author)
	b.WriteString("<author><name>" + xmlEscape(name) + "</name>")
	if email != "" {
		b.WriteString("<email>" + xmlEscape(email) + "</email>")
	}
	if uri != "" {
		b.WriteString("<uri>" + xmlEscape(uri) + "</uri>")
	}
	b.WriteString("</author>")
}

// feedPosts returns the posts to include, newest first and capped at the
// configured post limit (default 10). s.posts is already newest-first.
func (s *Site) feedPosts() []*Document {
	limit := 10
	if v, ok := s.feedConfig()["posts_limit"].(int); ok && v >= 0 {
		limit = v
	}
	posts := s.posts
	if len(posts) > limit {
		posts = posts[:limit]
	}
	return posts
}

// writeFeedEntry emits one <entry> for a post, matching the plugin template.
func (s *Site) writeFeedEntry(b *strings.Builder, post map[string]any) {
	lang, _ := post["lang"].(string)
	if lang != "" {
		b.WriteString(`<entry xml:lang="` + lang + `">`)
	} else {
		b.WriteString("<entry>")
	}

	title := xmlEscape(normalizeWS(stripHTML(smartifyText(toS(post["title"]), s.Config))))
	url := toS(post["url"])
	b.WriteString(`<title type="html">` + title + "</title>")
	b.WriteString(`<link href="` + s.absoluteURL(url) + `" rel="alternate" type="text/html" title="` + title + `" />`)
	b.WriteString("<published>" + xmlSchemaDate(post["date"], siteLocation(s.Config)) + "</published>")
	updated := post["date"]
	if lm, ok := post["last_modified_at"]; ok && lm != nil {
		updated = lm
	}
	b.WriteString("<updated>" + xmlSchemaDate(updated, siteLocation(s.Config)) + "</updated>")
	b.WriteString("<id>" + xmlEscape(s.absoluteURL(toS(post["id"]))) + "</id>")

	if !s.feedExcerptOnly(post) {
		b.WriteString(`<content type="html" xml:base="` + xmlEscape(s.absoluteURL(url)) + `"><![CDATA[` +
			strings.TrimSpace(toS(post["content"])) + "]]></content>")
	}

	name, email, uri := s.postAuthorParts(post)
	b.WriteString("<author><name>" + xmlEscape(name) + "</name>")
	if email != "" {
		b.WriteString("<email>" + xmlEscape(email) + "</email>")
	}
	if uri != "" {
		b.WriteString("<uri>" + xmlEscape(uri) + "</uri>")
	}
	b.WriteString("</author>")

	s.writeFeedCategories(b, post)
	for _, tag := range stringList(post["tags"]) {
		b.WriteString(`<category term="` + xmlEscape(tag) + `" />`)
	}

	summary := feedSummary(post)
	if summary != "" {
		b.WriteString(`<summary type="html"><![CDATA[` + normalizeWS(stripHTML(summary)) + "]]></summary>")
	}
	s.writeFeedImage(b, post)
	b.WriteString("</entry>")
}

// feedExcerptOnly resolves post.feed.excerpt_only | default: site.feed.excerpt_only.
func (s *Site) feedExcerptOnly(post map[string]any) bool {
	if pf, ok := post["feed"].(map[string]any); ok {
		if v, ok := pf["excerpt_only"].(bool); ok {
			return v
		}
	}
	if v, ok := s.feedConfig()["excerpt_only"].(bool); ok {
		return v
	}
	return false
}

// writeFeedCategories emits <category> for post.category, else for each of
// post.categories.
func (s *Site) writeFeedCategories(b *strings.Builder, post map[string]any) {
	if c, ok := post["category"]; ok && toS(c) != "" {
		b.WriteString(`<category term="` + xmlEscape(toS(c)) + `" />`)
		return
	}
	for _, c := range stringList(post["categories"]) {
		b.WriteString(`<category term="` + xmlEscape(c) + `" />`)
	}
}

// writeFeedImage emits the media:thumbnail / media:content pair for post.image.
func (s *Site) writeFeedImage(b *strings.Builder, post map[string]any) {
	img := feedImagePath(post["image"])
	if img == "" {
		return
	}
	if !strings.Contains(img, "://") {
		img = s.absoluteURL(img)
	}
	esc := xmlEscape(img)
	b.WriteString(`<media:thumbnail xmlns:media="http://search.yahoo.com/mrss/" url="` + esc + `" />`)
	b.WriteString(`<media:content medium="image" url="` + esc + `" xmlns:media="http://search.yahoo.com/mrss/" />`)
}

// feedImagePath resolves post.image.path | default: post.image.
func feedImagePath(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		return toS(t["path"])
	}
	return ""
}

// feedSummary resolves post.description | default: post.excerpt.
func feedSummary(post map[string]any) string {
	if d := toS(post["description"]); d != "" {
		return d
	}
	return toS(post["excerpt"])
}

// postAuthorParts resolves the entry author following the plugin's chain:
// post.author | post.authors[0] | site.author, then site.data.authors[name].
func (s *Site) postAuthorParts(post map[string]any) (name, email, uri string) {
	var author any = post["author"]
	if author == nil || toS(author) == "" {
		if authors := post["authors"]; authors != nil {
			if list, ok := authors.([]any); ok && len(list) > 0 {
				author = list[0]
			}
		}
	}
	if author == nil || toS(author) == "" {
		author = s.Config["author"]
	}
	// A string author may reference an entry in site.data.authors.
	if key, ok := author.(string); ok {
		if da := s.dataAuthors(); da != nil {
			if resolved, ok := da[key]; ok {
				author = resolved
			}
		}
	}
	return authorParts(author)
}

// dataAuthors returns site.data.authors as a map, when present.
func (s *Site) dataAuthors() map[string]any {
	data, ok := s.data["authors"].(map[string]any)
	if !ok {
		return nil
	}
	return data
}

// authorParts extracts (name, email, uri) from a scalar or hash author value,
// mirroring `author.name | default: author`.
func authorParts(author any) (name, email, uri string) {
	switch a := author.(type) {
	case nil:
		return "", "", ""
	case map[string]any:
		name = toS(a["name"])
		email = toS(a["email"])
		uri = toS(a["uri"])
		return name, email, uri
	default:
		return toS(a), "", ""
	}
}

// absoluteURL applies Jekyll's absolute_url filter for the site's url/baseurl.
func (s *Site) absoluteURL(path string) string {
	rel := relativeURL(s.Config.str("baseurl"), path)
	if isAbsoluteURL(rel) {
		return rel
	}
	return strings.TrimSuffix(s.Config.str("url"), "/") + rel
}

// xmlSchemaDate formats a value as an XML-schema (RFC3339) timestamp in the site
// timezone loc, matching Jekyll's date_to_xmlschema under an ENV["TZ"] set from
// site.config["timezone"].
func xmlSchemaDate(v any, loc *time.Location) string {
	if t, ok := toTimeIn(v, loc); ok {
		return inLoc(t, loc).Format(time.RFC3339)
	}
	return toS(v)
}

// smartifyText applies kramdown's SmartyPants pass (curly quotes, dashes) the way
// Jekyll's `smartify` filter does.
func smartifyText(s string, cfg Config) string {
	return strings.TrimSpace(stripP(kramdown.ToHTML(s, kramdownOptions(cfg))))
}

var (
	stripHTMLScriptRe  = regexp.MustCompile(`(?is)<script.*?</script>`)
	stripHTMLStyleRe   = regexp.MustCompile(`(?is)<style.*?</style>`)
	stripHTMLCommentRe = regexp.MustCompile(`(?s)<!--.*?-->`)
	stripHTMLTagRe     = regexp.MustCompile(`(?s)<.*?>`)
)

// stripHTML removes HTML tags (and script/style/comment bodies) like Liquid's
// strip_html filter.
func stripHTML(s string) string {
	s = stripHTMLScriptRe.ReplaceAllString(s, "")
	s = stripHTMLStyleRe.ReplaceAllString(s, "")
	s = stripHTMLCommentRe.ReplaceAllString(s, "")
	return stripHTMLTagRe.ReplaceAllString(s, "")
}

// normalizeWS collapses runs of whitespace to single spaces, like Jekyll's
// normalize_whitespace filter.
func normalizeWS(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

var feedMetaRe = regexp.MustCompile(`(?s)(\s*)\{%(-?)\s*feed_meta\s*(-?)%\}(\s*)`)

// expandFeedMeta replaces {% feed_meta %} (jekyll-feed's tag) with the Atom
// autodiscovery <link>, matching JekyllFeed::MetaTag. Liquid whitespace-control
// markers ({%- ... -%}) are honoured.
func (r *renderer) expandFeedMeta(src string) string {
	return feedMetaRe.ReplaceAllStringFunc(src, func(m string) string {
		sub := feedMetaRe.FindStringSubmatch(m)
		return trimWrap(sub[1], sub[2], r.site.feedMetaLink(), sub[3], sub[4])
	})
}

// feedMetaLink builds the <link rel="alternate" type="application/atom+xml"> tag.
func (s *Site) feedMetaLink() string {
	path := "feed.xml"
	if p := s.feedConfigStr("path"); p != "" {
		path = p
	}
	href := s.absoluteURL(path)
	out := `<link type="application/atom+xml" rel="alternate" href="` + xmlEscape(href) + `"`
	if title := s.feedTitle(); title != "" {
		out += ` title="` + xmlEscape(title) + `"`
	}
	return out + " />"
}
