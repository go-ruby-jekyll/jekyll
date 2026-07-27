// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDifferentialGolden is the core parity test: it builds testdata/site and
// asserts the output is byte-for-byte identical to testdata/expected, which was
// produced by Ruby Jekyll 4.4.1.
func TestDifferentialGolden(t *testing.T) {
	dst := t.TempDir()
	cfg, err := LoadConfig("testdata/site", nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg["source"] = "testdata/site"
	cfg["destination"] = dst
	site := NewSite(cfg)
	if err := site.Build(); err != nil {
		t.Fatal(err)
	}
	expectedRoot := "testdata/expected"
	var got, want []string
	filepath.WalkDir(dst, func(p string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(dst, p)
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	filepath.WalkDir(expectedRoot, func(p string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(expectedRoot, p)
			want = append(want, filepath.ToSlash(rel))
		}
		return nil
	})
	if strings.Join(sortedCopy(got), "\n") != strings.Join(sortedCopy(want), "\n") {
		t.Fatalf("file tree mismatch\n got: %v\nwant: %v", sortedCopy(got), sortedCopy(want))
	}
	for _, rel := range want {
		g, _ := os.ReadFile(filepath.Join(dst, rel))
		w, _ := os.ReadFile(filepath.Join(expectedRoot, rel))
		if string(g) != string(w) {
			t.Errorf("%s differs\n--- got ---\n%s\n--- want ---\n%s", rel, g, w)
		}
	}
}

func sortedCopy(s []string) []string {
	c := append([]string(nil), s...)
	for i := range c {
		for j := i + 1; j < len(c); j++ {
			if c[j] < c[i] {
				c[i], c[j] = c[j], c[i]
			}
		}
	}
	return c
}

func TestDraftsFutureLimit(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\nbody a\n")
	writeTest(t, src, "_posts/2999-01-01-future.md", "---\ntitle: F\n---\nfuture\n")
	writeTest(t, src, "_drafts/d.md", "---\ntitle: D\n---\ndraft\n")

	// default: no drafts, no future
	s := buildInto(t, src, map[string]any{})
	if len(s.posts) != 1 {
		t.Fatalf("want 1 post, got %d", len(s.posts))
	}
	// drafts + future
	s2 := buildInto(t, src, map[string]any{"show_drafts": true, "future": true})
	if len(s2.posts) != 3 {
		t.Fatalf("want 3 posts with drafts+future, got %d", len(s2.posts))
	}
	// limit_posts
	s3 := buildInto(t, src, map[string]any{"future": true, "limit_posts": 1})
	if len(s3.posts) != 1 {
		t.Fatalf("limit_posts=1 want 1, got %d", len(s3.posts))
	}
}

func TestUnpublished(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "hidden.md", "---\ntitle: H\npublished: false\n---\nx\n")
	s := buildInto(t, src, map[string]any{})
	if len(s.pages) != 0 {
		t.Fatalf("published:false page should be dropped, got %d", len(s.pages))
	}
	s2 := buildInto(t, src, map[string]any{"unpublished": true})
	if len(s2.pages) != 1 {
		t.Fatalf("--unpublished should keep it, got %d", len(s2.pages))
	}
}

func TestSassPassthrough(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "style.scss", "---\n---\n.a { color: red; }\n")
	s := buildInto(t, src, map[string]any{})
	if len(s.warnings) == 0 {
		t.Fatal("expected a Sass warning")
	}
	// URL maps to .css
	found := false
	for _, d := range s.pages {
		if strings.HasSuffix(d.url, "style.css") {
			found = true
		}
	}
	if !found {
		t.Fatal("scss should map to .css url")
	}
}

func TestStrictFrontMatter(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "bad.md", "---\ntitle: [unterminated\n---\nx\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	cfg["strict_front_matter"] = true
	site := NewSite(cfg)
	if err := site.Build(); err == nil {
		t.Fatal("strict_front_matter should fail the build on bad YAML")
	}
}

func TestExcludeAndStatic(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "exclude:\n  - secret.txt\n")
	writeTest(t, src, "secret.txt", "nope\n")
	writeTest(t, src, "robots.txt", "ok\n")
	dst := t.TempDir()
	s := buildIntoDst(t, src, dst, map[string]any{})
	_ = s
	if _, err := os.Stat(filepath.Join(dst, "secret.txt")); err == nil {
		t.Fatal("excluded file should not be written")
	}
	if _, err := os.Stat(filepath.Join(dst, "robots.txt")); err != nil {
		t.Fatal("static file should be copied")
	}
}

// ---- helpers ----

func writeTest(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func buildInto(t *testing.T, src string, overrides map[string]any) *Site {
	return buildIntoDst(t, src, t.TempDir(), overrides)
}

func buildIntoDst(t *testing.T, src, dst string, overrides map[string]any) *Site {
	t.Helper()
	cfg, err := LoadConfig(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg["source"] = src
	cfg["destination"] = dst
	for k, v := range overrides {
		cfg[k] = v
	}
	site := NewSite(cfg)
	if err := site.Build(); err != nil {
		t.Fatal(err)
	}
	return site
}
