// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// feedUpdatedRe extracts the feed-level <updated> (site.time) from a feed so the
// golden comparison can pin our site.time to the value the oracle was built with.
var feedUpdatedRe = regexp.MustCompile(`<updated>([^<]+)</updated>`)

// TestFeedGoldenMinimal builds a themeless site with a real `jekyll new` post and
// asserts the generated /feed.xml is byte-identical to Ruby jekyll-feed 0.17.0.
func TestFeedGoldenMinimal(t *testing.T) {
	golden, err := os.ReadFile("testdata/feed/expected_feed.xml")
	if err != nil {
		t.Fatal(err)
	}
	siteTime, err := time.Parse(time.RFC3339, feedUpdatedRe.FindStringSubmatch(string(golden))[1])
	if err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	cfg, err := LoadConfig("testdata/feed/site", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg["source"] = "testdata/feed/site"
	cfg["destination"] = dst
	site := NewSite(cfg)
	site.Time = siteTime
	if err := site.Build(); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "feed.xml"))
	if err != nil {
		t.Fatalf("feed.xml not written: %v", err)
	}
	if string(got) != string(golden) {
		t.Fatalf("feed mismatch\n--- got ---\n%s\n--- want ---\n%s", got, golden)
	}
}

// buildFeedString is a helper returning the generated feed content for a built site.
func buildFeedString(t *testing.T, s *Site) string {
	t.Helper()
	for _, g := range s.generated {
		if g.url == "/feed.xml" {
			return g.content
		}
	}
	t.Fatal("no feed generated")
	return ""
}

func TestFeedNotGeneratedWithoutPlugin(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\nbody\n")
	s := buildInto(t, src, nil)
	if len(s.generated) != 0 {
		t.Fatalf("no feed expected without jekyll-feed, got %d", len(s.generated))
	}
}

func TestFeedFieldsAndOptions(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", strings.Join([]string{
		"title: Blog",
		"description: A blog",
		"url: https://example.com",
		"lang: en",
		"author:",
		"  name: Jane",
		"  email: jane@example.com",
		"  uri: https://jane.example",
		"plugins:",
		"  - jekyll-feed",
	}, "\n")+"\n")
	writeTest(t, src, "_data/authors.yml", "bob:\n  name: Bob Smith\n  email: bob@example.com\n  uri: https://bob.example\n")
	// Newest post: single category, tag, explicit author from data, image, lang.
	writeTest(t, src, "_posts/2023-03-03-c.md", "---\ntitle: \"Post C\"\ncategory: news\ntags: [go]\nauthor: bob\nlang: fr\nimage: /img/c.png\nlast_modified_at: 2023-03-04 00:00:00 +0000\n---\nHello **C** world.\n")
	writeTest(t, src, "_posts/2023-02-02-b.md", "---\ntitle: \"Post B\"\ndescription: Custom summary\n---\nBody B.\n")
	s := buildInto(t, src, nil)
	feed := buildFeedString(t, s)

	// Two entries: C (with lang) then B.
	if strings.Count(feed, "<entry>") != 1 || strings.Count(feed, "<entry ") != 1 {
		t.Fatalf("expected two entries (one with lang), feed=%s", feed)
	}
	must := []string{
		`xml:lang="en"`, // feed lang
		`hreflang="en"`, // alternate hreflang
		`<author><name>Jane</name><email>jane@example.com</email><uri>https://jane.example</uri></author>`,
		`<entry xml:lang="fr">`, // per-post lang
		`<title type="html">Post C</title>`,
		`https://example.com/news/2023/03/03/c.html`, // absolute_url with url
		`<category term="news" />`,                   // single category
		`<category term="go" />`,                     // tag
		`<updated>2023-03-04T00:00:00Z</updated>`,    // last_modified_at
		`<author><name>Bob Smith</name><email>bob@example.com</email><uri>https://bob.example</uri></author>`, // data.authors
		`<media:thumbnail xmlns:media="http://search.yahoo.com/mrss/" url="https://example.com/img/c.png" />`,
		`<summary type="html"><![CDATA[Custom summary]]></summary>`, // description-derived summary (post B)
	}
	for _, m := range must {
		if !strings.Contains(feed, m) {
			t.Errorf("feed missing %q\n%s", m, feed)
		}
	}
}

func TestFeedPostsLimit(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nplugins:\n  - jekyll-feed\nfeed:\n  posts_limit: 1\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\nA.\n")
	writeTest(t, src, "_posts/2023-02-02-b.md", "---\ntitle: B\n---\nB.\n")
	feed := buildFeedString(t, buildInto(t, src, nil))
	if n := strings.Count(feed, "<entry>"); n != 1 {
		t.Fatalf("posts_limit:1 want 1 entry, got %d", n)
	}
	if !strings.Contains(feed, "<title type=\"html\">B</title>") {
		t.Fatalf("newest post should be kept: %s", feed)
	}
}

func TestFeedGeneratedWriteError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nplugins:\n  - jekyll-feed\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\nA.\n")
	dst := t.TempDir()
	// Pre-create the feed target as a directory so writeFile fails.
	if err := os.MkdirAll(filepath.Join(dst, "feed.xml"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := mustCfg(t, src)
	cfg["destination"] = dst
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("writing feed.xml over a directory should fail the build")
	}
}

func TestFeedHelpers(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nurl: https://ex.com\n")
	s := NewSite(mustCfg(t, src))
	// absolute_url passes an already-absolute URL through unchanged.
	if got := s.absoluteURL("https://cdn/x"); got != "https://cdn/x" {
		t.Fatalf("absoluteURL absolute passthrough: %q", got)
	}
	// xml-schema formatting of a non-date value returns it unchanged.
	if got := xmlSchemaDate("not-a-date"); got != "not-a-date" {
		t.Fatalf("xmlSchemaDate non-date: %q", got)
	}
}

func TestFeedMetaPathOverride(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: S\nplugins:\n  - jekyll-feed\nfeed:\n  path: atom.xml\n")
	writeTest(t, src, "index.html", "---\n---\n{% feed_meta %}")
	dst := t.TempDir()
	buildIntoDst(t, src, dst, nil)
	if got := readOut(t, dst, "index.html"); !strings.Contains(got, `href="/atom.xml"`) {
		t.Fatalf("feed_meta path override: %q", got)
	}
}

func TestFeedExcerptOnlyAndPostAuthorScalar(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nplugins:\n  - jekyll-feed\nfeed:\n  excerpt_only: true\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\nauthor: Solo\n---\nFirst para.\n\nSecond.\n")
	s := buildInto(t, src, nil)
	feed := buildFeedString(t, s)
	if strings.Contains(feed, "<content") {
		t.Error("excerpt_only should suppress <content>")
	}
	if !strings.Contains(feed, "<author><name>Solo</name></author>") {
		t.Errorf("scalar post author missing: %s", feed)
	}
	if !strings.Contains(feed, "<summary type=\"html\"><![CDATA[First para.]]></summary>") {
		t.Errorf("excerpt summary wrong: %s", feed)
	}
}

func TestFeedPerPostExcerptOnlyAndAuthorsList(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nname: Fallback\nplugins:\n  - jekyll-feed\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\nauthors:\n  - Ann\n  - Bea\nfeed:\n  excerpt_only: true\nimage:\n  path: https://cdn.example/x.png\n---\nBody.\n")
	s := buildInto(t, src, nil)
	feed := buildFeedString(t, s)
	if strings.Contains(feed, "<content") {
		t.Error("per-post excerpt_only should suppress content")
	}
	if !strings.Contains(feed, "<author><name>Ann</name></author>") {
		t.Errorf("authors[0] not used: %s", feed)
	}
	// Absolute image path is left untouched.
	if !strings.Contains(feed, `url="https://cdn.example/x.png"`) {
		t.Errorf("absolute image path altered: %s", feed)
	}
}

func TestFeedTitleFromNameAndSourceFileSkip(t *testing.T) {
	// title falls back to `name`; an existing feed.xml source file is not overwritten.
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "name: NameOnly\nplugins:\n  - jekyll-feed\nfeed:\n  path: atom.xml\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\nBody.\n")
	writeTest(t, src, "atom.xml", "PRE-EXISTING")
	s := buildInto(t, src, nil)
	if len(s.generated) != 0 {
		t.Fatalf("feed.path pointing at an existing file must be skipped, got %d", len(s.generated))
	}
}

func TestFeedMetaTag(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: My Site\nurl: https://ex.com\nplugins:\n  - jekyll-feed\n")
	writeTest(t, src, "index.html", "---\n---\n{% feed_meta %}")
	dst := t.TempDir()
	buildIntoDst(t, src, dst, nil)
	got := readOut(t, dst, "index.html")
	want := `<link type="application/atom+xml" rel="alternate" href="https://ex.com/feed.xml" title="My Site" />`
	if got != want {
		t.Fatalf("feed_meta = %q, want %q", got, want)
	}
}

func TestFeedMetaNoTitle(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "plugins:\n  - jekyll-feed\n")
	writeTest(t, src, "index.html", "---\n---\n{% feed_meta %}")
	dst := t.TempDir()
	buildIntoDst(t, src, dst, nil)
	if got := readOut(t, dst, "index.html"); got != `<link type="application/atom+xml" rel="alternate" href="/feed.xml" />` {
		t.Fatalf("feed_meta without title = %q", got)
	}
}

func TestThemeActivatesPluginDeps(t *testing.T) {
	withThemeGems(t)
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "theme: depstheme\ntitle: T\n")
	s := NewSite(mustCfg(t, src))
	if !s.pluginActive("jekyll-feed") || !s.pluginActive("jekyll-seo-tag") {
		t.Fatalf("theme gem runtime deps should activate plugins: %v", s.plugins)
	}
	if s.pluginActive("jekyll") {
		t.Fatal("non jekyll-* deps must be ignored")
	}
}

func TestThemeGemDepsMissing(t *testing.T) {
	if themeGemDeps("") != nil {
		t.Fatal("empty root -> nil")
	}
	if themeGemDeps(filepath.Join(t.TempDir(), "gems", "x-1.0.0")) != nil {
		t.Fatal("missing gemspec -> nil")
	}
}

func TestJekyllPayloadEnv(t *testing.T) {
	t.Setenv("JEKYLL_ENV", "production")
	if jekyllPayload()["environment"] != "production" {
		t.Fatal("JEKYLL_ENV not honoured")
	}
	t.Setenv("JEKYLL_ENV", "")
	if jekyllPayload()["environment"] != "development" {
		t.Fatal("default env should be development")
	}
}

func mustCfg(t *testing.T, src string) Config {
	t.Helper()
	cfg, err := LoadConfig(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	return cfg
}
