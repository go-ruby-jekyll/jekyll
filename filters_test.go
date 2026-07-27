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
