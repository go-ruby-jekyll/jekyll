// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import "strings"

// SassConverter is the pluggable converter for .scss/.sass documents. It is a
// NAMED, honestly-incomplete stub: a pure-Go Sass compiler (go-ruby-sass) does
// not exist yet, so this converter passes the stylesheet source through
// unchanged and records a loud warning rather than silently pretending to
// compile. The output extension is still mapped to .css so URLs match Jekyll.
type SassConverter struct{}

// matches reports whether the source extension is handled by this converter.
func (SassConverter) matches(ext string) bool {
	e := strings.ToLower(ext)
	return e == ".scss" || e == ".sass"
}

// convert returns the source unchanged (uncompiled). The caller is responsible
// for surfacing the warning returned by warning().
func (SassConverter) convert(src string) string { return src }

// warning is the message emitted when a Sass document is encountered.
func (SassConverter) warning(relPath string) string {
	return "Sass/SCSS is not compiled (go-ruby-sass is absent): " + relPath +
		" was copied through uncompiled."
}
