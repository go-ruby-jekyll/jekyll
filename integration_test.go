// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestIntegrationMinima is the end-to-end proof that a real `jekyll new`-style
// site on the minima theme BUILDS under go-ruby-jekyll with theme resolution,
// jekyll-feed and jekyll-seo-tag all wired together. It builds against a vendored
// minima gem fixture (testdata/gemroot/gems/minima-2.5.2) and compares every
// output to Ruby Jekyll 4.4.1 + minima 2.5.2 + jekyll-feed 0.17.0 +
// jekyll-seo-tag 2.9.0.
//
// about, post AND index.html are all asserted byte-identical to reference Jekyll:
// this is the full pure-Go Jekyll byte-parity capstone. index.html's former sole
// residual — an empty home page body, which go-ruby-kramdown once rendered as ""
// where Ruby kramdown yields "\n", dropping a newline inside home.html's
// {{ content }} slot — is closed now that empty Markdown renders as "\n" like the
// gem. go-ruby-liquid's {%- -%} / {{- -}} whitespace control is byte-exact against
// the liquid gem, including minima's skipped-conditional layout shape. feed.xml
// and assets/main.css are asserted byte-identical (feed modulo the site-time
// <updated>; CSS modulo the sourcemap trailer go-scss does not emit).
func TestIntegrationMinima(t *testing.T) {
	withThemeGems(t)
	dst := t.TempDir()
	cfg, err := LoadConfig("testdata/integration/site", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg["source"] = "testdata/integration/site"
	cfg["destination"] = dst
	site := NewSite(cfg)
	site.Time = time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	if err := site.Build(); err != nil {
		t.Fatalf("minima site failed to build: %v", err)
	}

	// The theme's gem dependencies must have activated both plugins.
	if !site.pluginActive("jekyll-feed") || !site.pluginActive("jekyll-seo-tag") {
		t.Fatalf("minima deps did not activate plugins: %v", site.plugins)
	}

	// Expected output tree (no .css.map: go-scss emits no source maps).
	assertTree(t, dst, []string{
		"about/index.html",
		"assets/main.css",
		"assets/minima-social-icons.svg",
		"feed.xml",
		"index.html",
		"intro/demo/2024/01/02/hello.html",
	})

	// HTML pages: byte-identical to reference Jekyll. index.html is now included in
	// the strict comparison — its former empty-body newline residual is closed, so
	// the full home+about+post triad is byte-exact (full pure-Go Jekyll byte parity).
	for out, golden := range map[string]string{
		"about/index.html":                 "about.html",
		"intro/demo/2024/01/02/hello.html": "post.html",
		"index.html":                       "index.html",
	} {
		got := readOut(t, dst, out)
		wantHTML := goldenText(t, golden)
		if got != wantHTML {
			t.Errorf("%s is not byte-identical to reference Jekyll\n got: %q\nwant: %q", out, got, wantHTML)
		}
	}

	// index.html must carry the byte-exact SEO block and feed autodiscovery link.
	index := readOut(t, dst, "index.html")
	for _, frag := range []string{
		"<!-- Begin Jekyll SEO tag v2.9.0 -->",
		`<meta property="og:site_name" content="Test Blog" />`,
		`{"@context":"https://schema.org","@type":"WebSite","description":"A test blog for integration.","headline":"Test Blog","name":"Test Blog","url":"/"}`,
		"<!-- End Jekyll SEO tag -->",
		`<link type="application/atom+xml" rel="alternate" href="/feed.xml" title="Test Blog" />`,
		`<a class="site-title" rel="author" href="/">Test Blog</a>`, // theme header include
		`<a class="page-link" href="/about/">About</a>`,             // nav from site.pages
	} {
		if !strings.Contains(index, frag) {
			t.Errorf("index.html missing byte-exact fragment %q", frag)
		}
	}

	// feed.xml: byte-identical modulo the site-time <updated>.
	if got, want := normalizeFeedTime(readOut(t, dst, "feed.xml")), normalizeFeedTime(goldenText(t, "feed.xml")); got != want {
		t.Errorf("feed.xml differs\n got: %s\nwant: %s", got, want)
	}

	// assets/main.css: byte-identical modulo the sourcemap trailer.
	gotCSS := strings.TrimRight(readOut(t, dst, "assets/main.css"), "\n")
	wantCSS := strings.TrimRight(stripSourcemap(goldenText(t, "main.css")), "\n")
	if gotCSS != wantCSS {
		t.Errorf("assets/main.css differs from minima's compiled stylesheet")
	}
}

func goldenText(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata/integration/expected", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func assertTree(t *testing.T, dst string, want []string) {
	t.Helper()
	var got []string
	filepath.WalkDir(dst, func(p string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(dst, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("output tree mismatch\n got: %v\nwant: %v", got, want)
	}
}

var feedUpdatedFirst = regexp.MustCompile(`<updated>[^<]*</updated>`)

// normalizeFeedTime replaces only the first (feed-level, site-time) <updated> so
// the rest of the feed — including each entry's deterministic dates — is compared
// byte-for-byte.
func normalizeFeedTime(s string) string {
	done := false
	return feedUpdatedFirst.ReplaceAllStringFunc(s, func(m string) string {
		if done {
			return m
		}
		done = true
		return "<updated>@</updated>"
	})
}

var sourcemapRe = regexp.MustCompile(`(?s)\s*/\*# sourceMappingURL=[^*]*\*/\s*$`)

func stripSourcemap(s string) string {
	return sourcemapRe.ReplaceAllString(s, "")
}
