// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"fmt"
	"regexp"
	"strings"
)

// builtinPermalinks maps Jekyll's named permalink styles to their templates.
var builtinPermalinks = map[string]string{
	"date":     "/:categories/:year/:month/:day/:title:output_ext",
	"pretty":   "/:categories/:year/:month/:day/:title/",
	"ordinal":  "/:categories/:year/:y_day/:title:output_ext",
	"weekdate": "/:categories/:year/:week/:short_day/:title:output_ext",
	"none":     "/:categories/:title:output_ext",
}

// resolvePermalinkStyle expands a named style to its template, or returns the
// string unchanged when it is already a template.
func resolvePermalinkStyle(style string) string {
	if t, ok := builtinPermalinks[style]; ok {
		return t
	}
	return style
}

var multiSlash = regexp.MustCompile(`/{2,}`)

// applyPermalink substitutes the :placeholders in a permalink template for the
// given document and returns the site-absolute URL.
func applyPermalink(tmpl string, d *Document, collectionRelPath string) string {
	repl := map[string]string{
		":year":       fmt.Sprintf("%04d", d.date.Year()),
		":month":      fmt.Sprintf("%02d", int(d.date.Month())),
		":i_month":    fmt.Sprintf("%d", int(d.date.Month())),
		":day":        fmt.Sprintf("%02d", d.date.Day()),
		":i_day":      fmt.Sprintf("%d", d.date.Day()),
		":hour":       fmt.Sprintf("%02d", d.date.Hour()),
		":minute":     fmt.Sprintf("%02d", d.date.Minute()),
		":second":     fmt.Sprintf("%02d", d.date.Second()),
		":short_year": fmt.Sprintf("%02d", d.date.Year()%100),
		":y_day":      fmt.Sprintf("%03d", d.date.YearDay()),
		":title":      d.slug,
		":slug":       d.slug,
		":name":       d.slug,
		":categories": strings.Join(d.categories, "/"),
		":path":       strings.TrimSuffix(collectionRelPath, d.ext),
		":output_ext": d.outputExt,
	}
	// Longest keys first so :i_month is not clobbered by :month, etc.
	order := []string{
		":output_ext", ":short_year", ":categories", ":i_month", ":i_day",
		":minute", ":second", ":y_day", ":month", ":title", ":slug", ":name",
		":path", ":hour", ":year", ":day",
	}
	out := tmpl
	for _, k := range order {
		out = strings.ReplaceAll(out, k, repl[k])
	}
	if !strings.HasPrefix(out, "/") {
		out = "/" + out
	}
	out = multiSlash.ReplaceAllString(out, "/")
	return out
}

// urlToOutputPath converts a site-absolute URL to a destination-relative file
// path. A trailing slash yields index.html; a bare directory URL likewise.
func urlToOutputPath(url string) string {
	p := strings.TrimPrefix(url, "/")
	if p == "" || strings.HasSuffix(p, "/") {
		return p + "index.html"
	}
	return p
}
