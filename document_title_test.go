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

// TestUntitledPageVsPost pins Jekyll's title semantics: a regular page
// (Jekyll::Page) with no front-matter title is left untitled, while a collection
// document such as a post (Jekyll::Document) derives a titleized title from its
// slug. Fabricating a page title was the dominant nav diff against minima.
func TestUntitledPageVsPost(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "plain-page.html", "---\n---\ntitle=[{{ page.title }}]\n")
	writeTest(t, src, "_posts/2023-01-02-my-first-post.md", "---\n---\ntitle=[{{ page.title }}]\n")
	buildIntoDst(t, src, dst, map[string]any{})

	pg, err := os.ReadFile(filepath.Join(dst, "plain-page.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pg), "title=[]") {
		t.Errorf("untitled page must not fabricate a title: %q", pg)
	}

	post, err := os.ReadFile(filepath.Join(dst, "2023", "01", "02", "my-first-post.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(post), "title=[My First Post]") {
		t.Errorf("untitled post must be titleized from slug: %q", post)
	}
}
