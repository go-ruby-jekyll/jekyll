// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"path/filepath"
	"time"
)

// scaffoldSite writes a minimal, buildable Jekyll site to path. When blank is
// true the content files are created empty (mirroring `jekyll new --blank`).
//
// Note: the reference `jekyll new` produces a minima-theme-based site that pulls
// its layouts from the gem; this scaffold is self-contained (layouts included in
// the tree) so the result builds without any Ruby gem.
func scaffoldSite(path string, blank bool) error {
	dirs := []string{"_posts", "_layouts", "_includes", "_data"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(path, d), 0o755); err != nil {
			return err
		}
	}
	files := map[string]string{
		"_config.yml": "title: Your awesome title\n" +
			"email: your-email@example.com\n" +
			"description: >-\n  Write an awesome description for your new site here.\n" +
			"baseurl: \"\"\nurl: \"\"\n",
		"_layouts/default.html": "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n" +
			"<title>{{ page.title }} · {{ site.title }}</title>\n</head>\n<body>\n" +
			"{% include header.html %}\n<main>{{ content }}</main>\n</body>\n</html>\n",
		"_layouts/post.html": "---\nlayout: default\n---\n<article>\n<h1>{{ page.title }}</h1>\n" +
			"<time>{{ page.date | date_to_string }}</time>\n{{ content }}\n</article>\n",
		"_includes/header.html": "<header><a href=\"{{ \"/\" | relative_url }}\">{{ site.title }}</a></header>\n",
		"index.md":              "---\nlayout: default\ntitle: Home\n---\n# {{ page.title }}\n\nWelcome to your new site.\n",
		"about.md":              "---\nlayout: default\ntitle: About\n---\n# About\n\nThis is the about page.\n",
		"404.html":              "---\nlayout: default\ntitle: \"404: Page not found\"\n---\n<h1>404</h1>\n<p>Page not found.</p>\n",
		".gitignore":            "_site/\n.jekyll-cache/\n.jekyll-metadata\nvendor/\n",
		"Gemfile":               "source \"https://rubygems.org\"\n\ngem \"jekyll\", \"~> 4.4\"\n",
	}
	welcome := "---\nlayout: post\ntitle: \"Welcome to Jekyll!\"\ndate: " +
		time.Now().Format("2006-01-02 15:04:05 -0700") + "\ncategories: jekyll update\n---\n" +
		"You'll find this post in your `_posts` directory.\n"
	postName := "_posts/" + time.Now().Format("2006-01-02") + "-welcome-to-jekyll.markdown"
	files[postName] = welcome

	for rel, content := range files {
		if blank && !isConfigOrLayout(rel) {
			content = ""
		}
		if err := writeFile(filepath.Join(path, rel), []byte(content)); err != nil {
			return err
		}
	}
	return nil
}

func isConfigOrLayout(rel string) bool {
	return rel == "_config.yml" || filepath.Dir(rel) == "_layouts" || filepath.Dir(rel) == "_includes"
}

// scaffoldTheme writes a Jekyll theme gem scaffold to a directory named after
// the theme, mirroring `jekyll new-theme`.
func scaffoldTheme(name string, codeOfConduct bool) error {
	dirs := []string{"_layouts", "_includes", "_sass", "assets"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(name, d), 0o755); err != nil {
			return err
		}
	}
	themeName := filepath.Base(name)
	files := map[string]string{
		themeName + ".gemspec": "# frozen_string_literal: true\n\nGem::Specification.new do |spec|\n" +
			"  spec.name          = \"" + themeName + "\"\n  spec.version       = \"0.1.0\"\n" +
			"  spec.authors       = [\"\"]\n  spec.summary       = \"A Jekyll theme.\"\n" +
			"  spec.homepage      = \"\"\n  spec.license       = \"MIT\"\n\n" +
			"  spec.files         = Dir[\"_layouts/**/*\", \"_includes/**/*\", \"_sass/**/*\", \"assets/**/*\", \"LICENSE.txt\", \"README.md\"]\n\n" +
			"  spec.add_runtime_dependency \"jekyll\", \"~> 4.4\"\nend\n",
		"Gemfile":   "source \"https://rubygems.org\"\n\ngemspec\n",
		"README.md": "# " + name + "\n\nA Jekyll theme scaffold.\n",
		"LICENSE.txt": "The MIT License (MIT)\n\nCopyright (c) " +
			time.Now().Format("2006") + "\n",
		"_layouts/default.html": "<!doctype html>\n<html lang=\"en\">\n<head><meta charset=\"utf-8\"><title>{{ page.title }}</title></head>\n" +
			"<body>{{ content }}</body>\n</html>\n",
		"_layouts/page.html": "---\nlayout: default\n---\n<article>{{ content }}</article>\n",
		"_layouts/post.html": "---\nlayout: default\n---\n<article>{{ content }}</article>\n",
	}
	if codeOfConduct {
		files["CODE_OF_CONDUCT.md"] = "# Contributor Covenant Code of Conduct\n\nBe excellent to each other.\n"
	}
	for rel, content := range files {
		if err := writeFile(filepath.Join(name, rel), []byte(content)); err != nil {
			return err
		}
	}
	return nil
}
