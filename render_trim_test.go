// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIncludeTrimMarkers verifies that Liquid whitespace-control markers on the
// include tag ({%- include -%}) collapse the surrounding whitespace, while plain
// markers preserve it. Minima uses trim markers on every include.
func TestIncludeTrimMarkers(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	writeTest(t, src, "_config.yml", "title: T\n")
	writeTest(t, src, "_includes/frag.html", "X")
	// {%- ... -%} trims the newline+indent on both sides -> "AXB".
	writeTest(t, src, "trim.html", "---\ntitle: A\n---\nA\n  {%- include frag.html -%}\n  B\n")
	// Plain {% include %} keeps the surrounding newlines -> "A\nX\nB".
	writeTest(t, src, "plain.html", "---\ntitle: B\n---\nA\n{% include frag.html %}\nB\n")
	// Left-only and right-only trims.
	writeTest(t, src, "left.html", "---\ntitle: C\n---\nA\n{%- include frag.html %}\nB\n")
	writeTest(t, src, "right.html", "---\ntitle: D\n---\nA\n{% include frag.html -%}\nB\n")
	buildIntoDst(t, src, dst, map[string]any{})

	for _, tc := range []struct{ file, want string }{
		{"trim.html", "AXB"},
		{"plain.html", "A\nX\nB"},
		{"left.html", "AX\nB"},
		{"right.html", "A\nXB"},
	} {
		out, err := os.ReadFile(filepath.Join(dst, tc.file))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), tc.want) {
			t.Errorf("%s: want %q in %q", tc.file, tc.want, out)
		}
	}
}
