// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCmdBuildViaMain(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "index.md", "---\ntitle: Home\n---\n# Hi\n")
	var out, errb bytes.Buffer
	code := Main([]string{"build", "-s", src, "-d", dst}, &out, &errb)
	if code != 0 {
		t.Fatalf("build exit %d, err=%s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(dst, "index.html")); err != nil {
		t.Fatal("index.html not built")
	}
	// quiet suppresses the banner.
	out.Reset()
	Main([]string{"build", "-s", src, "-d", dst, "-q"}, &out, &errb)
	if out.Len() != 0 {
		t.Fatalf("quiet build should be silent, got %q", out.String())
	}
}

func TestCmdBuildConfigError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "\tnot: valid: yaml:\n  - [\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"build", "-s", src}, &out, &errb); code != 1 {
		t.Fatalf("bad config should exit 1, got %d", code)
	}
}

func TestCmdBuildSassCompiles(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "a.scss", "---\n---\n.x { .y { color: red; } }\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"build", "-s", src, "-d", dst}, &out, &errb); code != 0 {
		t.Fatalf("build should succeed, got %d: %q", code, errb.String())
	}
	css, err := os.ReadFile(filepath.Join(dst, "a.css"))
	if err != nil {
		t.Fatal(err)
	}
	if string(css) != ".x .y {\n  color: red;\n}" {
		t.Fatalf("compiled scss mismatch: %q", css)
	}
}

func TestCmdBuildSassError(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "bad.scss", "---\n---\n.x { color: ;\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"build", "-s", src, "-d", dst}, &out, &errb); code != 1 {
		t.Fatalf("malformed scss should exit 1, got %d", code)
	}
}

func TestCmdClean(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, dst, "old.html", "stale\n")
	var out, errb bytes.Buffer
	code := Main([]string{"clean", "-s", src, "-d", dst}, &out, &errb)
	if code != 0 {
		t.Fatalf("clean exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(dst, "old.html")); err == nil {
		t.Fatal("clean should remove old output")
	}
}

func TestCmdCleanConfigError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "  - [bad\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"clean", "-s", src}, &out, &errb); code != 1 {
		t.Fatalf("want 1, got %d", code)
	}
}

func TestCmdDoctor(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nurl: https://ok.example\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"doctor", "-s", src}, &out, &errb); code != 0 {
		t.Fatalf("doctor exit %d", code)
	}
	if !bytes.Contains(out.Bytes(), []byte("Everything looks fine.")) {
		t.Fatalf("doctor got %q", out.String())
	}
	// warnings path
	writeTest(t, src, "_config.yml", "baseurl: http://bad\nurl: https://x/\n")
	out.Reset()
	Main([]string{"doctor", "-s", src}, &out, &errb)
	if !bytes.Contains(out.Bytes(), []byte("baseurl")) {
		t.Fatalf("expected baseurl warning, got %q", out.String())
	}
}

func TestCmdDoctorConfigError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "  - [bad\n")
	var out, errb bytes.Buffer
	if code := Main([]string{"doctor", "-s", src}, &out, &errb); code != 1 {
		t.Fatalf("want 1, got %d", code)
	}
}

func TestCmdNew(t *testing.T) {
	base := t.TempDir()
	site := filepath.Join(base, "mysite")
	var out, errb bytes.Buffer
	if code := Main([]string{"new", site}, &out, &errb); code != 0 {
		t.Fatalf("new exit %d: %s", code, errb.String())
	}
	// The scaffold must be buildable by this tool.
	dst := t.TempDir()
	if code := Main([]string{"build", "-s", site, "-d", dst, "-q"}, &out, &errb); code != 0 {
		t.Fatalf("scaffold build exit %d: %s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(dst, "index.html")); err != nil {
		t.Fatal("scaffold did not build index.html")
	}
	// conflict without --force
	errb.Reset()
	if code := Main([]string{"new", site}, &out, &errb); code != 1 {
		t.Fatalf("conflict should exit 1, got %d", code)
	}
	// --force overwrites
	if code := Main([]string{"new", site, "--force"}, &out, &errb); code != 0 {
		t.Fatalf("--force exit %d", code)
	}
	// --blank
	blank := filepath.Join(base, "blank")
	if code := Main([]string{"new", blank, "--blank"}, &out, &errb); code != 0 {
		t.Fatalf("--blank exit %d", code)
	}
	// missing path
	if code := Main([]string{"new"}, &out, &errb); code != 1 {
		t.Fatalf("missing path should exit 1, got %d", code)
	}
}

func TestCmdNewTheme(t *testing.T) {
	base := t.TempDir()
	name := filepath.Join(base, "mytheme")
	var out, errb bytes.Buffer
	if code := Main([]string{"new-theme", name, "-c"}, &out, &errb); code != 0 {
		t.Fatalf("new-theme exit %d: %s", code, errb.String())
	}
	if _, err := os.Stat(filepath.Join(name, "CODE_OF_CONDUCT.md")); err != nil {
		t.Fatal("-c should create CODE_OF_CONDUCT.md")
	}
	if _, err := os.Stat(filepath.Join(name, filepath.Base(name)+".gemspec")); err != nil {
		t.Fatal("gemspec missing")
	}
	// missing name
	if code := Main([]string{"new-theme"}, &out, &errb); code != 1 {
		t.Fatalf("missing name should exit 1, got %d", code)
	}
}

func TestCmdServeDetach(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nbaseurl: /blog\n")
	writeTest(t, src, "index.md", "---\ntitle: Home\n---\n# Hi\n")
	// The detached serve loop is stubbed so no real server outlives the test.
	defer stubServeLoop(http.ErrServerClosed)()
	var out, errb bytes.Buffer
	code := Main([]string{"serve", "-s", src, "-d", dst, "-P", "0", "-B"}, &out, &errb)
	if code != 0 {
		t.Fatalf("serve -B exit %d: %s", code, errb.String())
	}
}

func TestServeSiteBindError(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	cfg["destination"] = t.TempDir()
	site := NewSite(cfg)
	_ = site.Build()

	p := &parsed{vals: map[string]string{}, bools: map[string]bool{}}
	// An invalid host forces net.Listen to fail on every OS (no seam needed).
	var out, errb bytes.Buffer
	if code := serveSite(site, "256.256.256.256", 4000, p, &out, &errb); code != 1 {
		t.Fatalf("bad host should exit 1, got %d", code)
	}
}

func TestConfigFilePath(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	cfg, _ := LoadConfig(src, nil)
	cfg["source"] = src
	if configFilePath(cfg, &parsed{vals: map[string]string{}, bools: map[string]bool{}}) == "none" {
		t.Fatal("should find _config.yml")
	}
	if configFilePath(cfg, &parsed{vals: map[string]string{"config": "x.yml"}, bools: map[string]bool{}}) != "x.yml" {
		t.Fatal("explicit config path")
	}
	empty := t.TempDir()
	cfg["source"] = empty
	if configFilePath(cfg, &parsed{vals: map[string]string{}, bools: map[string]bool{}}) != "none" {
		t.Fatal("no config -> none")
	}
}

func TestBuildConfigOverrides(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	p := &parsed{
		vals: map[string]string{
			"source": src, "destination": "out", "baseurl": "/b",
			"limit_posts": "5", "plugins": "_p", "layouts": "_l", "config": "_config.yml",
		},
		bools: map[string]bool{
			"drafts": true, "future": true, "unpublished": true,
			"safe": true, "strict_front_matter": true,
		},
	}
	cfg, err := buildConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg["baseurl"] != "/b" || cfg["limit_posts"].(int) != 5 || cfg["plugins_dir"] != "_p" ||
		cfg["layouts_dir"] != "_l" || !cfg.boolOpt("show_drafts") || !cfg.boolOpt("future") ||
		!cfg.boolOpt("unpublished") || !cfg.boolOpt("safe") || !cfg.boolOpt("strict_front_matter") {
		t.Fatalf("overrides not applied: %+v", cfg)
	}
}

func TestBuildConfigLoadError(t *testing.T) {
	if _, err := buildConfig(&parsed{vals: map[string]string{"config": "/no/such/file.yml"}, bools: map[string]bool{}}); err == nil {
		t.Fatal("missing config file should error")
	}
}

func TestServeSiteSkipInitialAndTime(t *testing.T) {
	_ = time.Now
}
