// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-ruby-sass/sass"
)

// SassConverter is the pluggable converter for .scss/.sass documents. It mirrors
// jekyll-sass-converter: front-matter-carrying stylesheets under the site are
// compiled to CSS through github.com/go-ruby-sass/sass (a pure-Go, CGO-free
// dart-sass-compatible engine over go-scss/scss). Partials are resolved from the
// configured sass_dir (default _sass) plus any user load_paths.
//
// Source maps are not emitted (a named go-scss residual); accordingly the
// converter treats jekyll's `sass.sourcemap` as off, matching the "common
// surface" of dart-sass output.
type SassConverter struct {
	// source is the absolute site source root.
	source string
	// loadPaths is the ordered @use/@import search path (user load_paths first,
	// then sass_dir), each made absolute against the source root.
	loadPaths []string
	// style is the output style (expanded by default, compressed when configured).
	style sass.Style
}

// newSassConverter derives a converter from the site configuration, honouring
// the `sass` map's `sass_dir`, `style` and `load_paths` keys exactly as
// jekyll-sass-converter does (defaults: sass_dir=_sass, style=expanded).
func newSassConverter(s *Site) SassConverter {
	c := SassConverter{source: s.Source, style: sass.StyleExpanded}
	sassDir := "_sass"
	var userPaths []string
	if sc, ok := s.Config["sass"].(map[string]any); ok {
		if v, ok := sc["sass_dir"].(string); ok && v != "" {
			sassDir = v
		}
		if v, ok := sc["style"].(string); ok {
			if strings.TrimPrefix(v, ":") == "compressed" {
				c.style = sass.StyleCompressed
			}
		}
		userPaths = sassStringList(sc["load_paths"])
	}
	for _, p := range userPaths {
		c.loadPaths = append(c.loadPaths, c.abs(p))
	}
	c.loadPaths = append(c.loadPaths, c.abs(sassDir))
	return c
}

// abs resolves p against the site source root when it is not already absolute.
func (c SassConverter) abs(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.source, p)
}

// sassStringList normalises the YAML shapes a load_paths value can take.
func sassStringList(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return t
	}
	return nil
}

// matches reports whether the source extension is handled by this converter.
func (SassConverter) matches(ext string) bool {
	e := strings.ToLower(ext)
	return e == ".scss" || e == ".sass"
}

// convert compiles the (already Liquid-rendered) stylesheet source to CSS. The
// syntax is inferred from ext (.sass is the indented syntax, .scss is SCSS), and
// only existing load-path directories are passed to the engine, matching
// jekyll-sass-converter's directory filtering.
func (c SassConverter) convert(src, ext string) (string, error) {
	syntax := sass.SyntaxSCSS
	if strings.ToLower(ext) == ".sass" {
		syntax = sass.SyntaxIndented
	}
	var lp []string
	for _, p := range c.loadPaths {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			lp = append(lp, p)
		}
	}
	res, err := sass.CompileString(src, &sass.Options{
		Syntax:    syntax,
		Style:     c.style,
		LoadPaths: lp,
	})
	if err != nil {
		return "", err
	}
	// sass-embedded's Sass.compile_string (which jekyll-sass-converter calls)
	// returns CSS with no trailing newline, whereas the go-scss engine appends
	// one (a named residual). Trim a single trailing newline so the emitted CSS
	// byte-matches the gem's output.
	return strings.TrimSuffix(res.CSS, "\n"), nil
}
