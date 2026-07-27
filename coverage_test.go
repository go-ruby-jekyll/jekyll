// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestServeBlockingPaths(t *testing.T) {
	dst := t.TempDir()
	writeTest(t, dst, "index.html", "hi")
	site := NewSite(Config{"source": t.TempDir(), "destination": dst})
	site.Dest = dst
	p := &parsed{vals: map[string]string{}, bools: map[string]bool{}}

	// Success path: hook closes the server -> Serve returns ErrServerClosed -> 0.
	serveHook = func(srv *http.Server, _ net.Listener) {
		time.Sleep(5 * time.Millisecond)
		srv.Close()
	}
	var out, errb bytes.Buffer
	if code := serveSite(site, "127.0.0.1", 0, p, &out, &errb); code != 0 {
		t.Fatalf("blocking serve success exit %d", code)
	}

	// Error path: hook closes the listener -> Serve returns a non-graceful error -> 1.
	serveHook = func(_ *http.Server, ln net.Listener) {
		time.Sleep(5 * time.Millisecond)
		ln.Close()
	}
	if code := serveSite(site, "127.0.0.1", 0, p, &out, &errb); code != 1 {
		t.Fatalf("blocking serve error exit %d", code)
	}
	serveHook = nil
}

func TestCmdServeViaMainBlocking(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "index.md", "---\ntitle: H\n---\nx\n")
	serveHook = func(srv *http.Server, _ net.Listener) { srv.Close() }
	defer func() { serveHook = nil }()
	var out, errb bytes.Buffer
	if code := Main([]string{"serve", "-s", src, "-d", dst, "-P", "0"}, &out, &errb); code != 0 {
		t.Fatalf("serve via Main exit %d: %s", code, errb.String())
	}
	// skip-initial-build path
	if code := Main([]string{"serve", "-s", src, "-d", dst, "-P", "0", "--skip-initial-build"}, &out, &errb); code != 0 {
		t.Fatalf("serve --skip-initial-build exit %d", code)
	}
}

func TestCmdServeConfigAndBuildErrors(t *testing.T) {
	var out, errb bytes.Buffer
	// config error
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "  - [bad\n")
	if code := Main([]string{"serve", "-s", src, "-P", "0"}, &out, &errb); code != 1 {
		t.Fatalf("serve config error exit %d", code)
	}
	// build error (missing include)
	src2 := t.TempDir()
	writeTest(t, src2, "_config.yml", "title: T\n")
	writeTest(t, src2, "p.html", "---\ntitle: P\n---\n{% include nope.html %}\n")
	if code := Main([]string{"serve", "-s", src2, "-d", t.TempDir(), "-P", "0"}, &out, &errb); code != 1 {
		t.Fatalf("serve build error exit %d", code)
	}
}

func TestDispatchUnknown(t *testing.T) {
	var out, errb bytes.Buffer
	if code := dispatch("nonexistent", &parsed{bools: map[string]bool{}}, &out, &errb); code != 1 {
		t.Fatalf("dispatch unknown exit %d", code)
	}
}

func TestFindCommandAliasAndMissing(t *testing.T) {
	if findCommand("b") == nil || findCommand("hyde") == nil || findCommand("s") == nil {
		t.Fatal("aliases should resolve")
	}
	if findCommand("nope") != nil {
		t.Fatal("missing should be nil")
	}
}

func TestCollectionOutputPermalinkDefaults(t *testing.T) {
	s := NewSite(Config{"collections": map[string]any{"x": map[string]any{}}})
	if s.collectionOutput("x") {
		t.Fatal("no output key -> false")
	}
	if s.collectionOutput("missing") {
		t.Fatal("missing collection -> false")
	}
	if s.collectionPermalink("x") != "/:collection/:path/" {
		t.Fatal("default collection permalink")
	}
	if s.collectionPermalink("missing") != "/:collection/:path/" {
		t.Fatal("missing collection permalink default")
	}
}

func TestMustRel(t *testing.T) {
	if mustRel("relbase", "/abs/target") != "/abs/target" {
		t.Fatal("mustRel error path should return target")
	}
	if mustRel("/a", "/a/b") != "b" {
		t.Fatal("mustRel ok path")
	}
}

func TestToSVariants(t *testing.T) {
	tm := time.Date(2023, 1, 2, 3, 4, 5, 0, time.UTC)
	if toS(tm) != "2023-01-02 03:04:05 +0000" {
		t.Fatalf("toS(time)=%q", toS(tm))
	}
	if toS(42) != "42" {
		t.Fatal("toS(int)")
	}
	if toS("s") != "s" {
		t.Fatal("toS(string)")
	}
}

func TestExcludedPrefixAndNonString(t *testing.T) {
	s := NewSite(Config{"exclude": []any{"dir", 123}})
	if !s.excluded("dir/file.txt") {
		t.Fatal("prefix exclude")
	}
	if s.excluded("other.txt") {
		t.Fatal("non-excluded")
	}
}

func TestSkipDirCases(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "collections:\n  team:\n    output: true\n")
	writeTest(t, src, "_sass/x.scss", ".a{}\n")
	writeTest(t, src, ".hidden/f.txt", "x\n")
	writeTest(t, src, "_team/m.md", "---\nname: M\n---\nbio\n")
	writeTest(t, src, "normal/page.md", "---\ntitle: N\n---\nhi\n")
	dst := filepath.Join(src, "_site")
	s := buildIntoDst(t, src, dst, map[string]any{})
	// team collection doc should be read
	if len(s.collections["team"]) != 1 {
		t.Fatalf("team docs: %d", len(s.collections["team"]))
	}
}

func TestReadDataSkipsAndErrors(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_data/notes.txt", "ignored\n")
	writeTest(t, src, "_data/ok.yml", "a: 1\n")
	s := buildInto(t, src, map[string]any{})
	if _, ok := s.data["ok"]; !ok {
		t.Fatal("yml data should load")
	}
	if _, ok := s.data["notes"]; ok {
		t.Fatal("txt should be skipped")
	}
	// bad yaml in data -> Read error
	src2 := t.TempDir()
	writeTest(t, src2, "_config.yml", "title: T\n")
	writeTest(t, src2, "_data/bad.yml", ": : :\n- [\n")
	cfg, _ := LoadConfig(src2, nil)
	cfg["source"] = src2
	cfg["destination"] = t.TempDir()
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("bad data yaml should error")
	}
}

func TestBadFrontMatterNonStrict(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "bad.md", "---\nx: [\n---\nbody\n")
	// non-strict: bad fm is tolerated, file treated with raw body
	s := buildInto(t, src, map[string]any{})
	_ = s
}

func TestCollectionDocPermalinkDateAndNoFM(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "collections:\n  team:\n    output: true\n")
	writeTest(t, src, "_team/a.md", "---\nname: A\ndate: 2022-02-02\npermalink: /people/a/\n---\nbio\n")
	writeTest(t, src, "_team/nofm.md", "no front matter here\n")
	s := buildInto(t, src, map[string]any{})
	found := false
	for _, d := range s.collections["team"] {
		if d.url == "/people/a/" {
			found = true
		}
	}
	if !found {
		t.Fatal("collection permalink front matter not applied")
	}
}

func TestPostBadDateNameAndFMDate(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_posts/not-a-date.md", "---\ntitle: X\n---\nb\n")
	writeTest(t, src, "_posts/2023-01-01-withfm.md", "---\ntitle: Y\ndate: 2024-06-06\npermalink: /p/y/\n---\nb\n")
	s := buildInto(t, src, map[string]any{"future": true})
	var yURL string
	for _, p := range s.posts {
		if p.slug == "withfm" {
			yURL = p.url
		}
	}
	if yURL != "/p/y/" {
		t.Fatalf("post permalink fm: %q", yURL)
	}
}

func TestPagePermalink(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "custom.html", "---\ntitle: C\npermalink: /custom-path/\n---\nx\n")
	s := buildInto(t, src, map[string]any{})
	if s.pages[0].url != "/custom-path/" {
		t.Fatalf("page permalink: %q", s.pages[0].url)
	}
}

func TestProcessCollectionWithDatesAndTags(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "collections:\n  team:\n    output: true\n")
	writeTest(t, src, "_team/b.md", "---\nname: B\ndate: 2022-02-02\n---\nx\n")
	writeTest(t, src, "_team/a.md", "---\nname: A\ndate: 2021-01-01\n---\nx\n")
	writeTest(t, src, "_posts/2023-01-01-p.md", "---\ntitle: P\ntags: [go, jekyll]\n---\nx\n")
	s := buildInto(t, src, map[string]any{})
	// dated collection sorted ascending: a(2021) before b(2022)
	if s.collections["team"][0].slug != "a" {
		t.Fatalf("dated collection sort: %v", s.collections["team"][0].slug)
	}
	tags := s.sitePayload["tags"].(map[string]any)
	if _, ok := tags["go"]; !ok {
		t.Fatal("tags grouping missing")
	}
}

func TestApplyLayoutsMissingAndCycle(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	// missing layout -> build error
	writeTest(t, src, "p.md", "---\ntitle: P\nlayout: ghost\n---\nx\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("missing layout should error")
	}
	// self-referential layout -> seen guard stops
	src2 := t.TempDir()
	writeTest(t, src2, "_config.yml", "title: T\n")
	writeTest(t, src2, "_layouts/loop.html", "---\nlayout: loop\n---\n{{ content }}L")
	writeTest(t, src2, "q.md", "---\ntitle: Q\nlayout: loop\n---\nx\n")
	buildInto(t, src2, map[string]any{})
}

func TestLoadLayoutBadFrontMatterAndMarkdownExt(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_layouts/bad.html", "---\nx: [\n---\nbody {{ content }}")
	writeTest(t, src, "_layouts/md.markdown", "{{ content }} m")
	s := NewSite(Config{"source": src})
	s.Source = src
	if _, data, err := s.loadLayout("bad"); err != nil || len(data) != 0 {
		t.Fatalf("bad fm layout should reset data: %v %v", data, err)
	}
	if _, _, err := s.loadLayout("md"); err != nil {
		t.Fatalf(".markdown ext: %v", err)
	}
}

func TestWriteAndBuildDestErrors(t *testing.T) {
	// write() with a Dest whose parent is a file
	base := t.TempDir()
	fileAsDir := filepath.Join(base, "afile")
	os.WriteFile(fileAsDir, []byte("x"), 0o644)
	s := NewSite(Config{"source": base, "destination": filepath.Join(fileAsDir, "sub")})
	s.Dest = filepath.Join(fileAsDir, "sub")
	s.renderList = []*Document{{url: "/a.html", output: "x"}}
	if err := s.write(); err == nil {
		t.Fatal("write into bad dest should error")
	}
	// static file copy error
	s2 := NewSite(Config{"source": base, "destination": t.TempDir()})
	s2.staticFiles = []staticFile{{relPath: "x.txt", absPath: filepath.Join(base, "missing.txt")}}
	if err := s2.write(); err == nil {
		t.Fatal("write with missing static should error")
	}
}

func TestCleanReadDirError(t *testing.T) {
	base := t.TempDir()
	fileAsDest := filepath.Join(base, "f")
	os.WriteFile(fileAsDest, []byte("x"), 0o644)
	s := NewSite(Config{"source": base, "destination": fileAsDest})
	s.Dest = fileAsDest
	if err := s.clean(); err == nil {
		t.Fatal("clean on a file dest should error")
	}
}

func TestCmdBuildTraceAndErrors(t *testing.T) {
	// build error with --trace
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "p.html", "---\ntitle: P\n---\n{% include nope.html %}\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"build", "-s", src, "-d", t.TempDir(), "-t"}, &out, &errb); code != 1 {
		t.Fatalf("build error should exit 1, got %d", code)
	}
	if !strings.Contains(errb.String(), "Error") {
		t.Fatal("expected error output")
	}
	// clean error: dest is a file
	base := t.TempDir()
	fileDest := filepath.Join(base, "f")
	os.WriteFile(fileDest, []byte("x"), 0o644)
	src2 := t.TempDir()
	writeTest(t, src2, "_config.yml", "title: T\n")
	errb.Reset()
	if code := Main([]string{"build", "-s", src2, "-d", fileDest}, &out, &errb); code != 1 {
		t.Fatalf("clean error should exit 1, got %d", code)
	}
}

func TestCmdCleanError(t *testing.T) {
	base := t.TempDir()
	fileDest := filepath.Join(base, "f")
	os.WriteFile(fileDest, []byte("x"), 0o644)
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"clean", "-s", src, "-d", fileDest}, &out, &errb); code != 1 {
		t.Fatalf("clean error exit %d", code)
	}
}

func TestHighlightUnknownLangFallback(t *testing.T) {
	out := highlightFigure("code here", "no-such-lang-xyz")
	if !strings.Contains(out, "code here") {
		t.Fatalf("fallback should escape code: %s", out)
	}
	out2 := highlightBlock("x < y", "no-such-lang-xyz")
	if !strings.Contains(out2, "x &lt; y") {
		t.Fatalf("block fallback should escape: %s", out2)
	}
}

func TestPlaintextCodeBlock(t *testing.T) {
	html := convertMarkdown("```\nplain code\n```\n", Config{})
	if !strings.Contains(html, "language-plaintext") {
		t.Fatalf("no-lang block should be plaintext: %s", html)
	}
}

func TestMainHelpBogusSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"help", "bogus"}, &out, &errb); code != 0 {
		t.Fatalf("help bogus exit %d", code)
	}
	if !strings.Contains(out.String(), "Usage") {
		t.Fatal("help bogus should show root help")
	}
}

func TestScaffoldErrorBranches(t *testing.T) {
	// scaffoldSite where the target path is a file -> MkdirAll error
	base := t.TempDir()
	f := filepath.Join(base, "file")
	os.WriteFile(f, []byte("x"), 0o644)
	if err := scaffoldSite(filepath.Join(f, "site"), false); err == nil {
		t.Fatal("scaffoldSite under a file should error")
	}
	if err := scaffoldTheme(filepath.Join(f, "theme"), true); err == nil {
		t.Fatal("scaffoldTheme under a file should error")
	}
	// cmdNew/cmdNewTheme surface those errors
	var out, errb bytes.Buffer
	if code := Main([]string{"new", filepath.Join(f, "s")}, &out, &errb); code != 1 {
		t.Fatalf("cmdNew scaffold error exit %d", code)
	}
	if code := Main([]string{"new-theme", filepath.Join(f, "t")}, &out, &errb); code != 1 {
		t.Fatalf("cmdNewTheme scaffold error exit %d", code)
	}
}

func TestBuildDocumentReadError(t *testing.T) {
	s := NewSite(Config{"source": t.TempDir()})
	if _, _, err := s.buildDocument(filepath.Join(t.TempDir(), "nope.md"), "nope.md"); err == nil {
		t.Fatal("reading a missing file should error")
	}
}
