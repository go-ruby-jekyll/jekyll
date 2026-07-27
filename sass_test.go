// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSassStringList(t *testing.T) {
	cases := []struct {
		in   any
		want []string
	}{
		{"a", []string{"a"}},
		{[]any{"a", 1, "b"}, []string{"a", "b"}}, // non-string entries dropped
		{[]string{"x", "y"}, []string{"x", "y"}},
		{42, nil},
		{nil, nil},
	}
	for _, c := range cases {
		if got := sassStringList(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("sassStringList(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNewSassConverterConfig(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "abslib")
	s := &Site{
		Source: "/site/root",
		Config: Config{
			"sass": map[string]any{
				"sass_dir":   "styles",
				"style":      ":compressed",
				"load_paths": []any{"rel/lib", abs},
			},
		},
	}
	c := newSassConverter(s)
	if c.style != "compressed" {
		t.Fatalf("style = %q, want compressed", c.style)
	}
	want := []string{
		filepath.Join("/site/root", "rel/lib"), // relative → joined to source
		abs,                                    // absolute → kept verbatim
		filepath.Join("/site/root", "styles"),  // sass_dir last
	}
	if !reflect.DeepEqual(c.loadPaths, want) {
		t.Fatalf("loadPaths = %v, want %v", c.loadPaths, want)
	}
}

func TestNewSassConverterDefaults(t *testing.T) {
	// No sass config at all → default _sass dir, expanded style.
	c := newSassConverter(&Site{Source: "/root", Config: Config{}})
	if c.style != "expanded" {
		t.Fatalf("default style = %q, want expanded", c.style)
	}
	if len(c.loadPaths) != 1 || c.loadPaths[0] != filepath.Join("/root", "_sass") {
		t.Fatalf("default loadPaths = %v", c.loadPaths)
	}
}

func TestSassConvertIndentedAndMissingPath(t *testing.T) {
	// .sass (indented) syntax, with a non-existent load path that must be
	// filtered out before reaching the engine.
	c := SassConverter{loadPaths: []string{filepath.Join(t.TempDir(), "nope")}}
	got, err := c.convert(".a\n  color: red\n", ".SASS")
	if err != nil {
		t.Fatal(err)
	}
	if got != ".a {\n  color: red;\n}" {
		t.Fatalf("indented convert = %q", got)
	}
}
