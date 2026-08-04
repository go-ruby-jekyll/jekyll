// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Build reads, renders and writes the whole site to the destination.
func (s *Site) Build() error {
	if err := s.Read(); err != nil {
		return err
	}
	s.process()
	if err := s.render(); err != nil {
		return err
	}
	s.runGenerators()
	return s.write()
}

// process sorts and filters the documents, then assembles the payload exposed to
// templates as `site`.
func (s *Site) process() {
	// Newest-first posts.
	sort.SliceStable(s.posts, func(i, j int) bool {
		return s.posts[i].date.After(s.posts[j].date)
	})
	s.posts = s.filterPosts(s.posts)
	s.collections["posts"] = s.posts

	// Collection docs sorted by date then slug.
	for label, docs := range s.collections {
		if label == "posts" {
			continue
		}
		docs = s.filterPublished(docs)
		sort.SliceStable(docs, func(i, j int) bool {
			if docs[i].hasDate || docs[j].hasDate {
				return docs[i].date.Before(docs[j].date)
			}
			return docs[i].slug < docs[j].slug
		})
		s.collections[label] = docs
	}
	s.pages = s.filterPublished(s.pages)

	s.buildPayload()
}

func (s *Site) filterPosts(posts []*Document) []*Document {
	future := s.Config.boolOpt("future")
	posts = s.filterPublished(posts)
	var out []*Document
	for _, p := range posts {
		if !future && p.hasDate && p.date.After(s.Time) {
			continue
		}
		out = append(out, p)
	}
	if lim, ok := s.Config["limit_posts"].(int); ok && lim > 0 && lim < len(out) {
		out = out[:lim]
	}
	return out
}

func (s *Site) filterPublished(docs []*Document) []*Document {
	unpublished := s.Config.boolOpt("unpublished")
	var out []*Document
	for _, d := range docs {
		if !unpublished {
			if pub, ok := d.data["published"].(bool); ok && !pub {
				continue
			}
		}
		out = append(out, d)
	}
	return out
}

func (s *Site) docMap(d *Document) map[string]any {
	if m, ok := s.docMaps[d]; ok {
		return m
	}
	m := d.toLiquid()
	s.docMaps[d] = m
	return m
}

func mapsFor(s *Site, docs []*Document) []any {
	out := make([]any, len(docs))
	for i, d := range docs {
		out[i] = s.docMap(d)
	}
	return out
}

func (s *Site) buildPayload() {
	site := map[string]any{}
	for k, v := range s.Config {
		site[k] = v
	}
	site["posts"] = mapsFor(s, s.posts)
	site["pages"] = mapsFor(s, s.pages)
	site["data"] = s.data
	site["time"] = s.Time

	var documents []any
	for label, docs := range s.collections {
		ms := mapsFor(s, docs)
		site[label] = ms
		documents = append(documents, ms...)
	}
	site["documents"] = documents

	// categories / tags groupings over posts.
	cats := map[string][]any{}
	tags := map[string][]any{}
	for _, p := range s.posts {
		m := s.docMap(p)
		for _, c := range p.categories {
			cats[c] = append(cats[c], m)
		}
		for _, t := range p.tags {
			tags[t] = append(tags[t], m)
		}
	}
	site["categories"] = toAnyMap(cats)
	site["tags"] = toAnyMap(tags)

	s.sitePayload = site
}

func toAnyMap(m map[string][]any) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// render renders every document's content and wraps it in its layout chain.
func (s *Site) render() error {
	var all []*Document
	all = append(all, s.posts...)
	for _, label := range s.collectionLabels() {
		if s.collectionOutput(label) {
			all = append(all, s.collections[label]...)
		}
	}
	all = append(all, s.pages...)

	for _, d := range all {
		pageMap := s.docMap(d)
		assigns := s.baseAssigns(pageMap)
		rendered, err := s.r.render(d.body, assigns)
		if err != nil {
			return err
		}
		if d.isMarkdown(s.Config) {
			rendered = convertMarkdown(rendered, s.Config)
		} else if (SassConverter{}).matches(d.ext) {
			css, err := newSassConverter(s).convert(rendered, d.ext)
			if err != nil {
				return err
			}
			rendered = css
		}
		d.content = rendered
		pageMap["content"] = rendered
		if d.collection != "" {
			ex, err := s.excerptFor(d, pageMap)
			if err != nil {
				return err
			}
			pageMap["excerpt"] = ex
		}
	}
	for _, d := range all {
		out, err := s.applyLayouts(d)
		if err != nil {
			return err
		}
		d.output = out
	}
	s.renderList = all
	return nil
}

func (s *Site) applyLayouts(d *Document) (string, error) {
	content := d.content
	pageMap := s.docMap(d)
	layoutName, _ := d.data["layout"].(string)
	seen := map[string]bool{}
	for layoutName != "" && !seen[layoutName] {
		seen[layoutName] = true
		lsrc, ldata, err := s.loadLayout(layoutName)
		if err != nil {
			return content, err
		}
		assigns := s.baseAssigns(pageMap)
		assigns["content"] = content
		assigns["layout"] = ldata
		content, err = s.r.render(lsrc, assigns)
		if err != nil {
			return content, err
		}
		layoutName, _ = ldata["layout"].(string)
	}
	return content, nil
}

func (s *Site) loadLayout(name string) (string, map[string]any, error) {
	dir := s.Config.str("layouts_dir")
	if dir == "" {
		dir = "_layouts"
	}
	// Site _layouts first, then the theme's _layouts (site overrides theme).
	roots := []string{filepath.Join(s.Source, dir)}
	if td := s.themeDir("_layouts"); td != "" {
		roots = append(roots, td)
	}
	for _, root := range roots {
		for _, ext := range []string{".html", ".md", ".markdown", ""} {
			p := filepath.Join(root, name+ext)
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			data, body, _, ferr := parseFrontMatter(raw)
			if ferr != nil {
				data = map[string]any{}
				body = string(raw)
			}
			if data == nil {
				data = map[string]any{}
			}
			return body, data, nil
		}
	}
	return "", nil, &parseErr{"Layout '" + name + "' requested but not found."}
}

// write flushes rendered documents and static files to the destination.
func (s *Site) write() error {
	for _, d := range s.renderList {
		if err := writeFile(filepath.Join(s.Dest, urlToOutputPath(d.url)), []byte(d.output)); err != nil {
			return err
		}
	}
	for _, f := range s.staticFiles {
		if err := copyFile(f.absPath, filepath.Join(s.Dest, f.relPath)); err != nil {
			return err
		}
	}
	for _, g := range s.generated {
		if err := writeFile(filepath.Join(s.Dest, urlToOutputPath(g.url)), []byte(g.content)); err != nil {
			return err
		}
	}
	return nil
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// keepFilePatterns returns the configured keep_files list.
func (s *Site) keepFilePatterns() []string {
	var out []string
	if kf, ok := s.Config["keep_files"].([]any); ok {
		for _, k := range kf {
			if ks, ok := k.(string); ok {
				out = append(out, ks)
			}
		}
	}
	return out
}

// osReadDir and osRemoveAll are indirections over the filesystem so the error
// branches of clean can be exercised deterministically on every OS via a test
// seam (a read-only directory or a file-as-destination behaves differently
// across platforms, so injection is the only portable way to reach them).
var (
	osReadDir   = os.ReadDir
	osRemoveAll = os.RemoveAll
	osReadFile  = os.ReadFile
)

// clean removes the destination, preserving keep_files entries.
func (s *Site) clean() error {
	keep := s.keepFilePatterns()
	entries, err := osReadDir(s.Dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if kept(e.Name(), keep) {
			continue
		}
		if err := osRemoveAll(filepath.Join(s.Dest, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func kept(name string, keep []string) bool {
	for _, k := range keep {
		if name == k || strings.HasPrefix(name, k) {
			return true
		}
	}
	return false
}
