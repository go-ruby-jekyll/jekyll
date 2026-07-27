// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLinkAndPostURLTags(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nbaseurl: /blog\n")
	writeTest(t, src, "_posts/2023-05-05-hello.md", "---\ntitle: Hello\n---\nbody\n")
	writeTest(t, src, "about.md", "---\ntitle: About\n---\nAbout page\n")
	writeTest(t, src, "links.html",
		"---\ntitle: Links\n---\n"+
			"<a href=\"{% link about.md %}\">about</a>\n"+
			"<a href=\"{% post_url 2023-05-05-hello %}\">post</a>\n"+
			"<a href=\"{% link missing.md %}\">x</a>\n")
	s := buildIntoDst(t, src, dst, map[string]any{})
	_ = s
	out, _ := os.ReadFile(filepath.Join(dst, "links.html"))
	str := string(out)
	if !strings.Contains(str, `href="/blog/about.html"`) {
		t.Errorf("link tag not resolved: %s", str)
	}
	if !strings.Contains(str, `href="/blog/hello.html"`) && !strings.Contains(str, `href="/blog/2023/05/05/hello.html"`) {
		// permalink default is date style; url is /2023/05/05/hello.html
		if !strings.Contains(str, "/blog/2023/05/05/hello.html") {
			t.Errorf("post_url not resolved: %s", str)
		}
	}
}

func TestIncludeWithVariableParam(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_includes/box.html", "<b>{{ include.label }}</b>")
	writeTest(t, src, "p.html", "---\ntitle: P\nkind: warn\n---\n{% include box.html label=page.kind %}\n")
	buildIntoDst(t, src, dst, map[string]any{})
	out, _ := os.ReadFile(filepath.Join(dst, "p.html"))
	if !strings.Contains(string(out), "<b>warn</b>") {
		t.Errorf("include var param not resolved: %s", out)
	}
}

func TestIncludeNotFound(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "p.html", "---\ntitle: P\n---\n{% include nope.html %}\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	site := NewSite(cfg)
	if err := site.Build(); err == nil {
		t.Fatal("missing include should error")
	}
}

func TestRenderDepthGuard(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_includes/loop.html", "{% include loop.html %}")
	writeTest(t, src, "p.html", "---\ntitle: P\n---\n{% include loop.html %}\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("infinite include should hit depth guard")
	}
}

func TestServeFileHandler(t *testing.T) {
	dst := t.TempDir()
	writeTest(t, dst, "index.html", "<h1>home</h1>")
	writeTest(t, dst, "sub/index.html", "<h1>sub</h1>")
	// no baseurl
	h := fileHandler(dst, "", false)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "home") {
		t.Fatalf("index serve failed: %d %s", rec.Code, rec.Body.String())
	}
	// directory serves index.html even with listing disabled
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/sub/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "sub") {
		t.Fatalf("dir index failed: %d", rec.Code)
	}
	// with baseurl: prefix + redirect
	hb := fileHandler(dst, "/blog", true)
	rec = httptest.NewRecorder()
	hb.ServeHTTP(rec, httptest.NewRequest("GET", "/blog/", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "home") {
		t.Fatalf("baseurl serve failed: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	hb.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("root should redirect, got %d", rec.Code)
	}
}

func TestNoDirListingOpenError(t *testing.T) {
	n := noDirListing{fs: http.Dir(t.TempDir()), listing: false}
	if _, err := n.Open("/does-not-exist"); err == nil {
		t.Fatal("expected open error")
	}
	// listing enabled returns the file directly
	dir := t.TempDir()
	writeTest(t, dir, "index.html", "x")
	n2 := noDirListing{fs: http.Dir(dir), listing: true}
	f, err := n2.Open("/index.html")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	// nonListingFile.Readdir returns nothing
	n3 := noDirListing{fs: http.Dir(dir), listing: false}
	f3, _ := n3.Open("/")
	if infos, _ := f3.Readdir(-1); infos != nil {
		t.Fatal("Readdir should return nil")
	}
	f3.Close()
}

func TestRealServeLoopClosedListener(t *testing.T) {
	// Exercises the production serve loop deterministically on every OS: serving
	// on an already-closed listener returns immediately with an error, so there
	// is no blocking and no platform-specific shutdown timing.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
	if err := realServeLoop(&http.Server{}, ln); err == nil {
		t.Fatal("serving a closed listener should return an error")
	}
}

func TestServeDetachRealPort(t *testing.T) {
	dst := t.TempDir()
	writeTest(t, dst, "index.html", "hi")
	cfg := Config{"source": t.TempDir(), "destination": dst}
	site := NewSite(cfg)
	site.Dest = dst
	p := &parsed{vals: map[string]string{}, bools: map[string]bool{"detach": true}}
	// The detached serve loop is stubbed so no real server outlives the test.
	defer stubServeLoop(http.ErrServerClosed)()
	var out, errb strings.Builder
	if code := serveSite(site, "127.0.0.1", 0, p, &out, &errb); code != 0 {
		t.Fatalf("detach serve exit %d", code)
	}
}

func TestLookupVar(t *testing.T) {
	a := map[string]any{"page": map[string]any{"x": "v"}}
	if v, ok := lookupVar(a, "page.x"); !ok || v != "v" {
		t.Fatal("lookupVar dotted")
	}
	if _, ok := lookupVar(a, "page.missing"); ok {
		t.Fatal("missing key")
	}
	if _, ok := lookupVar(a, "page.x.y"); ok {
		t.Fatal("descend into non-map")
	}
}

func TestParseIncludeParamsForms(t *testing.T) {
	got := parseIncludeParams(` a="q" b='s' c=literal d=page.k `, map[string]any{"page": map[string]any{"k": "V"}})
	if got["a"] != "q" || got["b"] != "s" || got["c"] != "literal" || got["d"] != "V" {
		t.Fatalf("params: %+v", got)
	}
}

func TestKramdownOptionsOverrides(t *testing.T) {
	o := kramdownOptions(Config{"kramdown": map[string]any{"auto_ids": false, "hard_wrap": true}})
	if o.AutoIds || !o.HardWrap {
		t.Fatalf("kramdown overrides not applied: %+v", o)
	}
}

func TestMergeCollectionsListForm(t *testing.T) {
	dst := defaultConfig()
	mergeConfig(dst, map[string]any{"collections": []any{"authors", "team", 3}})
	cm := dst["collections"].(map[string]any)
	if _, ok := cm["authors"]; !ok {
		t.Fatal("list-form collection not merged")
	}
	if _, ok := cm["posts"]; !ok {
		t.Fatal("default posts collection lost")
	}
}

func TestPermalinkPlaceholders(t *testing.T) {
	d := &Document{
		date: time.Date(2023, 3, 9, 14, 5, 6, 0, time.UTC), slug: "post",
		outputExt: ".html", categories: []string{"news"}, ext: ".md",
	}
	url := applyPermalink("/:categories/:year/:i_month/:i_day/:hour/:minute/:second/:short_year/:y_day/:title:output_ext", d, "x.md")
	for _, want := range []string{"news", "2023", "/3/", "14", "05", "06", "/23/", "068", "post.html"} {
		if !strings.Contains(url, want) {
			t.Errorf("permalink %q missing %q", url, want)
		}
	}
	// pretty style + no leading slash template gets one
	if u := applyPermalink("no-slash/:title/", &Document{slug: "t"}, ""); !strings.HasPrefix(u, "/") {
		t.Fatalf("permalink should be rooted: %q", u)
	}
}

func TestResolvePermalinkStyle(t *testing.T) {
	if resolvePermalinkStyle("pretty") != "/:categories/:year/:month/:day/:title/" {
		t.Fatal("pretty style")
	}
	if resolvePermalinkStyle("/custom/:title/") != "/custom/:title/" {
		t.Fatal("custom passthrough")
	}
}

func TestUrlToOutputPath(t *testing.T) {
	if urlToOutputPath("/a/b.html") != "a/b.html" {
		t.Fatal("file url")
	}
	if urlToOutputPath("/a/") != "a/index.html" {
		t.Fatal("dir url")
	}
	if urlToOutputPath("/") != "index.html" {
		t.Fatal("root url")
	}
}

func TestFrontMatterEdge(t *testing.T) {
	// empty front matter block -> empty map
	d, body, ok, err := parseFrontMatter([]byte("---\n---\nbody\n"))
	if err != nil || !ok || len(d) != 0 || body != "body\n" {
		t.Fatalf("empty fm: %v %q %v %v", d, body, ok, err)
	}
	// no front matter
	if _, _, ok, _ := parseFrontMatter([]byte("plain")); ok {
		t.Fatal("no fm should be ok=false")
	}
	// invalid yaml
	if _, _, ok, err := parseFrontMatter([]byte("---\nx: [\n---\n")); !ok || err == nil {
		t.Fatal("invalid yaml should surface error with ok=true")
	}
}

func TestTitleizeAndToAnySlice(t *testing.T) {
	if titleize("hello-world_foo") != "Hello World Foo" {
		t.Fatalf("titleize=%q", titleize("hello-world_foo"))
	}
	if len(toAnySlice([]string{"a", "b"})) != 2 {
		t.Fatal("toAnySlice")
	}
}

func TestStringListForms(t *testing.T) {
	if got := stringList("a b"); len(got) != 2 {
		t.Fatal("string form")
	}
	if got := stringList([]any{"a", "b"}); len(got) != 2 {
		t.Fatal("[]any form")
	}
	if got := stringList([]string{"a"}); len(got) != 1 {
		t.Fatal("[]string form")
	}
	if got := stringList(42); got != nil {
		t.Fatal("other -> nil")
	}
}

func TestFinalizeHelpNoNewline(t *testing.T) {
	if finalizeHelp("x") != "x" {
		t.Fatal("no trailing newline unchanged")
	}
}

func TestRelativeURLEmptyBase(t *testing.T) {
	if got := relativeURL("", "x"); got != "/x" {
		t.Fatalf("relativeURL empty base got %q", got)
	}
}

func TestAtoiDefault(t *testing.T) {
	if atoiDefault("bad", 7) != 7 || atoiDefault("9", 0) != 9 {
		t.Fatal("atoiDefault")
	}
}

func TestWriteAndCopyErrors(t *testing.T) {
	// writeFile: parent is a file -> MkdirAll fails
	base := t.TempDir()
	fileAsDir := filepath.Join(base, "f")
	os.WriteFile(fileAsDir, []byte("x"), 0o644)
	if err := writeFile(filepath.Join(fileAsDir, "child"), []byte("y")); err == nil {
		t.Fatal("writeFile under a file should fail")
	}
	// copyFile: missing source
	if err := copyFile(filepath.Join(base, "nope"), filepath.Join(base, "dst")); err == nil {
		t.Fatal("copyFile missing src should fail")
	}
	// copyFile: bad dst parent
	if err := copyFile(fileAsDir, filepath.Join(fileAsDir, "x", "y")); err == nil {
		t.Fatal("copyFile bad dst should fail")
	}
}

func TestCleanKeepFiles(t *testing.T) {
	dst := t.TempDir()
	writeTest(t, dst, ".git/config", "keepme")
	writeTest(t, dst, "old.html", "stale")
	cfg := Config{"source": t.TempDir(), "destination": dst, "keep_files": []any{".git"}}
	s := NewSite(cfg)
	s.Dest = dst
	if err := s.clean(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".git")); err != nil {
		t.Fatal(".git should be kept")
	}
	if _, err := os.Stat(filepath.Join(dst, "old.html")); err == nil {
		t.Fatal("old.html should be removed")
	}
	// clean on a missing dest is a no-op
	s.Dest = filepath.Join(dst, "missing")
	if err := s.clean(); err != nil {
		t.Fatal("clean missing dest should be nil")
	}
}

func TestLoadLayoutVariants(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_layouts/md.md", "---\nlayout: base\n---\n{{ content }} md")
	cfg := Config{"source": src}
	s := NewSite(cfg)
	s.Source = src
	if _, data, err := s.loadLayout("md"); err != nil || data["layout"] != "base" {
		t.Fatalf("md layout: %v %v", data, err)
	}
	if _, _, err := s.loadLayout("nope"); err == nil {
		t.Fatal("missing layout should error")
	}
}
