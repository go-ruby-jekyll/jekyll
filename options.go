// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

// optSpec describes a single command-line switch exactly as the reference
// mercenary/optparse surface presents it. The fields drive both parsing and the
// generated `--help` text, so the two can never drift.
type optSpec struct {
	long    string // long name without leading "--"
	short   string // single-letter short name without "-", or "" if none
	arg     string // argument placeholder as printed (e.g. "[DIR]", "DESTINATION"); "" for a boolean flag
	desc    string // help description
	negate  bool   // supports the --[no-]long form (boolean)
	boolean bool   // takes no argument
}

// The canonical global option block, printed last in every subcommand's help and
// inherited by every command (mirrors Mercenary's default program options).
var globalOpts = []optSpec{
	{long: "source", short: "s", arg: "[DIR]", desc: "Source directory (defaults to ./)"},
	{long: "destination", short: "d", arg: "[DIR]", desc: "Destination directory (defaults to ./_site)"},
	{long: "safe", boolean: true, desc: "Safe mode (defaults to false)"},
	{long: "plugins", short: "p", arg: "PLUGINS_DIR1[,PLUGINS_DIR2[,...]]", desc: "Plugins directory (defaults to ./_plugins)"},
	{long: "layouts", arg: "DIR", desc: "Layouts directory (defaults to ./_layouts)"},
	{long: "profile", boolean: true, desc: "Generate a Liquid rendering profile"},
	{long: "help", short: "h", boolean: true, desc: "Show this message"},
	{long: "version", short: "v", boolean: true, desc: "Print the name and version"},
	{long: "trace", short: "t", boolean: true, desc: "Show the full backtrace when an error occurs"},
}

// The build option block, shared by build, serve and clean.
var buildOpts = []optSpec{
	{long: "config", arg: "CONFIG_FILE[,CONFIG_FILE2,...]", desc: "Custom configuration file"},
	{long: "destination", short: "d", arg: "DESTINATION", desc: "The current folder will be generated into DESTINATION"},
	{long: "source", short: "s", arg: "SOURCE", desc: "Custom source directory"},
	{long: "future", boolean: true, desc: "Publishes posts with a future date"},
	{long: "limit_posts", arg: "MAX_POSTS", desc: "Limits the number of posts to parse and publish"},
	{long: "watch", short: "w", boolean: true, negate: true, desc: "Watch for changes and rebuild"},
	{long: "baseurl", short: "b", arg: "URL", desc: "Serve the website from the given base URL"},
	{long: "force_polling", boolean: true, desc: "Force watch to use polling"},
	{long: "lsi", boolean: true, desc: "Use LSI for improved related posts"},
	{long: "drafts", short: "D", boolean: true, desc: "Render posts in the _drafts folder"},
	{long: "unpublished", boolean: true, desc: "Render posts that were marked as unpublished"},
	{long: "disable-disk-cache", boolean: true, desc: "Disable caching to disk in non-safe mode"},
	{long: "quiet", short: "q", boolean: true, desc: "Silence output."},
	{long: "verbose", short: "V", boolean: true, desc: "Print verbose output."},
	{long: "incremental", short: "I", boolean: true, desc: "Enable incremental rebuild."},
	{long: "strict_front_matter", boolean: true, desc: "Fail if errors are present in front matter"},
}

// The serve-only additions, appended after buildOpts.
var serveOpts = []optSpec{
	{long: "ssl-cert", arg: "[CERT]", desc: "X.509 (SSL) certificate."},
	{long: "host", short: "H", arg: "[HOST]", desc: "Host to bind to"},
	{long: "open-url", short: "o", boolean: true, desc: "Launch your site in a browser"},
	{long: "detach", short: "B", boolean: true, desc: "Run the server in the background"},
	{long: "ssl-key", arg: "[KEY]", desc: "X.509 (SSL) Private Key."},
	{long: "port", short: "P", arg: "[PORT]", desc: "Port to listen on"},
	{long: "show-dir-listing", boolean: true, desc: "Show a directory listing instead of loading your index file."},
	{long: "skip-initial-build", boolean: true, desc: "Skips the initial site build which occurs before the server is started."},
	{long: "livereload", short: "l", boolean: true, desc: "Use LiveReload to automatically refresh browsers"},
	{long: "livereload-ignore", arg: "GLOB1[,GLOB2[,...]]", desc: "Files for LiveReload to ignore. Remember to quote the values so your shell won't expand them"},
	{long: "livereload-min-delay", arg: "[SECONDS]", desc: "Minimum reload delay"},
	{long: "livereload-max-delay", arg: "[SECONDS]", desc: "Maximum reload delay"},
	{long: "livereload-port", arg: "[PORT]", desc: "Port for LiveReload to listen on"},
}

var newOpts = []optSpec{
	{long: "force", boolean: true, desc: "Force creation even if PATH already exists"},
	{long: "blank", boolean: true, desc: "Creates scaffolding but with empty files"},
	{long: "skip-bundle", boolean: true, desc: "Skip 'bundle install'"},
}

var newThemeOpts = []optSpec{
	{long: "code-of-conduct", short: "c", boolean: true, desc: "Include a Code of Conduct. (defaults to false)"},
}

// command describes one subcommand of the CLI.
type command struct {
	name     string
	aliases  []string
	usageArg string // e.g. "[options]", "PATH", "NAME", "[subcommand]"
	desc     string
	opts     []optSpec // full option list in help-print order (may repeat source/destination)
}

func concat(groups ...[]optSpec) []optSpec {
	var out []optSpec
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// commands is the full mercenary command tree, in the exact order the reference
// prints it under "Subcommands:".
var commands = []*command{
	{name: "compose", usageArg: "", desc: ""},
	{name: "docs", usageArg: "", desc: ""},
	{name: "import", usageArg: "", desc: ""},
	{name: "build", aliases: []string{"b"}, usageArg: "[options]", desc: "Build your site", opts: concat(buildOpts, globalOpts)},
	{name: "clean", usageArg: "[subcommand]", desc: "Clean the site (removes site output and metadata file) without building.", opts: concat(buildOpts, globalOpts)},
	{name: "doctor", aliases: []string{"hyde"}, usageArg: "", desc: "Search site and print specific deprecation warnings", opts: concat(buildOpts[:1], globalOpts)},
	{name: "help", usageArg: "", desc: "Show the help message, optionally for a given subcommand."},
	{name: "new", usageArg: "PATH", desc: "Creates a new Jekyll site scaffold in PATH", opts: concat(newOpts, globalOpts)},
	{name: "new-theme", usageArg: "NAME", desc: "Creates a new Jekyll theme scaffold", opts: concat(newThemeOpts, globalOpts)},
	{name: "serve", aliases: []string{"server", "s"}, usageArg: "[options]", desc: "Serve your site locally", opts: concat(buildOpts, serveOpts, globalOpts)},
}

// findCommand resolves a name or alias to its command.
func findCommand(name string) *command {
	for _, c := range commands {
		if c.name == name {
			return c
		}
		for _, a := range c.aliases {
			if a == name {
				return c
			}
		}
	}
	return nil
}
