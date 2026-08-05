// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"testing"
	"time"
)

func TestJekyllFilters(t *testing.T) {
	cfg := Config{"baseurl": "/blog", "url": "https://example.com", "markdown": "kramdown"}
	f := jekyllFilters(cfg)
	call := func(name string, in any, args ...any) any {
		v, err := f[name](in, args)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return v
	}
	tm := time.Date(2023, 1, 15, 9, 30, 0, 0, time.UTC)
	checks := []struct {
		name string
		got  any
		want any
	}{
		{"relative_url", call("relative_url", "/x.html"), "/blog/x.html"},
		{"relative_url-slash", call("relative_url", "/"), "/blog/"},
		{"relative_url-abs", call("relative_url", "http://a/b"), "http://a/b"},
		{"absolute_url", call("absolute_url", "/x"), "https://example.com/blog/x"},
		{"absolute_url-abs", call("absolute_url", "http://a"), "http://a"},
		{"date_to_string", call("date_to_string", tm), "15 Jan 2023"},
		{"date_to_long_string", call("date_to_long_string", tm), "15 January 2023"},
		{"date_to_xmlschema", call("date_to_xmlschema", tm), "2023-01-15T09:30:00Z"},
		{"xml_escape", call("xml_escape", `a<b>&"'`), "a&lt;b&gt;&amp;&quot;&#39;"},
		{"slugify", call("slugify", "Hello World! Foo"), "hello-world-foo"},
		{"slugify-none", call("slugify", "Hello World", "none"), "hello world"},
		{"slugify-raw", call("slugify", "a  b", "raw"), "a-b"},
		{"number_of_words", call("number_of_words", "one two three"), 3},
		{"normalize_whitespace", call("normalize_whitespace", "a   b\n c"), "a b c"},
		{"markdownify", call("markdownify", "**hi**"), "<p><strong>hi</strong></p>"},
		{"smartify", call("smartify", "hi"), "hi"},
		{"array_to_sentence_string", call("array_to_sentence_string", []any{"a", "b", "c"}), "a, b, and c"},
		{"a2s-two", call("array_to_sentence_string", []any{"a", "b"}), "a and b"},
		{"a2s-one", call("array_to_sentence_string", []any{"a"}), "a"},
		{"a2s-zero", call("array_to_sentence_string", []any{}), ""},
		{"jsonify", call("jsonify", map[string]any{"k": "v"}), `{"k":"v"}`},
		{"cgi_escape", call("cgi_escape", "a b&c"), "a+b%26c"},
		{"date_to_rfc822", call("date_to_rfc822", tm), "Sun, 15 Jan 2023 09:30:00 +0000"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
	// uri_escape returns an escaped path
	if _, err := f["uri_escape"]("/a b", nil); err != nil {
		t.Fatal(err)
	}
	// jsonify with a time and slice
	if _, err := f["jsonify"]([]any{tm, 1}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestFilterHelpersOnStrings(t *testing.T) {
	// timeFilter falls back to string passthrough on non-time input.
	if got := toS(nil); got != "" {
		t.Fatalf("toS(nil)=%q", got)
	}
	if _, ok := toTime("2023-01-15"); !ok {
		t.Fatal("toTime should parse date")
	}
	if _, ok := toTime(42); ok {
		t.Fatal("toTime(int) should fail")
	}
	f := jekyllFilters(Config{})
	if v, _ := f["date_to_string"]("not-a-date", nil); v != "not-a-date" {
		t.Fatalf("date passthrough got %v", v)
	}
	if v, _ := f["date_to_xmlschema"]("not-a-date", nil); v != "not-a-date" {
		t.Fatalf("xmlschema passthrough got %v", v)
	}
}

func TestToStringSliceAndStripP(t *testing.T) {
	if got := toStringSlice([]string{"a"}); got[0] != "a" {
		t.Fatal("toStringSlice []string")
	}
	if got := toStringSlice(3); got != nil {
		t.Fatal("toStringSlice non-slice should be nil")
	}
	if got := stripP("<p>x</p>"); got != "x" {
		t.Fatalf("stripP=%q", got)
	}
}

// TestTimezone verifies faithful `timezone:` handling: when set, date filters
// render in that IANA zone with DST honored; when unset, each date keeps its
// authored offset (no localization to the build host).
func TestTimezone(t *testing.T) {
	winter := time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC) // no DST
	summer := time.Date(2023, 7, 15, 12, 0, 0, 0, time.UTC) // DST
	xs := func(cfg Config, in any) string {
		v, err := jekyllFilters(cfg)["date_to_xmlschema"](in, nil)
		if err != nil {
			t.Fatal(err)
		}
		return v.(string)
	}
	cases := []struct {
		name string
		cfg  Config
		in   any
		want string
	}{
		{"paris-winter", Config{"timezone": "Europe/Paris"}, winter, "2023-01-15T13:00:00+01:00"},
		{"paris-summer-dst", Config{"timezone": "Europe/Paris"}, summer, "2023-07-15T14:00:00+02:00"},
		{"ny-winter", Config{"timezone": "America/New_York"}, winter, "2023-01-15T07:00:00-05:00"},
		{"ny-summer-dst", Config{"timezone": "America/New_York"}, summer, "2023-07-15T08:00:00-04:00"},
		{"unset-keeps-utc", Config{}, winter, "2023-01-15T12:00:00Z"},
		{"unset-keeps-offset", Config{}, "2023-01-15T09:30:00+05:00", "2023-01-15T09:30:00+05:00"},
		{"invalid-zone-falls-back", Config{"timezone": "Not/AZone"}, winter, "2023-01-15T12:00:00Z"},
		{"zoneless-string-in-zone", Config{"timezone": "Europe/Paris"}, "2023-07-15 00:00:00", "2023-07-15T00:00:00+02:00"},
		{"zoneless-string-unset-utc", Config{}, "2023-07-15", "2023-07-15T00:00:00Z"},
	}
	for _, c := range cases {
		if got := xs(c.cfg, c.in); got != c.want {
			t.Errorf("%s: date_to_xmlschema = %q, want %q", c.name, got, c.want)
		}
	}
	// siteLocation: unset -> nil (no conversion); set -> the zone.
	if siteLocation(Config{}) != nil {
		t.Error("siteLocation(unset) should be nil")
	}
	if loc := siteLocation(Config{"timezone": "Europe/Paris"}); loc == nil || loc.String() != "Europe/Paris" {
		t.Errorf("siteLocation(Paris) = %v", loc)
	}
	// feed xmlSchemaDate honors the same zone (nil = keep offset).
	if got := xmlSchemaDate(summer, siteLocation(Config{"timezone": "America/New_York"})); got != "2023-07-15T08:00:00-04:00" {
		t.Errorf("xmlSchemaDate ny-summer = %q", got)
	}
	if got := xmlSchemaDate("2023-01-15T09:30:00Z", nil); got != "2023-01-15T09:30:00Z" {
		t.Errorf("xmlSchemaDate keep-Z = %q", got)
	}
}
