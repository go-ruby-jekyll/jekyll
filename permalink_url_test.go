// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import "testing"

func TestDefaultPageURL(t *testing.T) {
	cases := []struct {
		rel, ext, outputExt, want string
	}{
		// A root index.html is served at "/" (the index.html is stripped),
		// matching Ruby Jekyll — verified against jekyll 4.4.1.
		{"index.html", ".html", ".html", "/"},
		{"index.md", ".md", ".html", "/"},
		// A nested index page is served at its directory URL.
		{"blog/index.html", ".html", ".html", "/blog/"},
		{"a/b/index.md", ".md", ".html", "/a/b/"},
		// Ordinary pages keep their path with the output extension.
		{"about.md", ".md", ".html", "/about.html"},
		{"docs/guide.html", ".html", ".html", "/docs/guide.html"},
		// A non-.html output (e.g. a data page) is never treated as an index
		// directory, even when named index.
		{"index.xml", ".xml", ".xml", "/index.xml"},
		{"feed/index.json", ".json", ".json", "/feed/index.json"},
	}
	for _, c := range cases {
		if got := defaultPageURL(c.rel, c.ext, c.outputExt); got != c.want {
			t.Errorf("defaultPageURL(%q,%q,%q) = %q, want %q",
				c.rel, c.ext, c.outputExt, got, c.want)
		}
	}
}

func TestPathBaseDir(t *testing.T) {
	if got := pathBase("index.html"); got != "index.html" {
		t.Errorf("pathBase bare = %q", got)
	}
	if got := pathDir("index.html"); got != "." {
		t.Errorf("pathDir bare = %q, want .", got)
	}
}
