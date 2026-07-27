// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package main

import "testing"

func TestMainEntry(t *testing.T) {
	var gotCode int
	osExit = func(c int) { gotCode = c }
	osArgs = []string{"jekyll", "--version"}
	main()
	if gotCode != 0 {
		t.Fatalf("main exit code = %d, want 0", gotCode)
	}
}
