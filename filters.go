// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-kramdown/kramdown"
	"github.com/go-liquid/liquid"
)

// jekyllFilters returns the Jekyll-specific Liquid filter vocabulary bound to
// this build's configuration. They are registered with go-ruby-liquid through
// its custom-filter hook, so they work anywhere a filter can appear — including
// inside {% for %} loops, which is why they cannot be pre-expanded.
//
// String-valued filters (relative_url, date_to_*, slugify, …) are exact.
// Array/hash filters (where_exp, group_by) are best-effort; see README.
func jekyllFilters(cfg Config) map[string]liquid.Filter {
	baseurl := cfg.str("baseurl")
	siteURL := cfg.str("url")
	loc := siteLocation(cfg)
	f := map[string]liquid.Filter{}

	f["relative_url"] = func(in any, _ []any) (any, error) {
		return relativeURL(baseurl, toS(in)), nil
	}
	f["absolute_url"] = func(in any, _ []any) (any, error) {
		rel := relativeURL(baseurl, toS(in))
		if isAbsoluteURL(rel) {
			return rel, nil
		}
		return strings.TrimSuffix(siteURL, "/") + rel, nil
	}
	f["date_to_string"] = timeFilter("02 Jan 2006", loc)
	f["date_to_long_string"] = timeFilter("02 January 2006", loc)
	f["date_to_xmlschema"] = func(in any, _ []any) (any, error) {
		if t, ok := toTimeIn(in, loc); ok {
			return inLoc(t, loc).Format(time.RFC3339), nil
		}
		return toS(in), nil
	}
	f["date_to_rfc822"] = timeFilter("Mon, 02 Jan 2006 15:04:05 -0700", loc)
	f["xml_escape"] = func(in any, _ []any) (any, error) { return xmlEscape(toS(in)), nil }
	f["cgi_escape"] = func(in any, _ []any) (any, error) { return url.QueryEscape(toS(in)), nil }
	f["uri_escape"] = func(in any, _ []any) (any, error) {
		return (&url.URL{Path: toS(in)}).EscapedPath(), nil
	}
	f["slugify"] = func(in any, args []any) (any, error) {
		mode := "default"
		if len(args) > 0 {
			mode = toS(args[0])
		}
		return slugify(toS(in), mode), nil
	}
	f["number_of_words"] = func(in any, _ []any) (any, error) {
		return len(strings.Fields(toS(in))), nil
	}
	f["normalize_whitespace"] = func(in any, _ []any) (any, error) {
		return strings.Join(strings.Fields(toS(in)), " "), nil
	}
	f["markdownify"] = func(in any, _ []any) (any, error) {
		return strings.TrimRight(kramdown.ToHTML(toS(in), kramdownOptions(cfg)), "\n"), nil
	}
	f["smartify"] = func(in any, _ []any) (any, error) {
		return strings.TrimSpace(stripP(kramdown.ToHTML(toS(in), kramdownOptions(cfg)))), nil
	}
	f["jsonify"] = func(in any, _ []any) (any, error) {
		b, err := json.Marshal(jsonSafe(in))
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	f["array_to_sentence_string"] = func(in any, args []any) (any, error) {
		conn := "and"
		if len(args) > 0 {
			conn = toS(args[0])
		}
		return arrayToSentence(in, conn), nil
	}
	f["jekyll_where_exp_unsupported"] = nil // placeholder marker; see README residuals
	delete(f, "jekyll_where_exp_unsupported")
	return f
}

func timeFilter(layout string, loc *time.Location) liquid.Filter {
	return func(in any, _ []any) (any, error) {
		if t, ok := toTimeIn(in, loc); ok {
			return inLoc(t, loc).Format(layout), nil
		}
		return toS(in), nil
	}
}

// siteLocation returns the time.Location that Jekyll date filters render in, or
// nil when `timezone:` is unset. Jekyll sets ENV["TZ"] from
// site.config["timezone"] (an IANA zone name), so when it is set every date
// renders in that zone with DST honored — we mirror that by converting with
// .In(loc). When it is absent we return nil, meaning "no conversion": each date
// keeps the offset it was authored with (UTC for zone-less front-matter, or the
// explicit offset otherwise). Jekyll's own no-timezone behavior is machine-local;
// we deliberately do NOT localize to the build host's incidental zone, so a build
// is reproducible. Set `timezone:` explicitly to control the offset.
func siteLocation(cfg Config) *time.Location {
	if tz := cfg.str("timezone"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			return loc
		}
	}
	return nil
}

// inLoc renders t in loc, or leaves its offset untouched when loc is nil.
func inLoc(t time.Time, loc *time.Location) time.Time {
	if loc == nil {
		return t
	}
	return t.In(loc)
}

func toS(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if t, ok := v.(time.Time); ok {
		return t.Format("2006-01-02 15:04:05 -0700")
	}
	return fmt.Sprint(v)
}

func toTime(v any) (time.Time, bool) { return toTimeIn(v, time.UTC) }

// toTimeIn coerces a value to a time.Time, interpreting zone-less string forms
// in loc (matching Jekyll, which reads front-matter wall-clock times as being in
// the site timezone) while forms that carry an explicit offset keep their
// instant. A time.Time value is returned as-is; the caller converts with .In(loc)
// at format time.
func toTimeIn(v any, loc *time.Location) (time.Time, bool) {
	// When no site timezone is configured, zone-less forms parse as UTC (the
	// long-standing default), which .In(nil-loc) then leaves untouched.
	zoneless := loc
	if zoneless == nil {
		zoneless = time.UTC
	}
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		// Offset-bearing layouts: absolute instant, keep as parsed.
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05 -0700"} {
			if p, err := time.Parse(layout, t); err == nil {
				return p, true
			}
		}
		// Zone-less layouts: interpret the wall clock in the site zone (UTC when
		// unset), matching Jekyll reading front-matter times under ENV["TZ"].
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02"} {
			if p, err := time.ParseInLocation(layout, t, zoneless); err == nil {
				return p, true
			}
		}
	}
	return time.Time{}, false
}

// relativeURL prepends the (trailing-slash-stripped) baseurl to a site-absolute
// path and collapses duplicate slashes, matching Jekyll::Filters#relative_url.
func relativeURL(baseurl, input string) string {
	if isAbsoluteURL(input) {
		return input
	}
	b := strings.TrimSuffix(baseurl, "/")
	if input != "" && !strings.HasPrefix(input, "/") {
		input = "/" + input
	}
	out := b + input
	if !strings.HasPrefix(out, "/") {
		out = "/" + out
	}
	return multiSlash.ReplaceAllString(out, "/")
}

func isAbsoluteURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "//")
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s, mode string) string {
	low := strings.ToLower(s)
	switch mode {
	case "none":
		return low
	case "raw":
		return regexp.MustCompile(`\s+`).ReplaceAllString(low, "-")
	default: // "default", "pretty", "ascii", "latin"
		out := slugNonAlnum.ReplaceAllString(low, "-")
		return strings.Trim(out, "-")
	}
}

func stripP(html string) string {
	html = strings.TrimSpace(html)
	html = strings.TrimPrefix(html, "<p>")
	html = strings.TrimSuffix(html, "</p>")
	return html
}

func arrayToSentence(in any, conn string) string {
	items := toStringSlice(in)
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " " + conn + " " + items[1]
	default:
		return strings.Join(items[:len(items)-1], ", ") + ", " + conn + " " + items[len(items)-1]
	}
}

func toStringSlice(in any) []string {
	switch v := in.(type) {
	case []any:
		out := make([]string, len(v))
		for i, e := range v {
			out[i] = toS(e)
		}
		return out
	case []string:
		return v
	}
	return nil
}

// jsonSafe converts values (including time.Time) into JSON-encodable forms.
func jsonSafe(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.Format(time.RFC3339)
	case map[string]any:
		m := map[string]any{}
		for k, val := range t {
			m[k] = jsonSafe(val)
		}
		return m
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = jsonSafe(e)
		}
		return out
	}
	return v
}
