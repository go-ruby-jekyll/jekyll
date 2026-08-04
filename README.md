<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-jekyll/brand/main/social/go-ruby-jekyll-jekyll.png" alt="go-ruby-jekyll/jekyll" width="720"></p>

# jekyll — go-ruby-jekyll

[![ci](https://github.com/go-ruby-jekyll/jekyll/actions/workflows/ci.yml/badge.svg)](https://github.com/go-ruby-jekyll/jekyll/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-ruby-jekyll/jekyll.svg)](https://pkg.go.dev/github.com/go-ruby-jekyll/jekyll)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-ruby-jekyll/jekyll)](https://goreportcard.com/report/github.com/go-ruby-jekyll/jekyll)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)
[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-jekyll.github.io/docs/)

**A pure-Go (no cgo) `jekyll` command-line tool** whose command and option
surface is a drop-in match for Ruby [Jekyll](https://jekyllrb.com) 4.4.1, and
whose build pipeline reproduces a real site's `_site/` output — verified
**byte-for-byte** against the reference implementation.

It builds on the pure-Go Jekyll pillars:

- [go-ruby-liquid/liquid](https://github.com/go-ruby-liquid/liquid) — Liquid templating
- [go-ruby-kramdown/kramdown](https://github.com/go-ruby-kramdown/kramdown) — Markdown
- [go-ruby-rouge/rouge](https://github.com/go-ruby-rouge/rouge) — syntax highlighting

No Ruby runtime is required to build or run it. The resulting binary is static
and CGO-free, and cross-compiles to all six supported 64-bit targets
(amd64/arm64/riscv64/loong64/ppc64le/s390x).

## Install

```sh
go install github.com/go-ruby-jekyll/jekyll/cmd/jekyll@latest
```

## Usage

```sh
jekyll new my-site        # scaffold a self-contained site
jekyll build              # build ./_site
jekyll serve              # build and serve on http://127.0.0.1:4000
jekyll clean              # remove the generated site
jekyll doctor             # sanity-check configuration
```

The full command tree and every option/flag — with exact long/short forms,
defaults and descriptions — is captured in [CLI_COMPAT.md](CLI_COMPAT.md), which
is transcribed verbatim from `jekyll help` and used as the differential oracle.

## What is implemented

The core build pipeline matches Jekyll's behaviour:

- `_config.yml` load + deep-merge + CLI overrides
- YAML front matter (`---` blocks), `--strict_front_matter`
- `_posts`, `_drafts` (`--drafts`), `--future`, `--unpublished`, `--limit_posts`
- Collections (`_config.yml` `collections:`), `output`, per-collection `permalink`
- `_layouts` with layout chaining, `_includes` (incl. `{% include x.html k=v %}`)
- Liquid rendering with Jekyll's filters (`relative_url`, `absolute_url`,
  `date_to_string`/`_xmlschema`/`_rfc822`, `slugify`, `xml_escape`, `jsonify`,
  `markdownify`, `number_of_words`, `array_to_sentence_string`, …) and tags
  (`include`, `highlight`, `link`, `post_url`)
- SCSS/Sass (`.scss`/`.sass`) compiled through
  [go-ruby-sass](https://github.com/go-ruby-sass/sass) (pure-Go, CGO=0,
  dart-sass-compatible over [go-scss](https://github.com/go-scss/scss)), matching
  jekyll-sass-converter: the `_sass` load path (`sass.sass_dir`), `sass.style`,
  `sass.load_paths`, and front-matter-triggered conversion to `.css`
- Kramdown Markdown → HTML with Jekyll's Rouge code-block/inline wrappers
- Permalinks (named styles + `:placeholder` templates), `site`/`page` variables,
  `_data`, static files, `exclude`/underscore/dotfile rules
- `serve` (WEBrick-equivalent HTTP with baseurl prefix), `new`, `new-theme`,
  `clean`, `doctor`
- **Post/collection excerpts** (`page.excerpt`): the content up to
  `excerpt_separator` (default `"\n\n"`, overridable per-document), with trailing
  Markdown link-reference definitions carried over, rendered through the same
  Liquid + Markdown pipeline as the body — matching `Jekyll::Excerpt`
- **jekyll-feed** (enabled via `plugins:` or a theme's gem dependency): generates
  an Atom `/feed.xml` byte-for-byte with the plugin's template — feed
  title/subtitle/author, and per-post title/link/dates/id/content/author
  (resolved through `site.data.authors`)/categories/tags/summary/image, honouring
  `feed.path`, `feed.posts_limit` and `feed.excerpt_only`. The `{% feed_meta %}`
  autodiscovery tag is supported. (Per-collection / per-category / per-tag feed
  *files* are a named gap.)
- **Theme gems** (`theme:` in `_config.yml`): a gem laid out in the standard
  structure (`_layouts`, `_includes`, `_sass`, `assets`) is resolved from the
  Ruby gem path and layered *under* the site's own files, so a site file always
  overrides its theme equivalent — exactly like `Jekyll::Theme`. The gem is
  located by scanning `GEM_HOME`/`GEM_PATH` and `gem environment gempath`.

## Substrate gaps found (reported upstream)

- **go-ruby-liquid had no filter/tag registration API.** Jekyll's filters must be
  evaluable *inside* `{% for %}` loops, so they cannot be pre-expanded by a
  wrapper. This is fixed additively by
  [go-ruby-liquid#6](https://github.com/go-ruby-liquid/liquid/pull/6)
  (`liquid.WithFilter` / `WithFilters`), which this module depends on. Custom
  **tags** still have no engine hook, so `include`/`highlight`/`link`/`post_url`
  are expanded by a preprocessing layer here (works at page/layout scope; an
  `{% include %}` that references a loop variable is a residual — see below).
- **go-ruby-kramdown collapses the blank lines around a raw HTML block.** Ruby
  kramdown keeps a leading newline and a trailing blank line; go-ruby-kramdown
  does not. The only observed effect is the surrounding whitespace of a
  `{% highlight %}` block placed *inside a Markdown document* — the highlighted
  figure content itself is byte-identical. (`{% highlight %}` in an `.html` page
  is byte-identical.)

## Honestly-deferred surface (parsed, not yet behavioural)

| Area | Status |
|------|--------|
| Sass source maps (`sass.sourcemap`) | not emitted — go-scss does not yet produce source maps, so the `.css.map` file and the `/*# sourceMappingURL */` comment that jekyll-sass-converter writes under its default `sourcemap: always` are omitted. CSS output byte-matches the gem on the common surface; set `sass.sourcemap: never` for exact parity. Other go-scss residuals (advanced `@extend` unification, media-query conflict pruning, exotic `sass:meta`/`sass:selector`) are inherited. |
| `--lsi` related posts | flag parsed, ignored (needs a latent-semantic-indexing model). |
| `--incremental` | flag parsed, ignored (a full rebuild is always performed). |
| `--livereload` websocket | flag parsed; server serves without the live-reload injection. |
| `--watch` / `--profile` | parsed; watcher and Liquid profiler not implemented. |
| Ruby `_plugins/*.rb` | not executed (pure-Go, CGO=0). `--plugins` dir is still recorded. |
| `{% include %}` referencing a loop variable | residual of the missing tag-registration hook (page/layout-scope includes work). |
| Array/hash Jekyll filters (`where_exp`, `group_by`) | best-effort; string-valued filters are exact. |
| Theme gems with a non-standard layout | only the standard `_layouts`/`_includes`/`_sass`/`assets` structure is resolved; a theme that ships its files elsewhere (or via a custom `Jekyll::Theme` subclass) is not. |

## Tests & coverage

`testdata/site/` is a controlled Jekyll site; `testdata/expected/` is its
`_site/` as produced by **Ruby Jekyll 4.4.1**. The test suite builds the source
with this tool and asserts the output is byte-for-byte identical, plus unit
tests for the CLI surface, config, front matter, permalinks, filters and
converters. Coverage target: 100% including error branches.

## License

BSD-3-Clause. Copyright (c) the go-ruby-jekyll/jekyll authors.
