// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// jekyllVersion is the upstream Jekyll release whose behaviour and output this
// project reimplements. It is surfaced to templates as `jekyll.version` and in
// the generator tags emitted by jekyll-feed / jekyll-seo-tag, so the generated
// markup byte-matches that reference release.
const jekyllVersion = "4.4.1"

// activePlugins returns the set of enabled plugins: the names listed under
// `plugins:` in _config.yml, unioned with the Jekyll plugins the active theme
// gem declares as runtime dependencies (Jekyll auto-requires a theme's plugin
// dependencies, which is how `minima` pulls in jekyll-feed and jekyll-seo-tag).
func (s *Site) activePlugins() map[string]bool {
	set := map[string]bool{}
	for _, p := range stringList(s.Config["plugins"]) {
		set[p] = true
	}
	for _, p := range themeGemDeps(s.themeRoot) {
		set[p] = true
	}
	return set
}

// pluginActive reports whether the named plugin is enabled for this build.
func (s *Site) pluginActive(name string) bool {
	return s.plugins[name]
}

var runtimeDepRe = regexp.MustCompile(`add_runtime_dependency\s*[( ]\s*%q<([^>]+)>`)

// themeGemDeps reads the installed gemspec matching a theme gem directory and
// returns its `jekyll-*` runtime dependencies. The gemspec lives in the
// `specifications/` sibling of the gem's `gems/` directory. An unresolved theme,
// or a missing/unreadable gemspec, yields no dependencies.
func themeGemDeps(themeRoot string) []string {
	if themeRoot == "" {
		return nil
	}
	base := filepath.Base(themeRoot)   // e.g. "minima-2.5.2"
	gemsDir := filepath.Dir(themeRoot) // .../gems
	specPath := filepath.Join(filepath.Dir(gemsDir), "specifications", base+".gemspec")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range runtimeDepRe.FindAllStringSubmatch(string(raw), -1) {
		name := m[1]
		if strings.HasPrefix(name, "jekyll-") && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// baseAssigns builds the common Liquid scope (`site`, `page`, `jekyll`) shared by
// content, layout and generator rendering.
func (s *Site) baseAssigns(pageMap map[string]any) map[string]any {
	return map[string]any{
		"site":   s.sitePayload,
		"page":   pageMap,
		"jekyll": jekyllPayload(),
	}
}

// jekyllPayload mirrors Jekyll's `jekyll` Liquid object (version + environment).
// The environment follows JEKYLL_ENV, defaulting to "development" as upstream.
func jekyllPayload() map[string]any {
	env := os.Getenv("JEKYLL_ENV")
	if env == "" {
		env = "development"
	}
	return map[string]any{"version": jekyllVersion, "environment": env}
}
