// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/go-ruby-kramdown/kramdown"
)

// jekyll-seo-tag (https://github.com/jekyll/jekyll-seo-tag) provides the
// {% seo %} tag, emitting the SEO/OpenGraph/Twitter/JSON-LD <head> block. This
// file reproduces the plugin's template + Drop logic (v2.9.0) so the output
// byte-matches the gem for a standard configuration. Fields that depend on
// runtime the CGO-free engine does not model (Liquid pagination) are omitted,
// which the README names.
const seoTagVersion = "2.9.0"

var homepageOrAboutRe = regexp.MustCompile(`^/(about/)?(index\.html?)?$`)

// seoTag holds the resolved inputs for one {% seo %} expansion.
type seoTag struct {
	site          Config
	page          map[string]any
	data          map[string]any
	showTitle     bool // title? (false via `{% seo title=false %}`)
	showCanonical bool // canonical? (false via `canonical=false`)
}

var seoArgRe = regexp.MustCompile(`(?i)(title|canonical)\s*=\s*false`)

// expandSeo replaces {% seo [args] %} with the rendered SEO block, using the
// page currently in scope.
func (r *renderer) expandSeo(src string, assigns map[string]any) string {
	return seoTagRe.ReplaceAllStringFunc(src, func(m string) string {
		args := seoTagRe.FindStringSubmatch(m)[1]
		page, _ := assigns["page"].(map[string]any)
		st := seoTag{
			site:          r.site.Config,
			page:          page,
			data:          r.site.data,
			showTitle:     true,
			showCanonical: true,
		}
		for _, a := range seoArgRe.FindAllStringSubmatch(args, -1) {
			if strings.EqualFold(a[1], "title") {
				st.showTitle = false
			} else {
				st.showCanonical = false
			}
		}
		return st.render()
	})
}

var seoTagRe = regexp.MustCompile(`\{%\s*seo\b([^%]*)%\}`)

// render builds the full SEO <head> block.
func (t seoTag) render() string {
	var b strings.Builder
	line := func(s string) { b.WriteString(s + "\n") }

	b.WriteString("<!-- Begin Jekyll SEO tag v" + seoTagVersion + " -->\n")
	if title := t.title(); t.showTitle && title != "" {
		line("<title>" + title + "</title>")
	}
	line(`<meta name="generator" content="Jekyll v` + jekyllVersion + `" />`)
	pageTitle := t.pageTitle()
	if pageTitle != "" {
		line(`<meta property="og:title" content="` + pageTitle + `" />`)
	}
	if an := t.authorName(); an != "" {
		line(`<meta name="author" content="` + an + `" />`)
	}
	line(`<meta property="og:locale" content="` + t.pageLocale() + `" />`)
	if desc := t.description(); desc != "" {
		line(`<meta name="description" content="` + desc + `" />`)
		line(`<meta name="twitter:description" property="og:description" content="` + desc + `" />`)
	}
	if liquidTruthy(t.site["url"]) {
		canon := t.canonicalURL()
		if t.showCanonical {
			line(`<link rel="canonical" href="` + canon + `" />`)
		}
		line(`<meta property="og:url" content="` + canon + `" />`)
	}
	if st := t.siteTitle(); st != "" {
		line(`<meta property="og:site_name" content="` + st + `" />`)
	}
	img := t.image()
	if img != nil {
		line(`<meta property="og:image" content="` + img.path + `" />`)
		if img.height != "" {
			line(`<meta property="og:image:height" content="` + img.height + `" />`)
		}
		if img.width != "" {
			line(`<meta property="og:image:width" content="` + img.width + `" />`)
		}
		if img.alt != "" {
			line(`<meta property="og:image:alt" content="` + img.alt + `" />`)
		}
	}
	if dp := t.datePublished(); dp != "" {
		line(`<meta property="og:type" content="article" />`)
		line(`<meta property="article:published_time" content="` + dp + `" />`)
		if dm := t.dateModified(); dm != "" {
			line(`<meta property="article:modified_time" content="` + dm + `" />`)
		}
	} else {
		line(`<meta property="og:type" content="website" />`)
	}
	if img != nil {
		line(`<meta name="twitter:card" content="` + t.twitterCard() + `" />`)
		line(`<meta name="twitter:image" content="` + img.path + `" />`)
	} else {
		line(`<meta name="twitter:card" content="summary" />`)
	}
	if img != nil && img.alt != "" {
		line(`<meta name="twitter:image:alt" content="` + img.alt + `" />`)
	}
	if pageTitle != "" {
		line(`<meta name="twitter:title" content="` + pageTitle + `" />`)
	}
	t.renderTwitter(line)
	t.renderFacebook(line)
	t.renderWebmaster(line)
	line(`<script type="application/ld+json">`)
	b.WriteString(t.jsonLD() + "</script>\n")
	// The template file ends with a trailing newline, which the plugin's minify
	// pass preserves; keep it so the tag output byte-matches.
	b.WriteString("<!-- End Jekyll SEO tag -->\n")
	return b.String()
}

func (t seoTag) renderTwitter(line func(string)) {
	tw, ok := t.site["twitter"].(map[string]any)
	if !liquidTruthy(t.site["twitter"]) {
		return
	}
	user := ""
	if ok {
		user = removeAt(toS(tw["username"]))
	}
	line(`<meta name="twitter:site" content="@` + user + `" />`)
	if at := t.authorTwitter(); at != "" {
		line(`<meta name="twitter:creator" content="@` + at + `" />`)
	}
}

func (t seoTag) renderFacebook(line func(string)) {
	fb, ok := t.site["facebook"].(map[string]any)
	if !ok {
		return
	}
	if v := toS(fb["admins"]); v != "" {
		line(`<meta property="fb:admins" content="` + v + `" />`)
	}
	if v := toS(fb["publisher"]); v != "" {
		line(`<meta property="article:publisher" content="` + v + `" />`)
	}
	if v := toS(fb["app_id"]); v != "" {
		line(`<meta property="fb:app_id" content="` + v + `" />`)
	}
}

func (t seoTag) renderWebmaster(line func(string)) {
	wv, ok := t.site["webmaster_verifications"].(map[string]any)
	if ok {
		emit := func(key, name string) {
			if v := toS(wv[key]); v != "" {
				line(`<meta name="` + name + `" content="` + v + `" />`)
			}
		}
		emit("google", "google-site-verification")
		emit("bing", "msvalidate.01")
		emit("alexa", "alexaVerifyID")
		emit("yandex", "yandex-verification")
		emit("baidu", "baidu-site-verification")
		emit("facebook", "facebook-domain-verification")
		return
	}
	if v := toS(t.site["google_site_verification"]); v != "" {
		line(`<meta name="google-site-verification" content="` + v + `" />`)
	}
}

// --- field computations (mirroring Drop) ---

func (t seoTag) siteTitle() string {
	if v := formatString(t.site["title"], t.site); v != "" {
		return v
	}
	return formatString(t.site["name"], t.site)
}

func (t seoTag) siteTagline() string     { return formatString(t.site["tagline"], t.site) }
func (t seoTag) siteDescription() string { return formatString(t.site["description"], t.site) }

func (t seoTag) pageTitle() string {
	title := formatString(t.page["title"], t.site)
	cat := formatString(t.page["title_category"], t.site)
	switch {
	case title != "" && cat != "" && title != cat:
		return title + " | " + cat
	case title != "":
		return title
	case cat != "":
		return cat
	default:
		return t.siteTitle()
	}
}

func (t seoTag) siteTaglineOrDescription() string {
	if v := t.siteTagline(); v != "" {
		return v
	}
	return t.siteDescription()
}

func (t seoTag) title() string {
	st := t.siteTitle()
	pt := t.pageTitle()
	switch {
	case st != "" && pt != st:
		return pt + " | " + st
	case t.siteDescription() != "" && st != "":
		return st + " | " + t.siteTaglineOrDescription()
	case pt != "":
		return pt
	default:
		return st
	}
}

func (t seoTag) homepageOrAbout() bool {
	return homepageOrAboutRe.MatchString(toS(t.page["url"]))
}

func (t seoTag) seoName() string {
	return formatString(subHash(t.page, "seo")["name"], t.site)
}

func (t seoTag) name() string {
	if n := t.seoName(); n != "" {
		return n
	}
	if !t.homepageOrAbout() {
		return ""
	}
	if n := formatString(subHash(t.site, "social")["name"], t.site); n != "" {
		return n
	}
	return t.siteTitle()
}

func (t seoTag) description() string {
	v := formatString(firstNonEmpty(t.page["description"], t.page["excerpt"]), t.site)
	if v == "" {
		v = t.siteDescription()
	}
	return snippetWords(v, 100)
}

func (t seoTag) pageLang() string {
	return firstStr(t.page["lang"], t.site["lang"], "en_US")
}

func (t seoTag) pageLocale() string {
	return strings.ReplaceAll(firstStr(t.page["locale"], t.site["locale"], t.pageLang()), "-", "_")
}

func (t seoTag) canonicalURL() string {
	if cu := toS(t.page["canonical_url"]); cu != "" {
		return cu
	}
	u := t.absoluteURL(toS(t.page["url"]))
	if strings.HasSuffix(u, "/index.html") {
		return strings.TrimSuffix(u, "index.html")
	}
	return u
}

// absoluteURL applies the site url/baseurl the way the plugin's filters do.
func (t seoTag) absoluteURL(path string) string {
	rel := relativeURL(t.site.str("baseurl"), path)
	if isAbsoluteURL(rel) {
		return rel
	}
	return strings.TrimSuffix(t.site.str("url"), "/") + rel
}

func (t seoTag) datePublished() string {
	if d, ok := t.page["date"]; ok && d != nil {
		return xmlSchemaDate(d)
	}
	return ""
}

func (t seoTag) dateModified() string {
	if v := subHash(t.page, "seo")["date_modified"]; v != nil {
		return xmlSchemaDate(v)
	}
	if v, ok := t.page["last_modified_at"]; ok && v != nil {
		return xmlSchemaDate(v)
	}
	if v, ok := t.page["date"]; ok && v != nil {
		return xmlSchemaDate(v)
	}
	return ""
}

func (t seoTag) ldType() string {
	if v := toS(subHash(t.page, "seo")["type"]); v != "" {
		return v
	}
	if t.homepageOrAbout() {
		return "WebSite"
	}
	if v, ok := t.page["date"]; ok && v != nil {
		return "BlogPosting"
	}
	return "WebPage"
}

func (t seoTag) links() []any {
	if v, ok := subHash(t.page, "seo")["links"].([]any); ok {
		return v
	}
	if t.homepageOrAbout() {
		if v, ok := subHash(t.site, "social")["links"].([]any); ok {
			return v
		}
	}
	return nil
}

func (t seoTag) logo() string {
	logo := toS(t.site["logo"])
	if logo == "" {
		return ""
	}
	if isAbsoluteURL(logo) {
		return uriEscapePath(logo)
	}
	return uriEscapePath(t.absoluteURL(logo))
}

func (t seoTag) twitterCard() string {
	if v := toS(subHash(t.page, "twitter")["card"]); v != "" {
		return v
	}
	if v := toS(subHash(t.site, "twitter")["card"]); v != "" {
		return v
	}
	return "summary_large_image"
}

// --- author drop ---

func (t seoTag) authorHash() map[string]any {
	sources := []any{t.page["author"]}
	if authors, ok := t.page["authors"].([]any); ok && len(authors) > 0 {
		sources = append(sources, authors[0])
	}
	sources = append(sources, t.site["author"])
	var resolved any
	for _, s := range sources {
		if toS(s) != "" {
			resolved = s
			break
		}
	}
	switch a := resolved.(type) {
	case map[string]any:
		return a
	case string:
		h := map[string]any{"name": a}
		if da, ok := t.data["authors"].(map[string]any); ok {
			if extra, ok := da[a].(map[string]any); ok {
				for k, v := range extra {
					h[k] = v
				}
			}
		}
		return h
	default:
		return map[string]any{}
	}
}

func (t seoTag) authorName() string { return toS(t.authorHash()["name"]) }

func (t seoTag) authorTwitter() string {
	h := t.authorHash()
	tw := h["twitter"]
	if tw == nil {
		tw = h["name"]
	}
	if s, ok := tw.(string); ok {
		return removeAt(s)
	}
	return ""
}

// --- image drop ---

type seoImage struct {
	path, height, width, alt string
}

func (t seoTag) image() *seoImage {
	meta := t.page["image"]
	var hash map[string]any
	switch m := meta.(type) {
	case map[string]any:
		hash = m
	case string:
		hash = map[string]any{"path": m}
	default:
		return nil
	}
	raw := firstStr(hash["path"], hash["facebook"], hash["twitter"])
	if raw == "" {
		return nil
	}
	var abs string
	if isAbsoluteURL(raw) {
		abs = raw
	} else if strings.HasPrefix(raw, "/") {
		abs = t.absoluteURL(raw)
	} else {
		dir := toS(t.page["url"])
		if !strings.HasSuffix(dir, "/") {
			dir = dir[:strings.LastIndex(dir, "/")+1]
		}
		abs = t.absoluteURL(dir + raw)
	}
	return &seoImage{
		path:   uriEscapePath(abs),
		height: toS(hash["height"]),
		width:  toS(hash["width"]),
		alt:    toS(hash["alt"]),
	}
}

// --- JSON-LD ---

// jsonLD renders the JSON-LD object with top-level keys sorted (as the plugin's
// JSONLDDrop#to_json does) and nested objects in the plugin's field order.
func (t seoTag) jsonLD() string {
	pairs := map[string]string{}
	add := func(k, v string) {
		if v != "" {
			pairs[k] = v
		}
	}
	add("@context", jsonString("https://schema.org"))
	add("@type", jsonString(t.ldType()))
	add("description", optJSONString(t.description()))
	add("url", optJSONString(t.canonicalURL()))
	add("headline", optJSONString(t.pageTitle()))
	add("name", optJSONString(t.name()))
	add("dateModified", optJSONString(t.dateModified()))
	add("datePublished", optJSONString(t.datePublished()))
	if s := t.jsonSameAs(); s != "" {
		add("sameAs", s)
	}
	if a := t.jsonAuthor(); a != "" {
		add("author", a)
	}
	if i := t.jsonImage(); i != "" {
		add("image", i)
	}
	if p := t.jsonPublisher(); p != "" {
		add("publisher", p)
	}
	if me := t.jsonMainEntity(); me != "" {
		add("mainEntityOfPage", me)
	}

	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(jsonString(k) + ":" + pairs[k])
	}
	b.WriteString("}")
	return b.String()
}

func (t seoTag) jsonSameAs() string {
	links := t.links()
	if len(links) == 0 {
		return ""
	}
	parts := make([]string, len(links))
	for i, l := range links {
		parts[i] = jsonString(toS(l))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func (t seoTag) jsonAuthor() string {
	h := t.authorHash()
	name := toS(h["name"])
	if name == "" {
		return ""
	}
	atype := toS(h["type"])
	if atype != "" && atype != "Organization" && atype != "Person" {
		return ""
	}
	if atype == "" {
		atype = "Person"
	}
	out := `{"@type":` + jsonString(atype) + `,"name":` + jsonString(name)
	if url := toS(h["url"]); url != "" {
		out += `,"url":` + jsonString(url)
	}
	return out + "}"
}

func (t seoTag) jsonImage() string {
	img := t.image()
	if img == nil {
		return ""
	}
	meta, _ := t.page["image"].(map[string]any)
	// One key (path only) → a bare URL string; otherwise an ImageObject.
	if len(meta) <= 1 {
		return jsonString(img.path)
	}
	out := `{"@type":"imageObject","url":` + jsonString(img.path)
	if img.height != "" {
		out += `,"height":` + jsonString(img.height)
	}
	if img.width != "" {
		out += `,"width":` + jsonString(img.width)
	}
	return out + "}"
}

func (t seoTag) jsonPublisher() string {
	logo := t.logo()
	if logo == "" {
		return ""
	}
	out := `{"@type":"Organization","logo":{"@type":"ImageObject","url":` + jsonString(logo) + `}`
	if name := t.authorName(); name != "" {
		out += `,"name":` + jsonString(name)
	}
	return out + "}"
}

func (t seoTag) jsonMainEntity() string {
	tp := t.ldType()
	if tp != "BlogPosting" && tp != "CreativeWork" {
		return ""
	}
	return `{"@type":"WebPage","@id":` + jsonString(t.canonicalURL()) + "}"
}

// --- shared helpers ---

// liquidTruthy mirrors Liquid truthiness: only nil and false are falsy (an empty
// string, 0 and empty collections are truthy).
func liquidTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	default:
		return true
	}
}

// markdownifyStr renders Markdown to HTML with a trailing newline trimmed, like
// Jekyll's markdownify filter.
func markdownifyStr(s string, cfg Config) string {
	return strings.TrimRight(kramdown.ToHTML(s, kramdownOptions(cfg)), "\n")
}

// formatString applies jekyll-seo-tag's format chain
// (markdownify → strip_html → normalize_whitespace → escape_once); an empty
// result is returned as "".
func formatString(v any, cfg Config) string {
	s := toS(v)
	if s == "" {
		return ""
	}
	return escapeOnce(normalizeWS(stripHTML(markdownifyStr(s, cfg))))
}

var escapeOnceRe = regexp.MustCompile(`["><']|&(?:[a-zA-Z]+|#\d+);|&`)

// escapeOnce escapes &, <, >, " and ' but leaves existing entities intact, like
// Liquid's escape_once filter.
func escapeOnce(s string) string {
	return escapeOnceRe.ReplaceAllStringFunc(s, func(m string) string {
		switch m {
		case `"`:
			return "&quot;"
		case ">":
			return "&gt;"
		case "<":
			return "&lt;"
		case "'":
			return "&#39;"
		case "&":
			return "&amp;"
		default:
			return m // an existing entity is preserved
		}
	})
}

// snippetWords truncates a string to at most max whitespace-separated words,
// appending an ellipsis when truncation occurred (Drop#snippet).
func snippetWords(s string, max int) string {
	if s == "" {
		return ""
	}
	fields := strings.Fields(s)
	if len(fields) <= max {
		return s
	}
	return strings.Join(fields[:max], " ") + "…"
}

// firstNonEmpty returns the first value whose string form is non-empty.
func firstNonEmpty(vs ...any) any {
	for _, v := range vs {
		if toS(v) != "" {
			return v
		}
	}
	return nil
}

// firstStr returns the first non-empty string among the values.
func firstStr(vs ...any) string {
	for _, v := range vs {
		if s := toS(v); s != "" {
			return s
		}
	}
	return ""
}

// subHash returns m[key] as a map, or an empty map.
func subHash(m map[string]any, key string) map[string]any {
	if h, ok := m[key].(map[string]any); ok {
		return h
	}
	return map[string]any{}
}

// removeAt strips a leading '@'.
func removeAt(s string) string { return strings.TrimPrefix(s, "@") }

// uriEscapePath escapes a URL path the way Jekyll's uri_escape filter does.
func uriEscapePath(s string) string {
	return (&url.URL{Path: s}).EscapedPath()
}

// jsonString encodes a Go string as JSON without HTML escaping, matching Ruby's
// to_json.
func jsonString(s string) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return strings.TrimRight(buf.String(), "\n")
}

// optJSONString returns "" for an empty string (so the JSON-LD key is omitted),
// else its JSON encoding.
func optJSONString(s string) string {
	if s == "" {
		return ""
	}
	return jsonString(s)
}
