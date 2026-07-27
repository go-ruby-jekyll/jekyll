// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strings"
)

// fileHandler serves the built site rooted at dest, under the given baseurl
// prefix. When showDirListing is false, directory listings are suppressed in
// favour of an index.html (matching WEBrick's behaviour under Jekyll).
func fileHandler(dest, baseurl string, showDirListing bool) http.Handler {
	fs := http.FileServer(noDirListing{http.Dir(dest), showDirListing})
	prefix := strings.TrimSuffix(baseurl, "/")
	if prefix == "" {
		return fs
	}
	mux := http.NewServeMux()
	mux.Handle(prefix+"/", http.StripPrefix(prefix, fs))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, prefix+"/", http.StatusFound)
	})
	return mux
}

// noDirListing wraps an http.FileSystem to optionally hide directory listings.
type noDirListing struct {
	fs      http.FileSystem
	listing bool
}

func (n noDirListing) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if n.listing {
		return f, nil
	}
	return nonListingFile{f}, nil
}

type nonListingFile struct{ http.File }

// Readdir returns no entries so http.FileServer will not render a populated
// listing; an index.html (served before Readdir is consulted) still works.
func (f nonListingFile) Readdir(int) ([]fs.FileInfo, error) {
	return nil, nil
}

// serveSite starts the development HTTP server. In detach mode it starts the
// listener and returns; otherwise it blocks serving requests.
func serveSite(site *Site, host string, port int, p *parsed, stdout, stderr io.Writer) int {
	addr := fmt.Sprintf("%s:%d", host, port)
	baseurl := site.Config.str("baseurl")
	handler := fileHandler(site.Dest, baseurl, p.bools["show-dir-listing"])
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	scheme := "http"
	fmt.Fprintf(stdout, "    Server address: %s://%s%s/\n", scheme, addr, strings.TrimSuffix(baseurl, "/"))
	srv := &http.Server{Handler: handler}
	if p.bools["detach"] {
		fmt.Fprintf(stdout, "  Server detached, listening on %s\n", addr)
		go serveLoop(srv, ln)
		return 0
	}
	fmt.Fprintf(stdout, "  Server running... press ctrl-c to stop.\n")
	if err := serveLoop(srv, ln); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// serveLoop is the blocking accept/serve loop, factored behind a package var so
// tests can drive serveSite's startup, host, detach and error paths
// deterministically without ever depending on a real indefinite ListenAndServe
// (whose shutdown semantics differ across platforms). Production runs the real
// loop, which returns http.ErrServerClosed on a graceful shutdown.
var serveLoop = realServeLoop

func realServeLoop(srv *http.Server, ln net.Listener) error {
	return srv.Serve(ln)
}
