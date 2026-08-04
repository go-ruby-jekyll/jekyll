// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"strings"
	"testing"
)

func TestExcerptEmptySeparator(t *testing.T) {
	// excerpt_separator: "" -> the whole body is the excerpt.
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nexcerpt_separator: \"\"\nplugins:\n  - jekyll-feed\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\nOne.\n\nTwo.\n")
	feed := buildFeedString(t, buildInto(t, src, nil))
	if !strings.Contains(feed, "One. Two.") {
		t.Fatalf("empty separator should include whole body: %s", feed)
	}
}

func TestExcerptFrontMatterSeparator(t *testing.T) {
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\nplugins:\n  - jekyll-feed\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\nexcerpt_separator: \"<!--more-->\"\n---\nIntro here.<!--more-->Rest.\n")
	feed := buildFeedString(t, buildInto(t, src, nil))
	if !strings.Contains(feed, "<![CDATA[Intro here.]]></summary>") {
		t.Fatalf("front-matter separator not honoured: %s", feed)
	}
}

func TestExcerptNonMarkdownCollection(t *testing.T) {
	// A custom-collection .html document exercises the non-markdown excerpt path.
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\ncollections:\n  notes:\n    output: true\n")
	writeTest(t, src, "_notes/n.html", "---\ntitle: N\n---\n<p>Hi</p>\n")
	s := buildInto(t, src, nil)
	var found bool
	for _, d := range s.collections["notes"] {
		if strings.Contains(toS(s.docMap(d)["excerpt"]), "<p>Hi</p>") {
			found = true
		}
	}
	if !found {
		t.Fatal("non-markdown collection excerpt not computed")
	}
}

func TestExcerptRenderError(t *testing.T) {
	// A Liquid block split across the excerpt separator renders as full content
	// but leaves the excerpt head unbalanced, surfacing a render error.
	src := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_posts/2023-01-01-a.md", "---\ntitle: A\n---\n{% if true %}\n\nhi{% endif %}\n")
	cfg := mustCfg(t, src)
	if err := NewSite(cfg).Build(); err == nil {
		t.Fatal("excerpt render error should fail the build")
	}
}
