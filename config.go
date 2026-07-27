// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the resolved site configuration: the defaults, deep-merged with the
// user's _config.yml file(s), then overlaid with command-line overrides. Keys
// are surfaced to templates under the `site` object.
type Config map[string]any

// defaultConfig returns Jekyll's built-in configuration defaults (the subset
// that affects a CGO-free build).
func defaultConfig() Config {
	return Config{
		"source":            ".",
		"destination":       "./_site",
		"collections_dir":   "",
		"plugins_dir":       "_plugins",
		"layouts_dir":       "_layouts",
		"includes_dir":      "_includes",
		"data_dir":          "_data",
		"permalink":         "date",
		"markdown":          "kramdown",
		"markdown_ext":      "markdown,mkdown,mkdn,mkd,md",
		"excerpt_separator": "\n\n",
		"keep_files":        []any{".git", ".svn"},
		"encoding":          "utf-8",
		"baseurl":           "",
		"url":               "",
		"title":             nil,
		"future":            false,
		"unpublished":       false,
		"show_drafts":       nil,
		"limit_posts":       0,
		"safe":              false,
		"include":           []any{".htaccess"},
		"exclude": []any{
			".sass-cache", ".jekyll-cache", "gemfiles", "Gemfile", "Gemfile.lock",
			"node_modules", "vendor/bundle/", "vendor/cache/", "vendor/gems/", "vendor/ruby/",
		},
		"collections": map[string]any{
			"posts": map[string]any{"output": true},
		},
		"defaults": []any{},
	}
}

// LoadConfig loads and merges configuration. source is the site root; when
// configFiles is empty, _config.yml (and _config.toml is *not* supported —
// residual) in source is used if present.
func LoadConfig(source string, configFiles []string) (Config, error) {
	cfg := defaultConfig()
	if len(configFiles) == 0 {
		if _, err := os.Stat(filepath.Join(source, "_config.yml")); err == nil {
			configFiles = []string{"_config.yml"}
		}
	}
	for _, f := range configFiles {
		if !filepath.IsAbs(f) {
			f = filepath.Join(source, f)
		}
		raw, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var m map[string]any
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		mergeConfig(cfg, m)
	}
	return cfg, nil
}

// mergeConfig deep-merges src into dst for the nested maps Jekyll treats
// specially (collections), and shallow-overrides everything else.
func mergeConfig(dst Config, src map[string]any) {
	for k, v := range src {
		if k == "collections" {
			dst[k] = mergeCollections(dst[k], v)
			continue
		}
		dst[k] = v
	}
}

// mergeCollections normalises the two accepted collection shapes (a list of
// names, or a map of name->settings) and merges them over the defaults.
func mergeCollections(existing, incoming any) any {
	out := map[string]any{}
	if em, ok := existing.(map[string]any); ok {
		for k, v := range em {
			out[k] = v
		}
	}
	switch c := incoming.(type) {
	case map[string]any:
		for k, v := range c {
			out[k] = v
		}
	case []any:
		for _, name := range c {
			if s, ok := name.(string); ok {
				if _, exists := out[s]; !exists {
					out[s] = map[string]any{}
				}
			}
		}
	}
	return out
}

// str returns a string config value or "".
func (c Config) str(key string) string {
	if v, ok := c[key].(string); ok {
		return v
	}
	return ""
}

// boolOpt returns a bool config value or false.
func (c Config) boolOpt(key string) bool {
	if v, ok := c[key].(bool); ok {
		return v
	}
	return false
}
