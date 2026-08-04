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

// gemsFixture is the absolute path of the vendored theme-gem fixture root (the
// directory a real gem installation calls "gems").
func gemsFixture(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", "gemroot", "gems"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// withThemeGems points theme resolution at the fixture for the duration of a
// test.
func withThemeGems(t *testing.T) {
	t.Helper()
	root := gemsFixture(t)
	prev := gemThemeRoots
	gemThemeRoots = func() []string { return []string{root} }
	t.Cleanup(func() { gemThemeRoots = prev })
}

func TestResolveThemePicksHighestVersion(t *testing.T) {
	withThemeGems(t)
	th := resolveTheme("testtheme")
	if !strings.HasSuffix(th.Root, "testtheme-1.2.0") {
		t.Fatalf("want testtheme-1.2.0, got %q", th.Root)
	}
	if got := resolveTheme("nosuchtheme").Root; got != "" {
		t.Fatalf("missing theme should resolve to empty, got %q", got)
	}
}

func TestThemeLayoutAndInclude(t *testing.T) {
	withThemeGems(t)
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "theme: testtheme\ntitle: T\n")
	writeTest(t, src, "index.html", "---\nlayout: base\n---\nHELLO")
	buildIntoDst(t, src, dst, nil)
	got := readOut(t, dst, "index.html")
	if got != "<html>HELLO|THEME-FOOTER</html>\n" {
		t.Fatalf("theme layout/include not applied: %q", got)
	}
}

func TestThemeAssetsCompiledAndCopied(t *testing.T) {
	withThemeGems(t)
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "theme: testtheme\ntitle: T\n")
	writeTest(t, src, "index.html", "---\nlayout: base\n---\nX")
	buildIntoDst(t, src, dst, nil)
	// Theme assets/style.scss compiles via the theme's _sass partial.
	if css := readOut(t, dst, "assets/style.css"); css != ".t {\n  color: green;\n}" {
		t.Fatalf("theme scss not compiled with theme _sass: %q", css)
	}
	// Theme static asset copied verbatim.
	if logo := readOut(t, dst, "assets/logo.txt"); logo != "THEME-LOGO\n" {
		t.Fatalf("theme static asset not copied: %q", logo)
	}
}

func TestSiteOverridesTheme(t *testing.T) {
	withThemeGems(t)
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "theme: testtheme\ntitle: T\n")
	writeTest(t, src, "index.html", "---\nlayout: base\n---\nY")
	// Site files that shadow the theme's layout, include and asset.
	writeTest(t, src, "_layouts/base.html", "SITE[{{ content }}|{% include tfoot.html %}]")
	writeTest(t, src, "_includes/tfoot.html", "SITE-FOOTER")
	writeTest(t, src, "assets/logo.txt", "SITE-LOGO\n")
	// A site page at the same path as a theme asset also wins (urlByPath path).
	writeTest(t, src, "assets/style.scss", "---\n---\n.s { color: blue; }\n")
	buildIntoDst(t, src, dst, nil)
	if got := readOut(t, dst, "index.html"); got != "SITE[Y|SITE-FOOTER]" {
		t.Fatalf("site should override theme layout+include: %q", got)
	}
	if got := readOut(t, dst, "assets/logo.txt"); got != "SITE-LOGO\n" {
		t.Fatalf("site should override theme asset: %q", got)
	}
	if got := readOut(t, dst, "assets/style.css"); got != ".s {\n  color: blue;\n}" {
		t.Fatalf("site scss page should override theme asset: %q", got)
	}
}

func TestThemeWithoutAssets(t *testing.T) {
	withThemeGems(t)
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "theme: noassets\ntitle: T\n")
	writeTest(t, src, "index.html", "---\nlayout: base\n---\nQ")
	buildIntoDst(t, src, dst, nil)
	if got := readOut(t, dst, "index.html"); got != "NOASSETS:Q\n" {
		t.Fatalf("theme without assets should still supply layouts: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "assets")); !os.IsNotExist(err) {
		t.Fatal("no assets dir should be produced")
	}
}

func TestThemeMissingIsLenient(t *testing.T) {
	// No theme configured: themeDir helpers return "" and no assets are read.
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "index.html", "---\n---\nZ")
	s := buildIntoDst(t, src, dst, nil)
	if s.themeDir("_layouts") != "" {
		t.Fatal("themeDir should be empty with no theme")
	}
}

func readOut(t *testing.T, dst, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dst, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestGemRootResolutionHelpers(t *testing.T) {
	sep := string(os.PathListSeparator)
	roots := gemRootsFromEnv("/a", "/b"+sep+"/a", "/c"+sep+"")
	want := []string{
		filepath.Join("/a", "gems"),
		filepath.Join("/b", "gems"),
		filepath.Join("/c", "gems"),
	}
	if strings.Join(roots, "|") != strings.Join(want, "|") {
		t.Fatalf("gemRootsFromEnv=%v want %v", roots, want)
	}
	// findThemeInRoots ignores unreadable roots and non-matching entries.
	if got := findThemeInRoots([]string{filepath.Join(t.TempDir(), "absent")}, "x"); got != "" {
		t.Fatalf("missing root should yield empty, got %q", got)
	}
	// A directory whose version suffix is a pre-release tag is skipped.
	if v := parseGemVersion("1.0.0.pre"); len(v) != 3 {
		t.Fatalf("pre-release parse: %v", v)
	}
	if parseGemVersion("beta") != nil {
		t.Fatal("non-numeric version should be nil")
	}
	if compareGemVersion([]int{1, 2}, []int{1, 2, 0}) != 0 {
		t.Fatal("1.2 should equal 1.2.0")
	}
	if compareGemVersion([]int{2}, []int{1, 9}) <= 0 {
		t.Fatal("2 > 1.9")
	}
	if compareGemVersion([]int{1, 0}, []int{1, 2}) >= 0 {
		t.Fatal("1.0 < 1.2")
	}
}

func TestDefaultGemThemeRootsAndEnv(t *testing.T) {
	// Exercise the default env-driven resolver with a stubbed
	// `gem environment gempath`, covering both its success and failure paths.
	prev := gemGempathOutput
	t.Cleanup(func() { gemGempathOutput = prev })

	t.Setenv("GEM_HOME", "/gh")
	t.Setenv("GEM_PATH", "")
	gemGempathOutput = func() ([]byte, error) { return []byte("  /ge\n"), nil }
	roots := defaultGemThemeRoots()
	if len(roots) != 2 || roots[0] != filepath.Join("/gh", "gems") || roots[1] != filepath.Join("/ge", "gems") {
		t.Fatalf("defaultGemThemeRoots=%v", roots)
	}

	// Failure path: gem unavailable → empty gempath, so only GEM_HOME remains.
	gemGempathOutput = func() ([]byte, error) { return nil, os.ErrNotExist }
	if got := defaultGemThemeRoots(); len(got) != 1 {
		t.Fatalf("with gem absent want 1 root, got %v", got)
	}

	// The real command runner must execute at least once (its result is ignored;
	// it may succeed or fail depending on whether `gem` is installed).
	_, _ = prev()
}
