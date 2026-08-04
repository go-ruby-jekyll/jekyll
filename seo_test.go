// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSeoGolden builds themeless .html pages carrying {% seo %} and asserts each
// rendered page is byte-identical to Ruby jekyll-seo-tag 2.9.0 for the home,
// about, generic-page and dated (BlogPosting) cases.
func TestSeoGolden(t *testing.T) {
	dst := t.TempDir()
	cfg, err := LoadConfig("testdata/seo/site", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg["source"] = "testdata/seo/site"
	cfg["destination"] = dst
	if err := NewSite(cfg).Build(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"index.html":         "index.html",
		"about/index.html":   "about.html",
		"contact/index.html": "contact.html",
		"blog/welcome.html":  "dated.html",
	}
	for out, golden := range cases {
		got, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(out)))
		if err != nil {
			t.Fatalf("%s: %v", out, err)
		}
		want, err := os.ReadFile(filepath.Join("testdata/seo/expected", golden))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s differs\n--- got ---\n%s\n--- want ---\n%s", out, got, want)
		}
	}
}

// seoBlock renders {% seo %} for a single page against the given config.
func seoBlock(t *testing.T, cfg Config, page map[string]any) string {
	t.Helper()
	cfg["source"] = t.TempDir()
	cfg["destination"] = t.TempDir()
	s := NewSite(cfg)
	s.sitePayload = map[string]any{}
	return (&renderer{site: s}).expandSeo("{% seo %}", map[string]any{"page": page})
}

func TestSeoTitleAndCanonicalDisabled(t *testing.T) {
	s := &renderer{site: NewSite(Config{"url": "https://x.com", "title": "T", "source": ".", "destination": "."})}
	out := s.expandSeo("{% seo title=false canonical=false %}", map[string]any{"page": map[string]any{"url": "/p/"}})
	if strings.Contains(out, "<title>") {
		t.Error("title=false should suppress <title>")
	}
	if strings.Contains(out, "rel=\"canonical\"") {
		t.Error("canonical=false should suppress canonical link")
	}
	if !strings.Contains(out, `<meta property="og:url" content="https://x.com/p/" />`) {
		t.Errorf("og:url should still be present: %s", out)
	}
}

func TestSeoAuthorImageTwitterFacebook(t *testing.T) {
	cfg := Config{
		"title": "Site", "url": "https://x.com",
		"author":   map[string]any{"name": "Jane", "twitter": "@janed"},
		"twitter":  map[string]any{"username": "@sitehandle", "card": "summary_large_image"},
		"facebook": map[string]any{"admins": "123", "publisher": "pub", "app_id": "app"},
	}
	page := map[string]any{
		"url":   "/post/",
		"title": "Hello",
		"date":  mustTime(t, "2023-01-02T03:04:05Z"),
		"image": map[string]any{"path": "/img/a.png", "height": 200, "width": 400, "alt": "Alt"},
	}
	out := seoBlock(t, cfg, page)
	for _, want := range []string{
		`<meta name="author" content="Jane" />`,
		`<meta property="og:image" content="https://x.com/img/a.png" />`,
		`<meta property="og:image:height" content="200" />`,
		`<meta property="og:image:width" content="400" />`,
		`<meta property="og:image:alt" content="Alt" />`,
		`<meta property="og:type" content="article" />`,
		`<meta name="twitter:card" content="summary_large_image" />`,
		`<meta name="twitter:image" content="https://x.com/img/a.png" />`,
		`<meta name="twitter:image:alt" content="Alt" />`,
		`<meta name="twitter:site" content="@sitehandle" />`,
		`<meta name="twitter:creator" content="@janed" />`,
		`<meta property="fb:admins" content="123" />`,
		`<meta property="article:publisher" content="pub" />`,
		`<meta property="fb:app_id" content="app" />`,
		`"author":{"@type":"Person","name":"Jane"}`,
		`"image":{"@type":"imageObject","url":"https://x.com/img/a.png","height":"200","width":"400"}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestSeoWebmasterAndLogoPublisher(t *testing.T) {
	cfg := Config{
		"title": "S", "url": "https://x.com", "logo": "/logo.png",
		"author":                  "Org Owner",
		"webmaster_verifications": map[string]any{"google": "g", "bing": "b", "alexa": "a", "yandex": "y", "baidu": "d", "facebook": "f"},
	}
	out := seoBlock(t, cfg, map[string]any{"url": "/", "title": "Home"})
	for _, want := range []string{
		`<meta name="google-site-verification" content="g" />`,
		`<meta name="msvalidate.01" content="b" />`,
		`<meta name="alexaVerifyID" content="a" />`,
		`<meta name="yandex-verification" content="y" />`,
		`<meta name="baidu-site-verification" content="d" />`,
		`<meta name="facebook-domain-verification" content="f" />`,
		`"publisher":{"@type":"Organization","logo":{"@type":"ImageObject","url":"https://x.com/logo.png"},"name":"Org Owner"}`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q\n%s", want, out)
		}
	}
}

func TestSeoGoogleVerificationFallbackAndSameAs(t *testing.T) {
	cfg := Config{
		"title": "S", "url": "https://x.com",
		"google_site_verification": "single",
		"social":                   map[string]any{"name": "Brand", "links": []any{"https://a", "https://b"}},
	}
	out := seoBlock(t, cfg, map[string]any{"url": "/"})
	if !strings.Contains(out, `<meta name="google-site-verification" content="single" />`) {
		t.Errorf("google_site_verification fallback missing: %s", out)
	}
	if !strings.Contains(out, `"sameAs":["https://a","https://b"]`) {
		t.Errorf("sameAs missing: %s", out)
	}
	// homepage social name overrides site title for the SEO name.
	if !strings.Contains(out, `"name":"Brand"`) {
		t.Errorf("social name not used: %s", out)
	}
}

func TestSeoImageRelativeAndAbsolutePaths(t *testing.T) {
	cfg := Config{"title": "S", "url": "https://x.com"}
	// A relative image path is resolved against the page directory.
	rel := seoBlock(t, cfg, map[string]any{"url": "/dir/page.html", "image": "pic.png"})
	if !strings.Contains(rel, `content="https://x.com/dir/pic.png"`) {
		t.Errorf("relative image not resolved: %s", rel)
	}
	// An already-absolute image URL is left untouched.
	abs := seoBlock(t, cfg, map[string]any{"url": "/p/", "image": "https://cdn/z.png"})
	if !strings.Contains(abs, `content="https://cdn/z.png"`) {
		t.Errorf("absolute image altered: %s", abs)
	}
}

func TestSeoDescriptionSnippetAndEscape(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("word ", 120))
	cfg := Config{"title": "S", "description": long}
	out := seoBlock(t, cfg, map[string]any{"url": "/x/"})
	if !strings.Contains(out, "…") {
		t.Errorf("long description should be snipped with an ellipsis: %s", out)
	}
	if escapeOnce(`a & b &amp; <c> "d"`) != `a &amp; b &amp; &lt;c&gt; &quot;d&quot;` {
		t.Fatalf("escapeOnce: %q", escapeOnce(`a & b &amp; <c> "d"`))
	}
}

func TestSeoDescriptionFromExcerptAndTitleCategory(t *testing.T) {
	cfg := Config{"title": "S"}
	page := map[string]any{
		"url":            "/n/",
		"title":          "Main",
		"title_category": "Cat",
		"excerpt":        "<p>From the excerpt.</p>",
	}
	out := seoBlock(t, cfg, page)
	if !strings.Contains(out, `content="From the excerpt." />`) {
		t.Errorf("description should come from excerpt: %s", out)
	}
	if !strings.Contains(out, `<meta property="og:title" content="Main | Cat" />`) {
		t.Errorf("page_title should combine title + title_category: %s", out)
	}
}

func TestLiquidTruthy(t *testing.T) {
	if liquidTruthy(nil) || liquidTruthy(false) {
		t.Fatal("nil/false are falsy")
	}
	if !liquidTruthy("") || !liquidTruthy(0) {
		t.Fatal("empty string and 0 are truthy in Liquid")
	}
}

func mustTime(t *testing.T, s string) any {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return tm
}

func newSeo(site Config, page map[string]any) seoTag {
	return seoTag{site: site, page: page, data: map[string]any{}, showTitle: true, showCanonical: true}
}

func TestSeoFieldBranches(t *testing.T) {
	tm := mustTime(t, "2023-05-06T07:08:09Z")

	// title/siteTitle empties, and name fallback.
	if got := newSeo(Config{}, map[string]any{}).title(); got != "" {
		t.Fatalf("empty title: %q", got)
	}
	if got := newSeo(Config{"name": "NM"}, map[string]any{}).siteTitle(); got != "NM" {
		t.Fatalf("name fallback: %q", got)
	}
	// title elsif uses tagline when present.
	if got := newSeo(Config{"title": "T", "tagline": "TG", "description": "D"}, map[string]any{"url": "/"}).title(); got != "T | TG" {
		t.Fatalf("tagline title: %q", got)
	}
	// pageTitle from category only, and the plain page-title return.
	if got := newSeo(Config{"title": "S"}, map[string]any{"title_category": "Cat"}).pageTitle(); got != "Cat" {
		t.Fatalf("category-only pageTitle: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{"title": "P"}).title(); got != "P" {
		t.Fatalf("page-only title: %q", got)
	}
	// canonicalURL from explicit key and from an index.html URL.
	if got := newSeo(Config{}, map[string]any{"canonical_url": "http://c"}).canonicalURL(); got != "http://c" {
		t.Fatalf("explicit canonical: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{"url": "/a/index.html"}).canonicalURL(); got != "/a/" {
		t.Fatalf("index.html canonical: %q", got)
	}
	// absoluteURL passes an absolute path through.
	if got := newSeo(Config{}, nil).absoluteURL("https://z"); got != "https://z" {
		t.Fatalf("absolute passthrough: %q", got)
	}
	// dateModified from seo.date_modified and from last_modified_at only.
	if got := newSeo(Config{}, map[string]any{"seo": map[string]any{"date_modified": tm}}).dateModified(); got != "2023-05-06T07:08:09Z" {
		t.Fatalf("seo.date_modified: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{"last_modified_at": tm}).dateModified(); got != "2023-05-06T07:08:09Z" {
		t.Fatalf("last_modified_at: %q", got)
	}
	// ldType from seo.type and the plain WebPage default.
	if got := newSeo(Config{}, map[string]any{"seo": map[string]any{"type": "Custom"}}).ldType(); got != "Custom" {
		t.Fatalf("seo.type: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{"url": "/x/"}).ldType(); got != "WebPage" {
		t.Fatalf("WebPage default: %q", got)
	}
	// links from seo.links.
	if got := newSeo(Config{}, map[string]any{"seo": map[string]any{"links": []any{"a"}}}).links(); len(got) != 1 {
		t.Fatalf("seo.links: %v", got)
	}
	// logo already absolute.
	if got := newSeo(Config{"logo": "https://l/x.png"}, nil).logo(); got != "https://l/x.png" {
		t.Fatalf("absolute logo: %q", got)
	}
	// twitterCard: page override, site override, default.
	if got := newSeo(Config{}, map[string]any{"twitter": map[string]any{"card": "pc"}}).twitterCard(); got != "pc" {
		t.Fatalf("page twitter card: %q", got)
	}
	if got := newSeo(Config{"twitter": map[string]any{"card": "sc"}}, map[string]any{}).twitterCard(); got != "sc" {
		t.Fatalf("site twitter card: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{}).twitterCard(); got != "summary_large_image" {
		t.Fatalf("default twitter card: %q", got)
	}
	// authorHash: authors[0], and authorTwitter name-fallback.
	if got := newSeo(Config{}, map[string]any{"authors": []any{"A0"}}).authorName(); got != "A0" {
		t.Fatalf("authors[0]: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{"author": "@N"}).authorTwitter(); got != "N" {
		t.Fatalf("author twitter name-fallback: %q", got)
	}
	if got := newSeo(Config{}, map[string]any{"author": map[string]any{"name": 123}}).authorTwitter(); got != "" {
		t.Fatalf("non-string twitter: %q", got)
	}
	// jsonAuthor: invalid type dropped, valid type + url kept.
	if got := newSeo(Config{"author": map[string]any{"name": "X", "type": "Robot"}}, map[string]any{}).jsonAuthor(); got != "" {
		t.Fatalf("invalid author type: %q", got)
	}
	if got := newSeo(Config{"author": map[string]any{"name": "X", "type": "Organization", "url": "u"}}, map[string]any{}).jsonAuthor(); got != `{"@type":"Organization","name":"X","url":"u"}` {
		t.Fatalf("author with url: %q", got)
	}
	// image path from the facebook fallback key.
	if img := newSeo(Config{"url": "https://x.com"}, map[string]any{"url": "/p/", "image": map[string]any{"facebook": "/f.png"}}).image(); img == nil || img.path != "https://x.com/f.png" {
		t.Fatalf("facebook image fallback: %+v", img)
	}
	// name from page.seo.name.
	if got := newSeo(Config{}, map[string]any{"url": "/x/", "seo": map[string]any{"name": "SN"}}).name(); got != "SN" {
		t.Fatalf("seo.name: %q", got)
	}
	// author string resolved against site.data.authors.
	da := seoTag{
		site: Config{}, showTitle: true, showCanonical: true,
		page: map[string]any{"author": "bob"},
		data: map[string]any{"authors": map[string]any{"bob": map[string]any{"name": "Bob", "twitter": "bt"}}},
	}
	if got := da.authorName(); got != "Bob" {
		t.Fatalf("data.authors merge: %q", got)
	}
	// image hash with no usable path resolves to nil.
	if img := newSeo(Config{}, map[string]any{"url": "/p/", "image": map[string]any{"alt": "x"}}).image(); img != nil {
		t.Fatalf("empty image should be nil: %+v", img)
	}
	// escapeOnce apostrophe, and firstStr all-empty.
	if escapeOnce("'") != "&#39;" {
		t.Fatalf("apostrophe: %q", escapeOnce("'"))
	}
	if firstStr(nil, nil) != "" {
		t.Fatal("firstStr all-empty should be empty")
	}
}
