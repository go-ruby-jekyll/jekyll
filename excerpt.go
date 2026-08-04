// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"regexp"
	"strings"
)

// linkRefDefRe matches a Markdown link-reference definition line
// (`[label]: url`), indented up to three spaces, exactly like Jekyll::Excerpt.
var linkRefDefRe = regexp.MustCompile(`(?m)^ {0,3}\[[^\]]+\]:.+$`)

// excerptFor computes a document's excerpt the way Jekyll::Excerpt does: it takes
// the content up to the excerpt separator (default "\n\n"), appends any
// link-reference definitions found in the remainder so excerpt-local references
// still resolve, then runs the same Liquid + Markdown pipeline as the body. When
// the separator is empty the whole body is used.
func (s *Site) excerptFor(d *Document, pageMap map[string]any) (string, error) {
	// The default configuration always provides excerpt_separator ("\n\n"); an
	// explicit "" disables splitting so the whole body becomes the excerpt.
	sep := s.Config.str("excerpt_separator")
	if v, ok := d.data["excerpt_separator"].(string); ok {
		sep = v
	}

	head := d.body
	if sep != "" {
		if idx := strings.Index(d.body, sep); idx >= 0 {
			head = d.body[:idx]
			tail := d.body[idx:]
			if defs := linkRefDefRe.FindAllString(tail, -1); len(defs) > 0 {
				head = head + "\n\n" + strings.Join(defs, "\n")
			}
		}
	}

	rendered, err := s.r.render(head, s.baseAssigns(pageMap))
	if err != nil {
		return "", err
	}
	if d.isMarkdown(s.Config) {
		rendered = convertMarkdown(rendered, s.Config)
	}
	return rendered, nil
}
