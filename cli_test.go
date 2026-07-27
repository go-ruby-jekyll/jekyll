// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestHelpGolden asserts every subcommand's --help output is byte-for-byte
// identical to Ruby Jekyll 4.4.1 (captured in testdata/help).
func TestHelpGolden(t *testing.T) {
	cases := map[string]string{
		"root":      "help",
		"build":     "build",
		"serve":     "serve",
		"new":       "new",
		"new-theme": "new-theme",
		"clean":     "clean",
		"doctor":    "doctor",
		"docs":      "docs",
		"import":    "import",
		"compose":   "compose",
	}
	for file, cmd := range cases {
		want, err := os.ReadFile(filepath.Join("testdata/help", file+".txt"))
		if err != nil {
			t.Fatal(err)
		}
		var out bytes.Buffer
		args := []string{"help"}
		if cmd != "help" {
			args = []string{"help", cmd}
		}
		Main(args, &out, &out)
		if out.String() != string(want) {
			t.Errorf("help %s mismatch\n--- got ---\n%q\n--- want ---\n%q", cmd, out.String(), string(want))
		}
	}
}

func TestMainVersion(t *testing.T) {
	for _, a := range [][]string{{"-v"}, {"--version"}, {"build", "--version"}} {
		var out bytes.Buffer
		if code := Main(a, &out, &out); code != 0 {
			t.Fatalf("version exit %d", code)
		}
		if out.String() != "jekyll 4.4.1\n" {
			t.Fatalf("version got %q", out.String())
		}
	}
}

func TestMainRootHelpAndEmpty(t *testing.T) {
	for _, a := range [][]string{{}, {"-h"}, {"--help"}} {
		var out bytes.Buffer
		if code := Main(a, &out, &out); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if len(out.String()) == 0 {
			t.Fatal("expected root help")
		}
	}
}

func TestMainUnknownAndBadFlag(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main([]string{"bogus"}, &out, &errb); code != 1 {
		t.Fatalf("unknown command should exit 1, got %d", code)
	}
	errb.Reset()
	if code := Main([]string{"--nope"}, &out, &errb); code != 1 {
		t.Fatalf("leading bad flag should exit 1, got %d", code)
	}
	errb.Reset()
	if code := Main([]string{"build", "--nope"}, &out, &errb); code != 1 {
		t.Fatalf("bad option should exit 1, got %d", code)
	}
}

func TestPluginSubcommands(t *testing.T) {
	for _, name := range []string{"docs", "import", "compose"} {
		var out, errb bytes.Buffer
		if code := Main([]string{name}, &out, &errb); code != 1 {
			t.Fatalf("%s should exit 1 (not installed), got %d", name, code)
		}
	}
}

func TestParseArgsVariants(t *testing.T) {
	c := findCommand("serve")
	p, err := parseArgs(c, []string{
		"--source=src", "-d", "out", "-P4001", "--no-watch",
		"-qV", "--host", "0.0.0.0", "pos1", "--", "pos2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.vals["source"] != "src" || p.vals["destination"] != "out" || p.vals["port"] != "4001" {
		t.Fatalf("vals: %+v", p.vals)
	}
	if p.bools["watch"] { // --no-watch
		t.Fatal("--no-watch should set watch=false")
	}
	if !p.bools["quiet"] || !p.bools["verbose"] { // -qV bundle
		t.Fatal("bundled -qV not parsed")
	}
	if p.vals["host"] != "0.0.0.0" {
		t.Fatal("host not parsed")
	}
	if len(p.pos) != 2 || p.pos[0] != "pos1" || p.pos[1] != "pos2" {
		t.Fatalf("positional: %v", p.pos)
	}
}

func TestParseArgsErrors(t *testing.T) {
	c := findCommand("build")
	if _, err := parseArgs(c, []string{"--config"}); err == nil {
		t.Fatal("missing long argument should error")
	}
	if _, err := parseArgs(c, []string{"-s"}); err == nil {
		t.Fatal("missing short argument should error")
	}
	if _, err := parseArgs(c, []string{"--bogus"}); err == nil {
		t.Fatal("unknown long should error")
	}
	if _, err := parseArgs(c, []string{"-Z"}); err == nil {
		t.Fatal("unknown short should error")
	}
	if _, err := parseArgs(c, []string{"-qZ"}); err == nil {
		t.Fatal("bad bundled short should error")
	}
	if _, err := parseArgs(c, []string{"--no-quiet"}); err == nil {
		t.Fatal("--no- on a non-negatable flag should error")
	}
}

func TestParsedHas(t *testing.T) {
	p := &parsed{vals: map[string]string{"x": "1"}, bools: map[string]bool{"y": true}}
	if !p.has("x") || !p.has("y") || p.has("z") {
		t.Fatal("has() wrong")
	}
}
