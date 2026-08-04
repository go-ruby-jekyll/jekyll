// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-ruby-liquid/liquid"
	"gopkg.in/yaml.v3"
)

// staticFile is a source file copied verbatim to the destination.
type staticFile struct {
	relPath string
	absPath string
}

// Site is a single Jekyll site: its resolved configuration and, after Read, the
// full set of documents, collections, data and static files.
type Site struct {
	Config Config
	Source string
	Dest   string
	Time   time.Time

	// themeRoot is the absolute path of the resolved theme gem (via `theme:` in
	// _config.yml), or "" when no theme is configured/found. Theme _layouts,
	// _includes, _sass and assets are layered under the site's own files.
	themeRoot string

	filters map[string]liquid.Filter
	r       *renderer

	posts       []*Document
	pages       []*Document
	collections map[string][]*Document // label -> docs, includes "posts"
	staticFiles []staticFile
	data        map[string]any

	docMaps       map[*Document]map[string]any
	urlByPath     map[string]string
	postURLByName map[string]string
	sitePayload   map[string]any
	renderList    []*Document
}

// NewSite builds a Site from a resolved configuration.
func NewSite(cfg Config) *Site {
	src, _ := filepath.Abs(cfg.str("source"))
	dst := cfg.str("destination")
	if !filepath.IsAbs(dst) {
		dst = filepath.Join(src, strings.TrimPrefix(dst, "./"))
	}
	s := &Site{
		Config:        cfg,
		Source:        src,
		Dest:          dst,
		Time:          time.Now(),
		collections:   map[string][]*Document{},
		data:          map[string]any{},
		docMaps:       map[*Document]map[string]any{},
		urlByPath:     map[string]string{},
		postURLByName: map[string]string{},
	}
	if name := cfg.str("theme"); name != "" {
		s.themeRoot = resolveTheme(name).Root
	}
	s.filters = jekyllFilters(cfg)
	s.r = &renderer{site: s}
	return s
}

// collectionLabels returns the configured collection names (excluding posts,
// which is handled specially).
func (s *Site) collectionLabels() []string {
	var labels []string
	if cm, ok := s.Config["collections"].(map[string]any); ok {
		for name := range cm {
			if name != "posts" {
				labels = append(labels, name)
			}
		}
	}
	sort.Strings(labels)
	return labels
}

func (s *Site) collectionOutput(label string) bool {
	if cm, ok := s.Config["collections"].(map[string]any); ok {
		if c, ok := cm[label].(map[string]any); ok {
			if v, ok := c["output"].(bool); ok {
				return v
			}
		}
	}
	return false
}

func (s *Site) collectionPermalink(label string) string {
	if cm, ok := s.Config["collections"].(map[string]any); ok {
		if c, ok := cm[label].(map[string]any); ok {
			if v, ok := c["permalink"].(string); ok {
				return v
			}
		}
	}
	return "/:collection/:path/"
}

// Read walks the source tree and classifies every file.
func (s *Site) Read() error {
	if err := s.readData(); err != nil {
		return err
	}
	labels := map[string]bool{}
	for _, l := range s.collectionLabels() {
		labels[l] = true
	}
	drafts := s.Config.boolOpt("show_drafts")

	if err := s.walkSource(labels, drafts); err != nil {
		return err
	}
	return s.readThemeAssets()
}

func (s *Site) walkSource(labels map[string]bool, drafts bool) error {
	return filepath.WalkDir(s.Source, func(path string, dEntry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(s.Source, path)
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if dEntry.IsDir() {
			if s.skipDir(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		top := strings.SplitN(rel, "/", 2)[0]
		switch {
		case top == "_posts":
			return s.readPost(path, rel, false)
		case top == "_drafts":
			if drafts {
				return s.readPost(path, rel, true)
			}
			return nil
		case strings.HasPrefix(top, "_") && labels[strings.TrimPrefix(top, "_")]:
			return s.readCollectionDoc(path, rel, strings.TrimPrefix(top, "_"))
		case s.excluded(rel) || strings.HasPrefix(top, "_") || strings.HasPrefix(top, "."):
			return nil
		default:
			return s.readPageOrStatic(path, rel)
		}
	})
}

// readThemeAssets layers the theme gem's `assets/` tree under the site: every
// theme asset is read as a page (when it carries front matter) or static file,
// unless the site already provides a file at the same relative path (site files
// override theme files). Only `assets/` produces output; the theme's _layouts,
// _includes and _sass are consumed on demand during rendering.
func (s *Site) readThemeAssets() error {
	root := s.themeDir("assets")
	if root == "" {
		return nil
	}
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		sub, _ := filepath.Rel(root, path)
		rel := filepath.ToSlash(filepath.Join("assets", sub))
		if s.hasSourceFile(rel) {
			return nil
		}
		return s.readPageOrStatic(path, rel)
	})
}

// hasSourceFile reports whether the site already declared a document or static
// file at rel (so a theme file at the same path must not shadow it).
func (s *Site) hasSourceFile(rel string) bool {
	if _, ok := s.urlByPath[rel]; ok {
		return true
	}
	for _, f := range s.staticFiles {
		if f.relPath == rel {
			return true
		}
	}
	return false
}

func (s *Site) skipDir(rel string) bool {
	top := strings.SplitN(rel, "/", 2)[0]
	if rel == filepath.ToSlash(mustRel(s.Source, s.Dest)) {
		return true
	}
	switch top {
	case "_layouts", "_includes", "_data", "_plugins", "_sass":
		return true
	}
	if s.excluded(rel) {
		return true
	}
	// Underscore/dot dirs that are not collections are skipped.
	if strings.HasPrefix(top, "_") {
		for _, l := range s.collectionLabels() {
			if top == "_"+l {
				return false
			}
		}
		if top == "_posts" || top == "_drafts" {
			return false
		}
		return true
	}
	if strings.HasPrefix(top, ".") {
		return true
	}
	return false
}

func mustRel(base, target string) string {
	r, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return r
}

func (s *Site) excluded(rel string) bool {
	ex, _ := s.Config["exclude"].([]any)
	for _, e := range ex {
		es, ok := e.(string)
		if !ok {
			continue
		}
		es = strings.TrimSuffix(filepath.ToSlash(es), "/")
		if rel == es || strings.HasPrefix(rel, es+"/") {
			return true
		}
	}
	return false
}

func (s *Site) readData() error {
	dir := s.Config.str("data_dir")
	if dir == "" {
		dir = "_data"
	}
	root := filepath.Join(s.Source, dir)
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		raw, err := osReadFile(path)
		if err != nil {
			return err
		}
		var v any
		if err := yaml.Unmarshal(raw, &v); err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		key := strings.TrimSuffix(filepath.ToSlash(rel), ext)
		s.data[key] = v
		return nil
	})
}

// buildDocument parses front matter and populates the common document fields.
func (s *Site) buildDocument(path, rel string) (*Document, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	if !hasFrontMatter(raw) {
		return nil, false, nil
	}
	data, body, _, err := parseFrontMatter(raw)
	if err != nil {
		if s.Config.boolOpt("strict_front_matter") {
			return nil, false, err
		}
		data, body = map[string]any{}, string(raw)
	}
	d := &Document{
		relPath: rel,
		data:    data,
		body:    body,
		ext:     filepath.Ext(rel),
	}
	d.outputExt = ".html"
	switch {
	case (SassConverter{}).matches(d.ext):
		d.outputExt = ".css"
	case !d.isMarkdown(s.Config) && d.ext != ".html":
		d.outputExt = d.ext
	}
	return d, true, nil
}

func (s *Site) readPost(path, rel string, draft bool) error {
	d, ok, err := s.buildDocument(path, rel)
	if err != nil || !ok {
		return err
	}
	d.collection = "posts"
	base := filepath.Base(rel)
	name := strings.TrimSuffix(base, d.ext)
	if !draft {
		// YYYY-MM-DD-title
		if len(name) > 11 && name[4] == '-' && name[7] == '-' && name[10] == '-' {
			if t, perr := time.Parse("2006-01-02", name[:10]); perr == nil {
				d.date, d.hasDate = t, true
			}
			d.slug = name[11:]
		} else {
			d.slug = name
		}
	} else {
		d.slug = name
		d.date, d.hasDate = s.Time, true
	}
	if fmDate, ok := d.data["date"]; ok {
		if t, ok := toTime(fmDate); ok {
			d.date, d.hasDate = t, true
		}
	}
	d.categories = extractCategories(d.data)
	d.tags = extractTags(d.data)
	// Determine permalink (front matter override or config style).
	pm := s.Config.str("permalink")
	if v, ok := d.data["permalink"].(string); ok {
		pm = v
	} else {
		pm = resolvePermalinkStyle(pm)
	}
	d.url = applyPermalink(pm, d, name)
	s.posts = append(s.posts, d)
	s.collections["posts"] = append(s.collections["posts"], d)
	s.urlByPath[rel] = d.url
	// post_url references a post by its filename without extension (date + slug).
	s.postURLByName[name] = d.url
	return nil
}

func (s *Site) readCollectionDoc(path, rel, label string) error {
	d, ok, err := s.buildDocument(path, rel)
	if err != nil || !ok {
		return err
	}
	d.collection = label
	collRel := strings.TrimPrefix(rel, "_"+label+"/")
	d.slug = strings.TrimSuffix(filepath.Base(collRel), d.ext)
	if fmDate, ok := d.data["date"]; ok {
		if t, ok := toTime(fmDate); ok {
			d.date, d.hasDate = t, true
		}
	}
	d.categories = extractCategories(d.data)
	d.tags = extractTags(d.data)
	pm := s.collectionPermalink(label)
	if v, ok := d.data["permalink"].(string); ok {
		pm = v
	}
	pm = strings.ReplaceAll(pm, ":collection", label)
	d.url = applyPermalink(pm, d, collRel)
	s.collections[label] = append(s.collections[label], d)
	s.urlByPath[rel] = d.url
	return nil
}

func (s *Site) readPageOrStatic(path, rel string) error {
	d, ok, err := s.buildDocument(path, rel)
	if err != nil {
		return err
	}
	if !ok {
		s.staticFiles = append(s.staticFiles, staticFile{relPath: rel, absPath: path})
		return nil
	}
	d.collection = ""
	d.slug = strings.TrimSuffix(filepath.Base(rel), d.ext)
	// Page URL: front-matter permalink, else path with output extension.
	if pm, ok := d.data["permalink"].(string); ok {
		d.url = applyPermalink(pm, d, rel)
	} else {
		d.url = defaultPageURL(rel, d.ext, d.outputExt)
	}
	s.pages = append(s.pages, d)
	s.urlByPath[rel] = d.url
	return nil
}

func extractCategories(data map[string]any) []string {
	out := stringList(data["categories"])
	if len(out) == 0 {
		out = stringList(data["category"])
	}
	return out
}

func extractTags(data map[string]any) []string {
	out := stringList(data["tags"])
	if len(out) == 0 {
		out = stringList(data["tag"])
	}
	return out
}

func stringList(v any) []string {
	switch t := v.(type) {
	case string:
		fields := strings.Fields(t)
		return fields
	case []any:
		var out []string
		for _, e := range t {
			out = append(out, strings.TrimSpace(toS(e)))
		}
		return out
	case []string:
		return t
	}
	return nil
}
