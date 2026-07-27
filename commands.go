// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// buildConfig resolves configuration from parsed build/serve/clean options.
func buildConfig(p *parsed) (Config, error) {
	source := "."
	if v, ok := p.vals["source"]; ok {
		source = v
	}
	var configFiles []string
	if v, ok := p.vals["config"]; ok {
		for _, f := range strings.Split(v, ",") {
			if f = strings.TrimSpace(f); f != "" {
				configFiles = append(configFiles, f)
			}
		}
	}
	cfg, err := LoadConfig(source, configFiles)
	if err != nil {
		return nil, err
	}
	cfg["source"] = source
	if v, ok := p.vals["destination"]; ok {
		cfg["destination"] = v
	}
	if v, ok := p.vals["baseurl"]; ok {
		cfg["baseurl"] = v
	}
	if v, ok := p.vals["limit_posts"]; ok {
		cfg["limit_posts"] = atoiDefault(v, 0)
	}
	if v, ok := p.vals["plugins"]; ok {
		cfg["plugins_dir"] = v
	}
	if v, ok := p.vals["layouts"]; ok {
		cfg["layouts_dir"] = v
	}
	if p.bools["drafts"] {
		cfg["show_drafts"] = true
	}
	if p.bools["future"] {
		cfg["future"] = true
	}
	if p.bools["unpublished"] {
		cfg["unpublished"] = true
	}
	if p.bools["safe"] {
		cfg["safe"] = true
	}
	if p.bools["strict_front_matter"] {
		cfg["strict_front_matter"] = true
	}
	return cfg, nil
}

func configFilePath(cfg Config, p *parsed) string {
	if v, ok := p.vals["config"]; ok {
		return v
	}
	def := filepath.Join(cfg.str("source"), "_config.yml")
	if _, err := os.Stat(def); err == nil {
		return def
	}
	return "none"
}

func cmdBuild(p *parsed, stdout, stderr io.Writer) int {
	cfg, err := buildConfig(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	site := NewSite(cfg)
	quiet := p.bools["quiet"]
	if !quiet {
		fmt.Fprintf(stdout, "Configuration file: %s\n", configFilePath(cfg, p))
		fmt.Fprintf(stdout, "            Source: %s\n", site.Source)
		fmt.Fprintf(stdout, "       Destination: %s\n", site.Dest)
		fmt.Fprintf(stdout, " Incremental build: disabled. Enable with --incremental\n")
		fmt.Fprintf(stdout, "      Generating... \n")
	}
	start := time.Now()
	if err := site.clean(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if err := site.Build(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		if p.bools["trace"] {
			fmt.Fprintf(stderr, "%+v\n", err)
		}
		return 1
	}
	for _, w := range site.warnings {
		fmt.Fprintf(stderr, "Warning: %s\n", w)
	}
	if !quiet {
		fmt.Fprintf(stdout, "                    done in %.3f seconds.\n", time.Since(start).Seconds())
		fmt.Fprintf(stdout, " Auto-regeneration: disabled. Use --watch to enable.\n")
	}
	return 0
}

func cmdClean(p *parsed, stdout, stderr io.Writer) int {
	cfg, err := buildConfig(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	site := NewSite(cfg)
	if err := site.clean(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	if !p.bools["quiet"] {
		fmt.Fprintf(stdout, "Cleaner: Removing %s...\n", site.Dest)
	}
	return 0
}

func cmdDoctor(p *parsed, stdout, stderr io.Writer) int {
	cfg, err := buildConfig(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	var warnings []string
	baseurl := cfg.str("baseurl")
	if baseurl != "" && !strings.HasPrefix(baseurl, "/") {
		warnings = append(warnings, "Your baseurl should be a path, not a URL. It should start with a forward slash.")
	}
	url := cfg.str("url")
	if url != "" && strings.HasSuffix(url, "/") {
		warnings = append(warnings, "Your url should not end with a trailing slash.")
	}
	if len(warnings) == 0 {
		fmt.Fprintln(stdout, "Configuration file: "+configFilePath(cfg, p))
		fmt.Fprintln(stdout, "Everything looks fine.")
		return 0
	}
	for _, w := range warnings {
		fmt.Fprintln(stdout, w)
	}
	return 0
}

func cmdServe(p *parsed, stdout, stderr io.Writer) int {
	cfg, err := buildConfig(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	site := NewSite(cfg)
	if !p.bools["skip-initial-build"] {
		if err := site.clean(); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		if err := site.Build(); err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
	}
	host := "127.0.0.1"
	if v, ok := p.vals["host"]; ok {
		host = v
	}
	port := 4000
	if v, ok := p.vals["port"]; ok {
		port = atoiDefault(v, 4000)
	}
	return serveSite(site, host, port, p, stdout, stderr)
}

// ---- scaffolding: new and new-theme ----

func cmdNew(p *parsed, stdout, stderr io.Writer) int {
	if len(p.pos) == 0 {
		fmt.Fprintln(stderr, "You must specify a path.")
		return 1
	}
	path := p.pos[0]
	if info, err := os.Stat(path); err == nil {
		empty := true
		if info.IsDir() {
			entries, _ := os.ReadDir(path)
			empty = len(entries) == 0
		}
		if !empty && !p.bools["force"] {
			fmt.Fprintf(stderr, "Conflict: %s exists and is not empty.\n", path)
			return 1
		}
	}
	if err := scaffoldSite(path, p.bools["blank"]); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	abs, _ := filepath.Abs(path)
	fmt.Fprintf(stdout, "New jekyll site installed in %s.\n", abs)
	return 0
}

func cmdNewTheme(p *parsed, stdout, stderr io.Writer) int {
	if len(p.pos) == 0 {
		fmt.Fprintln(stderr, "You must specify a theme name.")
		return 1
	}
	name := p.pos[0]
	if err := scaffoldTheme(name, p.bools["code-of-conduct"]); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	abs, _ := filepath.Abs(name)
	fmt.Fprintf(stdout, "Your new Jekyll theme, %s, is ready for you in %s!\n", name, abs)
	return 0
}
