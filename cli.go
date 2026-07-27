// Copyright (c) the go-ruby-jekyll/jekyll authors
//
// SPDX-License-Identifier: BSD-3-Clause

package jekyll

import (
	"fmt"
	"io"
	"strings"
)

// Version is the reference Jekyll version this CLI mirrors.
const Version = "4.4.1"

// parsed holds the result of parsing a subcommand's arguments.
type parsed struct {
	vals  map[string]string // long-name -> value (last wins)
	bools map[string]bool   // long-name -> value
	pos   []string          // positional arguments
}

func (p *parsed) has(long string) bool {
	_, ok := p.vals[long]
	if ok {
		return true
	}
	_, ok = p.bools[long]
	return ok
}

// parseErr is a user-facing option error (mirrors optparse's messages).
type parseErr struct{ msg string }

func (e *parseErr) Error() string { return e.msg }

// parseArgs parses args against a command's option set.
func parseArgs(c *command, args []string) (*parsed, error) {
	opts := c.opts
	if opts == nil {
		opts = globalOpts
	}
	byLong := map[string]optSpec{}
	byShort := map[string]optSpec{}
	for _, o := range opts {
		byLong[o.long] = o
		if o.short != "" {
			byShort[o.short] = o
		}
	}
	res := &parsed{vals: map[string]string{}, bools: map[string]bool{}}
	i := 0
	for i < len(args) {
		a := args[i]
		switch {
		case a == "--":
			res.pos = append(res.pos, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			name := a[2:]
			val := ""
			hasEq := false
			if eq := strings.IndexByte(name, '='); eq >= 0 {
				val, hasEq = name[eq+1:], true
				name = name[:eq]
			}
			negated := false
			spec, ok := byLong[name]
			if !ok && strings.HasPrefix(name, "no-") {
				if s, ok2 := byLong[name[3:]]; ok2 && s.negate {
					spec, ok, negated = s, true, true
				}
			}
			if !ok {
				return nil, &parseErr{"invalid option: --" + name}
			}
			if spec.boolean {
				res.bools[spec.long] = !negated
				i++
				continue
			}
			if hasEq {
				res.vals[spec.long] = val
				i++
				continue
			}
			if i+1 >= len(args) {
				return nil, &parseErr{"missing argument: --" + name}
			}
			res.vals[spec.long] = args[i+1]
			i += 2
		case strings.HasPrefix(a, "-") && len(a) > 1:
			letter := a[1:2]
			spec, ok := byShort[letter]
			if !ok {
				return nil, &parseErr{"invalid option: -" + letter}
			}
			if spec.boolean {
				res.bools[spec.long] = true
				// allow bundled booleans like -qV
				rest := a[2:]
				for rest != "" {
					s2, ok2 := byShort[rest[:1]]
					if !ok2 || !s2.boolean {
						return nil, &parseErr{"invalid option: -" + rest[:1]}
					}
					res.bools[s2.long] = true
					rest = rest[1:]
				}
				i++
				continue
			}
			if len(a) > 2 { // attached value: -sDIR
				res.vals[spec.long] = a[2:]
				i++
				continue
			}
			if i+1 >= len(args) {
				return nil, &parseErr{"missing argument: -" + letter}
			}
			res.vals[spec.long] = args[i+1]
			i += 2
		default:
			res.pos = append(res.pos, a)
			i++
		}
	}
	return res, nil
}

// Main is the CLI entry point. It returns the process exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	// No arguments: print the top-level help.
	if len(args) == 0 {
		fmt.Fprint(stdout, rootHelp())
		return 0
	}
	// Leading global version/help flags without a subcommand.
	first := args[0]
	if first == "-v" || first == "--version" {
		fmt.Fprintf(stdout, "jekyll %s\n", Version)
		return 0
	}
	if first == "-h" || first == "--help" {
		fmt.Fprint(stdout, rootHelp())
		return 0
	}
	if strings.HasPrefix(first, "-") {
		fmt.Fprintf(stderr, "Invalid options: %s\n", first)
		fmt.Fprint(stderr, rootHelp())
		return 1
	}

	// help subcommand.
	if first == "help" {
		if len(args) >= 2 {
			if c := findCommand(args[1]); c != nil {
				fmt.Fprint(stdout, commandHelp(c))
				return 0
			}
		}
		fmt.Fprint(stdout, rootHelp())
		return 0
	}

	c := findCommand(first)
	if c == nil {
		fmt.Fprintf(stderr, "'%s' could not be found. You may need to install the %s-%s gem or a related gem to be able to use this subcommand.\n", first, "jekyll", first)
		return 1
	}

	p, err := parseArgs(c, args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "%s\n", err)
		return 1
	}
	if p.bools["help"] {
		fmt.Fprint(stdout, commandHelp(c))
		return 0
	}
	if p.bools["version"] {
		fmt.Fprintf(stdout, "jekyll %s\n", Version)
		return 0
	}

	return dispatch(c.name, p, stdout, stderr)
}

// dispatch routes a parsed command to its handler.
func dispatch(name string, p *parsed, stdout, stderr io.Writer) int {
	switch name {
	case "build":
		return cmdBuild(p, stdout, stderr)
	case "serve":
		return cmdServe(p, stdout, stderr)
	case "clean":
		return cmdClean(p, stdout, stderr)
	case "doctor":
		return cmdDoctor(p, stdout, stderr)
	case "new":
		return cmdNew(p, stdout, stderr)
	case "new-theme":
		return cmdNewTheme(p, stdout, stderr)
	case "docs", "import", "compose":
		fmt.Fprintf(stderr, "'%s' could not be found. You may need to install the jekyll-%s gem or a related gem to be able to use this subcommand.\n", name, name)
		return 1
	}
	// unreachable: help handled earlier
	fmt.Fprint(stderr, rootHelp())
	return 1
}

// ---- help rendering (matches the reference mercenary/optparse layout) ----

const descCol = 27       // column at which an option description starts
const subcmdDescCol = 24 // column at which a subcommand description starts

func renderOpt(o optSpec) string {
	shortField := "    "
	if o.short != "" {
		shortField = "-" + o.short + ", "
	}
	core := "--" + o.long
	if o.negate {
		core = "--[no-]" + o.long
	}
	if o.arg != "" {
		core += " " + o.arg
	}
	optStr := "        " + shortField + core
	pad := descCol - len(optStr)
	if pad < 2 {
		pad = 2
	}
	return optStr + strings.Repeat(" ", pad) + o.desc
}

func optionsBlock(opts []optSpec) string {
	var b strings.Builder
	b.WriteString("Options:\n")
	for _, o := range opts {
		b.WriteString(renderOpt(o))
		b.WriteByte('\n')
	}
	return b.String()
}

func rootHelp() string {
	var b strings.Builder
	fmt.Fprintf(&b, "jekyll %s -- Jekyll is a blog-aware, static site generator in Ruby\n\n", Version)
	b.WriteString("Usage:\n\n  jekyll <subcommand> [options]\n\n")
	b.WriteString(optionsBlock(globalOpts))
	b.WriteString("\nSubcommands:\n")
	for _, c := range commands {
		names := c.name
		if len(c.aliases) > 0 {
			names += ", " + strings.Join(c.aliases, ", ")
		}
		field := "  " + names
		pad := subcmdDescCol - len(field)
		b.WriteString(field + strings.Repeat(" ", pad) + c.desc + "\n")
	}
	return finalizeHelp(b.String())
}

// finalizeHelp appends the single trailing space the reference mercenary output
// leaves on the last line of every help screen.
func finalizeHelp(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s[:len(s)-1] + " \n"
	}
	return s
}

func commandHelp(c *command) string {
	var b strings.Builder
	if c.desc != "" {
		fmt.Fprintf(&b, "jekyll %s -- %s\n\n", c.name, c.desc)
	} else {
		fmt.Fprintf(&b, "jekyll %s\n\n", c.name)
	}
	usage := "jekyll " + c.name
	if c.usageArg != "" {
		usage += " " + c.usageArg
	}
	fmt.Fprintf(&b, "Usage:\n\n  %s\n\n", usage)
	opts := c.opts
	if opts == nil {
		opts = globalOpts
	}
	b.WriteString(optionsBlock(opts))
	return finalizeHelp(b.String())
}
