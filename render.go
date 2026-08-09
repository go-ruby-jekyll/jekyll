// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-liquid/liquid"
)

// renderer runs the Liquid pipeline for one site. Because go-ruby-liquid has no
// tag-registration hook (see README "Substrate gaps"), Jekyll's tags — include,
// highlight, link, post_url — are expanded by this layer before the template is
// handed to the engine; Jekyll's filters are injected through the engine's
// custom-filter hook.
type renderer struct {
	site *Site
}

const maxRenderDepth = 50

// render evaluates src against assigns, expanding Jekyll tags and applying the
// Jekyll filter vocabulary.
func (r *renderer) render(src string, assigns map[string]any) (string, error) {
	return r.renderDepth(src, assigns, 0)
}

func (r *renderer) renderDepth(src string, assigns map[string]any, depth int) (string, error) {
	if depth > maxRenderDepth {
		return "", fmt.Errorf("include depth exceeded")
	}
	var perr error
	src = r.expandIncludes(src, assigns, depth, &perr)
	if perr != nil {
		return "", perr
	}
	src = r.expandHighlights(src)
	src = r.expandLinks(src, assigns)
	src = r.expandFeedMeta(src)
	src = r.expandSeo(src, assigns)
	tpl, err := liquid.Parse(src, liquid.WithFilters(r.site.filters), liquid.WithErrorMode(liquid.Lax))
	if err != nil {
		return "", err
	}
	return tpl.Render(assigns)
}

// includeRe matches {% include file [k=v ...] %}, including Liquid's
// whitespace-control trim markers {%- ... -%} (minima puts them on every
// include). Groups: 1=leading whitespace, 2=left trim marker, 3=file,
// 4=params, 5=right trim marker, 6=trailing whitespace.
var includeRe = regexp.MustCompile(`(?s)(\s*)\{%(-?)\s*include\s+(\S+?)((?:\s+[^%]*?)?)\s*(-?)%\}(\s*)`)

// expandIncludes replaces {% include file [k=v ...] %} tags with the rendered
// contents of the include file, scoped with an `include` map of the parameters.
// A leading `{%-` trims whitespace before the tag; a trailing `-%}` trims
// whitespace after it, mirroring Liquid's whitespace control.
func (r *renderer) expandIncludes(src string, assigns map[string]any, depth int, perr *error) string {
	return includeRe.ReplaceAllStringFunc(src, func(m string) string {
		if *perr != nil {
			return ""
		}
		sub := includeRe.FindStringSubmatch(m)
		lead, ltrim, rtrim, trail := sub[1], sub[2], sub[5], sub[6]
		name := strings.Trim(sub[3], `"'`)
		params := parseIncludeParams(sub[4], assigns)
		body, err := r.readInclude(name)
		if err != nil {
			*perr = err
			return ""
		}
		scoped := map[string]any{}
		for k, v := range assigns {
			scoped[k] = v
		}
		scoped["include"] = params
		out, err := r.renderDepth(body, scoped, depth+1)
		if err != nil {
			*perr = err
			return ""
		}
		var b strings.Builder
		if ltrim != "-" {
			b.WriteString(lead)
		}
		b.WriteString(out)
		if rtrim != "-" {
			b.WriteString(trail)
		}
		return b.String()
	})
}

func (r *renderer) readInclude(name string) (string, error) {
	dir := r.site.Config.str("includes_dir")
	if dir == "" {
		dir = "_includes"
	}
	// Site _includes first, then the theme's _includes (site overrides theme).
	roots := []string{filepath.Join(r.site.Source, dir)}
	if td := r.site.themeDir("_includes"); td != "" {
		roots = append(roots, td)
	}
	for _, root := range roots {
		if b, err := os.ReadFile(filepath.Join(root, name)); err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("Included file '%s' not found", name)
}

var includeParamRe = regexp.MustCompile(`(\w+)=("[^"]*"|'[^']*'|\S+)`)

func parseIncludeParams(s string, assigns map[string]any) map[string]any {
	out := map[string]any{}
	for _, m := range includeParamRe.FindAllStringSubmatch(s, -1) {
		key, raw := m[1], m[2]
		switch {
		case strings.HasPrefix(raw, `"`) || strings.HasPrefix(raw, `'`):
			out[key] = raw[1 : len(raw)-1]
		default:
			if v, ok := lookupVar(assigns, raw); ok {
				out[key] = v
			} else {
				out[key] = raw
			}
		}
	}
	return out
}

// lookupVar resolves a dotted variable path against an assigns tree.
func lookupVar(assigns map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = assigns
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

var highlightRe = regexp.MustCompile(`(?s)\{%\s*highlight\s+(\w+)([^%]*)%\}(.*?)\{%\s*endhighlight\s*%\}`)

// expandHighlights renders {% highlight LANG %}...{% endhighlight %} blocks
// through go-ruby-rouge, matching Jekyll's figure wrapper.
func (r *renderer) expandHighlights(src string) string {
	return highlightRe.ReplaceAllStringFunc(src, func(m string) string {
		sub := highlightRe.FindStringSubmatch(m)
		lang := sub[1]
		code := strings.Trim(sub[3], "\n")
		return highlightFigure(code, lang)
	})
}

var linkRe = regexp.MustCompile(`\{%\s*(link|post_url)\s+([^\s%]+)\s*%\}`)

// expandLinks resolves {% link path %} and {% post_url name %} to URLs.
func (r *renderer) expandLinks(src string, _ map[string]any) string {
	return linkRe.ReplaceAllStringFunc(src, func(m string) string {
		sub := linkRe.FindStringSubmatch(m)
		kind, ref := sub[1], strings.Trim(sub[2], `"'`)
		if kind == "post_url" {
			if u, ok := r.site.postURLByName[ref]; ok {
				return relativeURL(r.site.Config.str("baseurl"), u)
			}
			return ""
		}
		if u, ok := r.site.urlByPath[ref]; ok {
			return relativeURL(r.site.Config.str("baseurl"), u)
		}
		return ""
	})
}

// atoiDefault parses n or returns def.
func atoiDefault(s string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return v
	}
	return def
}
