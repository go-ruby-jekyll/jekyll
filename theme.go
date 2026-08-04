// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Theme resolves a Jekyll theme gem laid out in the standard structure
// (_layouts, _includes, _sass, assets) and layers its files UNDER the site's own
// so that a site file always overrides the theme's equivalent, exactly like
// Jekyll::Theme. Only the standard directory layout is handled; a theme that
// ships its files elsewhere is not resolved (the gap is named in the README).
type Theme struct {
	// Name is the configured theme name (e.g. "minima").
	Name string
	// Root is the absolute path of the resolved gem directory, or "" when the
	// theme gem could not be located.
	Root string
}

// resolveTheme locates the gem directory for a theme by scanning the Ruby gem
// roots for a `<name>-<version>` directory, choosing the highest version. It
// returns an empty Root when no such gem is installed.
func resolveTheme(name string) Theme {
	return Theme{Name: name, Root: findThemeInRoots(gemThemeRoots(), name)}
}

// gemThemeRoots is the (overridable) source of gem "gems" directories to scan.
var gemThemeRoots = defaultGemThemeRoots

// gemGempathOutput runs `gem environment gempath`. It is a package variable so
// tests can drive both the success and failure paths deterministically.
var gemGempathOutput = func() ([]byte, error) {
	return exec.Command("gem", "environment", "gempath").Output()
}

// gemEnvGempath returns the `gem environment gempath` output, or "" when the
// `gem` executable is unavailable.
func gemEnvGempath() string {
	out, err := gemGempathOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// defaultGemThemeRoots derives the list of gem `gems` directories from the
// GEM_HOME and GEM_PATH environment variables and, as a fallback, from the
// `gem environment gempath` command.
func defaultGemThemeRoots() []string {
	return gemRootsFromEnv(os.Getenv("GEM_HOME"), os.Getenv("GEM_PATH"), gemEnvGempath())
}

// gemRootsFromEnv turns the raw GEM_HOME, GEM_PATH and `gem environment gempath`
// values into an ordered, de-duplicated list of `<gemdir>/gems` directories.
func gemRootsFromEnv(gemHome, gemPath, gemEnv string) []string {
	var roots []string
	seen := map[string]bool{}
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		gems := filepath.Join(dir, "gems")
		if seen[gems] {
			return
		}
		seen[gems] = true
		roots = append(roots, gems)
	}
	add(gemHome)
	for _, p := range filepath.SplitList(gemPath) {
		add(p)
	}
	for _, p := range filepath.SplitList(gemEnv) {
		add(p)
	}
	return roots
}

// findThemeInRoots scans each gem root for `<name>-<version>` directories and
// returns the path of the highest-versioned match, or "" when none exists.
func findThemeInRoots(roots []string, name string) string {
	prefix := name + "-"
	var best string
	var bestVer []int
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
				continue
			}
			ver := parseGemVersion(strings.TrimPrefix(e.Name(), prefix))
			if ver == nil {
				continue
			}
			if best == "" || compareGemVersion(ver, bestVer) > 0 {
				best = filepath.Join(root, e.Name())
				bestVer = ver
			}
		}
	}
	return best
}

// parseGemVersion splits a gem version ("2.5.2") into its numeric components. A
// segment that is not purely numeric (a pre-release tag) stops the parse; nil is
// returned when there is no leading numeric component at all.
func parseGemVersion(v string) []int {
	var out []int
	for _, seg := range strings.Split(v, ".") {
		n, err := strconv.Atoi(seg)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// compareGemVersion compares two parsed versions, returning -1, 0 or 1.
func compareGemVersion(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var ai, bi int
		if i < len(a) {
			ai = a[i]
		}
		if i < len(b) {
			bi = b[i]
		}
		switch {
		case ai < bi:
			return -1
		case ai > bi:
			return 1
		}
	}
	return 0
}

// themeDir returns the absolute path of one of the theme's standard
// sub-directories (e.g. "_layouts"), or "" when no theme is resolved.
func (s *Site) themeDir(sub string) string {
	if s.themeRoot == "" {
		return ""
	}
	return filepath.Join(s.themeRoot, sub)
}
