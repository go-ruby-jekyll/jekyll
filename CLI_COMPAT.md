<!--
Copyright (c) the go-ruby-jekyll/jekyll authors
SPDX-License-Identifier: BSD-3-Clause
-->

# Jekyll CLI compatibility specification

Captured verbatim from the reference implementation on 2026-07-27:

```
$ jekyll --version
jekyll 4.4.1
$ ruby --version
ruby 4.0.5 (2026-05-20 revision 64336ffd0e) +PRISM [arm64-darwin25]
```

This file is the **differential oracle** for the pure-Go `jekyll` binary. Every
row is transcribed from the real `jekyll help <subcommand>` output. The `Match`
column records whether `go-ruby-jekyll/jekyll` reproduces the exact
long/short/argument/description surface (parser-level) and, where applicable, the
behaviour.

- `Surface` = the flag is accepted with the identical long form, short form,
  argument arity and help description.
- `Behaviour` = the flag actually changes the build the way real Jekyll does.

## Command tree (mercenary)

```
jekyll <subcommand> [options]

  compose                 (plugin: jekyll-compose — not bundled)
  docs                    (plugin: jekyll-docs — not bundled)
  import                  (plugin: jekyll-import — not bundled)
  build, b                Build your site
  clean                   Clean the site (removes site output and metadata file) without building.
  doctor, hyde            Search site and print specific deprecation warnings
  help                    Show the help message, optionally for a given subcommand.
  new                     Creates a new Jekyll site scaffold in PATH
  new-theme               Creates a new Jekyll theme scaffold
  serve, server, s        Serve your site locally
```

Aliases: `build`→`b`, `serve`→`server`,`s`, `doctor`→`hyde`.

## Global options (inherited by every subcommand, printed last in each help)

| Long | Short | Arg | Description | Surface | Behaviour |
|------|-------|-----|-------------|:---:|:---:|
| `--source` | `-s` | `[DIR]` | Source directory (defaults to ./) | yes | yes |
| `--destination` | `-d` | `[DIR]` | Destination directory (defaults to ./_site) | yes | yes |
| `--safe` | | | Safe mode (defaults to false) | yes | yes (disables custom plugins/symlinks) |
| `--plugins` | `-p` | `PLUGINS_DIR1[,PLUGINS_DIR2[,...]]` | Plugins directory (defaults to ./_plugins) | yes | partial (dir recorded; no Ruby plugin exec) |
| `--layouts` | | `DIR` | Layouts directory | yes | yes |
| `--profile` | | | Generate a Liquid rendering profile | yes | no-op (accepted; no profiler) |
| `--help` | `-h` | | Show this message | yes | yes |
| `--version` | `-v` | | Print the name and version | yes | yes |
| `--trace` | `-t` | | Show the full backtrace when an error occurs | yes | yes |

## `build` (alias `b`) — Build your site

Inherits the global block plus:

| Long | Short | Arg | Description | Surface | Behaviour |
|------|-------|-----|-------------|:---:|:---:|
| `--config` | | `CONFIG_FILE[,CONFIG_FILE2,...]` | Custom configuration file | yes | yes |
| `--destination` | `-d` | `DESTINATION` | The current folder will be generated into DESTINATION | yes | yes |
| `--source` | `-s` | `SOURCE` | Custom source directory | yes | yes |
| `--future` | | | Publishes posts with a future date | yes | yes |
| `--limit_posts` | | `MAX_POSTS` | Limits the number of posts to parse and publish | yes | yes |
| `--watch` | `-w` | `--[no-]watch` | Watch for changes and rebuild | yes | partial (polling watcher; see residuals) |
| `--baseurl` | `-b` | `URL` | Serve the website from the given base URL | yes | yes |
| `--force_polling` | | | Force watch to use polling | yes | yes (polling is the only mode) |
| `--lsi` | | | Use LSI for improved related posts | yes | **no** (residual: no LSI) |
| `--drafts` | `-D` | | Render posts in the _drafts folder | yes | yes |
| `--unpublished` | | | Render posts that were marked as unpublished | yes | yes |
| `--disable-disk-cache` | | | Disable caching to disk in non-safe mode | yes | no-op (no disk cache implemented) |
| `--quiet` | `-q` | | Silence output. | yes | yes |
| `--verbose` | `-V` | | Print verbose output. | yes | yes |
| `--incremental` | `-I` | | Enable incremental rebuild. | yes | **no** (residual: full rebuild always) |
| `--strict_front_matter` | | | Fail if errors are present in front matter | yes | yes |

## `serve` (aliases `server`, `s`) — Serve your site locally

Inherits the global block and the full `build` block, plus:

| Long | Short | Arg | Description | Surface | Behaviour |
|------|-------|-----|-------------|:---:|:---:|
| `--ssl-cert` | | `[CERT]` | X.509 (SSL) certificate. | yes | yes |
| `--host` | `-H` | `[HOST]` | Host to bind to | yes | yes |
| `--open-url` | `-o` | | Launch your site in a browser | yes | partial (best-effort `open`) |
| `--detach` | `-B` | | Run the server in the background | yes | yes |
| `--ssl-key` | | `[KEY]` | X.509 (SSL) Private Key. | yes | yes |
| `--port` | `-P` | `[PORT]` | Port to listen on | yes | yes |
| `--show-dir-listing` | | | Show a directory listing instead of loading your index file. | yes | yes |
| `--skip-initial-build` | | | Skips the initial site build which occurs before the server is started. | yes | yes |
| `--livereload` | `-l` | | Use LiveReload to automatically refresh browsers | yes | **no** (residual: flag accepted, no LR socket) |
| `--livereload-ignore` | | `GLOB1[,GLOB2[,...]]` | Files for LiveReload to ignore. | yes | no-op |
| `--livereload-min-delay` | | `[SECONDS]` | Minimum reload delay | yes | no-op |
| `--livereload-max-delay` | | `[SECONDS]` | Maximum reload delay | yes | no-op |
| `--livereload-port` | | `[PORT]` | Port for LiveReload to listen on | yes | no-op |

## `new PATH` — Creates a new Jekyll site scaffold in PATH

Inherits global block plus:

| Long | Short | Arg | Description | Surface | Behaviour |
|------|-------|-----|-------------|:---:|:---:|
| `--force` | | | Force creation even if PATH already exists | yes | yes |
| `--blank` | | | Creates scaffolding but with empty files | yes | yes |
| `--skip-bundle` | | | Skip 'bundle install' | yes | yes (never runs bundle) |

## `new-theme NAME` — Creates a new Jekyll theme scaffold

Inherits global block plus:

| Long | Short | Arg | Description | Surface | Behaviour |
|------|-------|-----|-------------|:---:|:---:|
| `--code-of-conduct` | `-c` | | Include a Code of Conduct. (defaults to false) | yes | yes |

## `clean` — Clean the site (removes site output and metadata file) without building.

Inherits global block plus the full `build` block (mercenary prints it). Only
`--source`, `--destination`, `--config` affect behaviour.

## `doctor` (alias `hyde`) — Search site and print specific deprecation warnings

Inherits global block only. Runs configuration/URL sanity checks.

## `docs`, `import`, `compose`

Plugin-provided subcommands. In stock Jekyll they are listed but require the
`jekyll-docs`, `jekyll-import`, `jekyll-compose` gems respectively; without them
they print an "unknown command" style error at run time. `go-ruby-jekyll`
reproduces the **listing** and prints an equivalent "not installed" message.

## Residual (not-yet-parity) behaviours — named loudly, not hidden

| Area | Status | Reason |
|------|--------|--------|
| SCSS/Sass conversion (`.scss`/`.sass`) | **deferred** | `go-ruby-sass` does not exist yet; `SassConverter` is a named pluggable stub that copies source through and emits a loud warning. |
| `--lsi` related posts | not implemented | needs a latent-semantic-indexing model; flag parsed, ignored. |
| `--incremental` | not implemented | full rebuild is always performed; flag parsed, ignored. |
| `--livereload` websocket | not implemented | flag parsed; server serves without the live-reload injection. |
| Ruby `_plugins/*.rb` execution | not implemented | pure-Go, CGO=0; custom Ruby plugins are not evaluated. `--plugins` dir is still recorded. |
| `--profile` | no-op | accepted for surface parity; no Liquid profiler. |
| Array/hash Jekyll filters (`where_exp`, `group_by`, `jsonify` of arrays) | partial | string-valued Jekyll filters are exact; array/hash ones depend on the go-ruby-liquid extension gap (see README "Substrate gaps"). |
