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
	"testing"
)

func skipIfRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("permission-based error path is not observable as root")
	}
}

func TestApplyLayoutsRenderError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_layouts/broken.html", "{% badtag %}{{ content }}")
	writeTest(t, src, "p.md", "---\ntitle: P\nlayout: broken\n---\nx\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("broken layout liquid should error")
	}
}

func TestRenderDepthParseError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "p.html", "---\ntitle: P\n---\n{% badtag %}\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("bad liquid in page should error")
	}
}

func TestCopyFileMkdirError(t *testing.T) {
	base := t.TempDir()
	fileAsDir := filepath.Join(base, "f")
	os.WriteFile(fileAsDir, []byte("x"), 0o644)
	src := filepath.Join(base, "src.txt")
	os.WriteFile(src, []byte("y"), 0o644)
	if err := copyFile(src, filepath.Join(fileAsDir, "sub", "dst")); err == nil {
		t.Fatal("copyFile MkdirAll under a file should error")
	}
}

func TestCleanRemoveAllError(t *testing.T) {
	skipIfRoot(t)
	dst := t.TempDir()
	writeTest(t, dst, "keepdir/child.html", "x")
	// make dst read-only so RemoveAll of its entry fails
	if err := os.Chmod(dst, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dst, 0o755)
	s := NewSite(Config{"source": t.TempDir(), "destination": dst})
	s.Dest = dst
	if err := s.clean(); err == nil {
		t.Fatal("clean should fail when dest is read-only")
	}
}

func TestMainBuildHelpFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"build", "--help"}, &out, &errb); code != 0 {
		t.Fatalf("build --help exit %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("Build your site")) {
		t.Fatal("build --help should print build help")
	}
}

func TestCmdServeCleanErrorAndHost(t *testing.T) {
	// clean error: dest is a file
	base := t.TempDir()
	fileDest := filepath.Join(base, "f")
	os.WriteFile(fileDest, []byte("x"), 0o644)
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"serve", "-s", src, "-d", fileDest, "-P", "0"}, &out, &errb); code != 1 {
		t.Fatalf("serve clean error exit %d", code)
	}
	// host override path
	src2 := t.TempDir()
	writeTest(t, src2, "_config.yml", "title: T\n")
	writeTest(t, src2, "index.md", "---\ntitle: H\n---\nx\n")
	serveHook = func(srv *http.Server, _ net.Listener) { srv.Close() }
	defer func() { serveHook = nil }()
	if code := Main([]string{"serve", "-s", src2, "-d", t.TempDir(), "-H", "127.0.0.1", "-P", "0"}, &out, &errb); code != 0 {
		t.Fatalf("serve with host exit %d", code)
	}
}

func TestDataNilFrontMatter(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	// front matter that YAML-parses to nil (comment only)
	writeTest(t, src, "p.html", "---\n# just a comment\n---\nbody\n")
	s := buildInto(t, src, map[string]any{})
	if len(s.pages) != 1 {
		t.Fatalf("comment-only front matter should still be a page: %d", len(s.pages))
	}
}

func TestJsonifyErrorAndSentenceConn(t *testing.T) {
	f := jekyllFilters(Config{})
	if _, err := f["jsonify"](make(chan int), nil); err == nil {
		t.Fatal("jsonify of a channel should error")
	}
	v, _ := f["array_to_sentence_string"]([]any{"a", "b", "c"}, []any{"or"})
	if v != "a, b, or c" {
		t.Fatalf("connector arg: %v", v)
	}
}

func TestRelativeURLLeadingSlash(t *testing.T) {
	if got := relativeURL("blog", ""); got != "/blog" {
		t.Fatalf("relativeURL(blog,'')=%q", got)
	}
}

func TestExpandIncludesShortCircuit(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	// two includes; the first is missing so the second must short-circuit
	writeTest(t, src, "_includes/ok.html", "OK")
	writeTest(t, src, "p.html", "---\ntitle: P\n---\n{% include missing.html %}{% include ok.html %}\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("missing include should error")
	}
}

func TestReadIncludeDefaultDir(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_includes/x.html", "X")
	s := NewSite(Config{"source": src})
	s.Source = src
	// includes_dir is empty -> default _includes
	got, err := s.r.readInclude("x.html")
	if err != nil || got != "X" {
		t.Fatalf("readInclude default dir: %q %v", got, err)
	}
}

func TestPostURLNotFound(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "p.html", "---\ntitle: P\n---\n[{% post_url does-not-exist %}]\n")
	dst := t.TempDir()
	buildIntoDst(t, src, dst, map[string]any{})
	out, _ := os.ReadFile(filepath.Join(dst, "p.html"))
	if !bytes.Contains(out, []byte("[]")) {
		t.Fatalf("unresolved post_url should be empty: %s", out)
	}
}

func TestScaffoldWriteFileError(t *testing.T) {
	// Pre-create one of the target files as a directory so writeFile fails.
	base := t.TempDir()
	site := filepath.Join(base, "s")
	os.MkdirAll(filepath.Join(site, "_config.yml"), 0o755) // _config.yml is a dir
	if err := scaffoldSite(site, false); err == nil {
		t.Fatal("scaffoldSite should fail when a target is a directory")
	}
	theme := filepath.Join(base, "t")
	os.MkdirAll(filepath.Join(theme, "Gemfile"), 0o755)
	if err := scaffoldTheme(theme, false); err == nil {
		t.Fatal("scaffoldTheme should fail when a target is a directory")
	}
}

func TestReadMissingSource(t *testing.T) {
	s := NewSite(Config{"source": filepath.Join(t.TempDir(), "does-not-exist")})
	s.Source = filepath.Join(t.TempDir(), "does-not-exist")
	if err := s.Read(); err == nil {
		t.Fatal("Read on a missing source should error")
	}
}

func TestSkipDirDestAndUnderscore(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	// pre-existing _site (dest inside source) and a private underscore dir
	writeTest(t, src, "_site/old.html", "stale")
	writeTest(t, src, "_private/secret.txt", "x")
	writeTest(t, src, "index.md", "---\ntitle: H\n---\nx\n")
	dst := filepath.Join(src, "_site")
	s := NewSite(Config{"source": src, "destination": dst})
	s.Source, _ = filepath.Abs(src)
	s.Dest = dst
	s.filters = jekyllFilters(s.Config)
	// Read should skip _site and _private without error.
	if err := s.Read(); err != nil {
		t.Fatal(err)
	}
}

func TestReadDataDefaultDir(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_data/x.yml", "k: v\n")
	s := &Site{Config: Config{}, Source: src, data: map[string]any{}}
	if err := s.readData(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.data["x"]; !ok {
		t.Fatal("readData default dir should load x")
	}
}

func TestNonHTMLNonMarkdownPage(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "feed.xml", "---\ntitle: Feed\n---\n<rss>{{ site.title }}</rss>\n")
	dst := t.TempDir()
	s := buildIntoDst(t, src, dst, map[string]any{})
	found := false
	for _, d := range s.pages {
		if d.url == "/feed.xml" {
			found = true
		}
	}
	if !found {
		t.Fatal("xml page should keep .xml output ext")
	}
	out, _ := os.ReadFile(filepath.Join(dst, "feed.xml"))
	if !bytes.Contains(out, []byte("<rss>T</rss>")) {
		t.Fatalf("xml page should render liquid: %s", out)
	}
}

func TestCopyFileCreateError(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "s.txt")
	os.WriteFile(src, []byte("x"), 0o644)
	existingDir := filepath.Join(base, "d")
	os.MkdirAll(existingDir, 0o755)
	// dst is an existing directory -> os.Create fails.
	if err := copyFile(src, existingDir); err == nil {
		t.Fatal("copyFile onto a directory should fail")
	}
}

func TestNullFrontMatter(t *testing.T) {
	data, body, ok, err := parseFrontMatter([]byte("---\n~\n---\nbody\n"))
	if err != nil || !ok {
		t.Fatalf("null fm: ok=%v err=%v", ok, err)
	}
	if data == nil || len(data) != 0 {
		t.Fatalf("null front matter should yield empty map, got %v", data)
	}
	if body != "body\n" {
		t.Fatalf("body=%q", body)
	}
}

func TestSkipDirExcluded(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "exclude:\n  - vendor\n")
	writeTest(t, src, "vendor/lib.txt", "x")
	writeTest(t, src, "index.md", "---\ntitle: H\n---\nx\n")
	dst := t.TempDir()
	buildIntoDst(t, src, dst, map[string]any{})
	if _, err := os.Stat(filepath.Join(dst, "vendor")); err == nil {
		t.Fatal("excluded dir should be skipped")
	}
}

func TestReadDataReadError(t *testing.T) {
	src := t.TempDir()
	dataDir := filepath.Join(src, "_data")
	os.MkdirAll(dataDir, 0o755)
	// A broken symlink named *.yml: WalkDir sees a file, ReadFile fails.
	if err := os.Symlink(filepath.Join(src, "nonexistent-target"), filepath.Join(dataDir, "broken.yml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	s := &Site{Config: Config{"data_dir": "_data"}, Source: src, data: map[string]any{}}
	if err := s.readData(); err == nil {
		t.Fatal("readData should fail on an unreadable file")
	}
}

func TestPostWithoutFrontMatterSkipped(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_posts/2023-01-01-plain.md", "no front matter\n")
	s := buildInto(t, src, map[string]any{})
	if len(s.posts) != 0 {
		t.Fatalf("post without front matter should be skipped, got %d", len(s.posts))
	}
}
