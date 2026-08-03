// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// frontMatterRe matches a leading YAML front-matter block delimited by lines of
// three or more hyphens, exactly like Jekyll's Document::YAML_FRONT_MATTER_REGEXP.
var frontMatterRe = regexp.MustCompile(`(?ms)\A---\s*\r?\n(.*?\n?)^((---|\.\.\.)\s*$\r?\n?)`)

// hasFrontMatter reports whether raw begins with a YAML front-matter block.
func hasFrontMatter(raw []byte) bool {
	return frontMatterRe.Match(raw)
}

// parseFrontMatter splits raw into its parsed front-matter map and the remaining
// body. It returns ok=false when there is no front-matter block.
func parseFrontMatter(raw []byte) (data map[string]any, body string, ok bool, err error) {
	m := frontMatterRe.FindSubmatchIndex(raw)
	if m == nil {
		return nil, string(raw), false, nil
	}
	yamlBlock := raw[m[2]:m[3]]
	body = string(raw[m[1]:])
	data = map[string]any{}
	if len(strings.TrimSpace(string(yamlBlock))) > 0 {
		if err := yaml.Unmarshal(yamlBlock, &data); err != nil {
			return nil, body, true, err
		}
		if data == nil {
			data = map[string]any{}
		}
	}
	return data, body, true, nil
}

// Document is a source file with front matter: a page, post or collection item.
type Document struct {
	relPath    string         // path relative to the site source
	data       map[string]any // front-matter keys
	body       string         // raw content after front matter
	content    string         // rendered content (Liquid + converter), pre-layout
	output     string         // final output after layouts
	url        string         // site-absolute URL (leading slash, no baseurl)
	ext        string         // source extension including dot, e.g. ".md"
	outputExt  string         // output extension, e.g. ".html"
	collection string         // "posts", "" for a page, or a custom collection label
	date       time.Time
	hasDate    bool
	categories []string
	tags       []string
	slug       string
}

// isMarkdown reports whether the document is converted through the Markdown
// pipeline (based on the configured markdown extensions).
func (d *Document) isMarkdown(cfg Config) bool {
	exts := strings.Split(cfg.str("markdown_ext"), ",")
	e := strings.TrimPrefix(strings.ToLower(d.ext), ".")
	for _, x := range exts {
		if strings.TrimSpace(x) == e {
			return true
		}
	}
	return false
}

// toLiquid builds the map exposed to templates as `page` (or as an element of
// `site.posts`, `site.<collection>`, …). The returned map is stored on the
// document set so its "content" entry is updated in place as rendering proceeds.
func (d *Document) toLiquid() map[string]any {
	m := map[string]any{}
	for k, v := range d.data {
		m[k] = v
	}
	m["url"] = d.url
	m["content"] = d.content
	m["path"] = d.relPath
	m["id"] = strings.TrimSuffix(d.url, d.outputExt)
	m["slug"] = d.slug
	m["ext"] = d.outputExt
	m["categories"] = toAnySlice(d.categories)
	m["tags"] = toAnySlice(d.tags)
	m["collection"] = d.collection
	if d.hasDate {
		m["date"] = d.date
	}
	// Jekyll derives a title from the slug only for collection documents (posts
	// and custom collections, via Jekyll::Document); regular pages (Jekyll::Page)
	// are left untitled so an untitled page renders an empty <title>/heading and
	// is skipped by title-filtered navigation.
	if _, ok := m["title"]; !ok && d.collection != "" {
		m["title"] = titleize(d.slug)
	}
	return m
}

func toAnySlice(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}

// titleize converts a slug to a human title ("hello-world" -> "Hello world"),
// matching Jekyll's Utils.titleize_slug for a missing title.
func titleize(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
