// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Command jekyll is a pure-Go, CGO-free reimplementation of the Ruby Jekyll
// command-line interface. Its command and option surface is a drop-in match for
// the reference `jekyll` binary (see CLI_COMPAT.md).
package main

import (
	"os"

	jekyll "github.com/go-ruby-jekyll/jekyll"
)

// osExit and osArgs are indirections so main can be exercised in tests.
var (
	osExit = os.Exit
	osArgs = os.Args
)

func main() {
	osExit(jekyll.Main(osArgs[1:], os.Stdout, os.Stderr))
}
