package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"golang.org/x/term"
)

var helpVerbSwap = regexp.MustCompile(`\b(view|messages|reply|resolve|open|pending|snooze|assign|unassign|label|priority|contact|conversations)\s+<id>`)

// idFirstHelpPrinter runs Kong's default printer into a buffer, swaps
// `<verb> <id>` → `<id> <verb>` so help reads in the user-facing order,
// tightens the layout, and adds color when a person is reading it.
func idFirstHelpPrinter(o kong.HelpOptions, ctx *kong.Context) error {
	var buf bytes.Buffer
	orig := ctx.Stdout
	ctx.Stdout = &buf
	err := kong.DefaultHelpPrinter(o, ctx)
	ctx.Stdout = orig
	if err != nil {
		return err
	}

	text := layoutHelp(helpVerbSwap.ReplaceAllString(buf.String(), `<id> $1`))
	if wantHelpColor(isTerminal(orig), ctx.Args) {
		text = colorizeHelp(text)
	}
	_, werr := fmt.Fprint(orig, text)
	return werr
}

// helpHeader matches section headers: "Examples:", "Flags:", "Conversations".
var helpHeader = regexp.MustCompile(`^[A-Z][A-Za-z ]*:?$`)

var exampleIndent = regexp.MustCompile(`(?m)^    ((?:chatwoot|export) )`)

// layoutHelp gives every section header a blank line under it and moves the
// section's content two columns left, so entries (commands, flags, examples)
// start flush left and their descriptions sit two spaces in.
func layoutHelp(text string) string {
	lines := strings.Split(exampleIndent.ReplaceAllString(text, "  $1"), "\n")
	out := make([]string, 0, len(lines)+8)
	inSection := false
	for i, line := range lines {
		switch {
		case helpHeader.MatchString(line):
			inSection = true
			out = append(out, line)
			if i+1 < len(lines) && lines[i+1] != "" {
				out = append(out, "")
			}
		case inSection && strings.HasPrefix(line, "  "):
			out = append(out, line[2:])
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// wantHelpColor reports whether help should be colored: only for a person at
// a terminal, and never with --no-color, NO_COLOR, or TERM=dumb.
func wantHelpColor(terminal bool, args []string) bool {
	return terminal &&
		os.Getenv("NO_COLOR") == "" &&
		os.Getenv("TERM") != "dumb" &&
		!slices.Contains(args, "--no-color")
}

func isTerminal(w any) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

const ansiReset = "\x1b[0m"

// helpStyle is muted on purpose: 256-color codes that read on dark and light
// backgrounds. Plain text (example commands) keeps the terminal's own color.
var helpStyle = struct {
	header, command, flag, placeholder, note string
}{
	header:      "\x1b[1;38;5;75m",  // bold soft blue
	command:     "\x1b[1m",          // bold
	flag:        "\x1b[38;5;80m",    // muted cyan
	placeholder: "\x1b[3;38;5;245m", // italic gray
	note:        "\x1b[38;5;244m",   // gray
}

var (
	helpPlaceholder = regexp.MustCompile(`\[<[a-z-]+>\](?: \.\.\.)?|<[a-z-]+>(?: \.\.\.)?|=[A-Z][A-Z,.|@]*|="[^"]*"|=\d+`)
	helpExample     = regexp.MustCompile(`^((?:chatwoot|export) .*?)(?:(\s{2,})(\S.*))?$`)
	helpCommand     = regexp.MustCompile(`^([a-z][\w-]*(?: \([\w-]+\))?(?: [a-z][\w-]*)*?)((?: [<\[].*)?)$`)
	helpFlag        = regexp.MustCompile(`^(\s*)((?:-\w, )?--[\w-]+\S*)(.*)$`)
	helpArgument    = regexp.MustCompile(`^(\[?<[\w-]+>\]?(?: \.\.\.)?)(.*)$`)
)

func paint(style, s string) string {
	if s == "" {
		return s
	}
	return style + s + ansiReset
}

func paintPlaceholders(s, after string) string {
	return helpPlaceholder.ReplaceAllStringFunc(s, func(m string) string {
		return helpStyle.placeholder + m + ansiReset + after
	})
}

// colorizeHelp styles laid-out help text. Each line is styled by the section
// it's in (Flags, Arguments, Examples, or a group of commands); lines that
// start with a space are descriptions and stay plain. It only adds escape
// codes: stripping them gives back the plain text.
func colorizeHelp(text string) string {
	lines := strings.Split(text, "\n")
	section := ""
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "Usage:"):
			lines[i] = paint(helpStyle.header, "Usage:") + paintPlaceholders(line[len("Usage:"):], "")
			continue
		case helpHeader.MatchString(line):
			section = line
			lines[i] = paint(helpStyle.header, line)
			continue
		case section == "" || line == "":
			continue
		}

		switch section {
		case "Flags:":
			if m := helpFlag.FindStringSubmatch(line); m != nil {
				lines[i] = m[1] + helpStyle.flag + paintPlaceholders(m[2], helpStyle.flag) + ansiReset + m[3]
			}
		case "Arguments:":
			if m := helpArgument.FindStringSubmatch(line); m != nil {
				lines[i] = paint(helpStyle.placeholder, m[1]) + m[2]
			}
		case "Examples:":
			if m := helpExample.FindStringSubmatch(line); m != nil {
				lines[i] = m[1] + m[2] + paint(helpStyle.note, m[3])
			}
		default: // a group of commands
			if m := helpCommand.FindStringSubmatch(line); m != nil {
				lines[i] = paint(helpStyle.command, m[1]) + paintPlaceholders(m[2], "")
			}
		}
	}
	return strings.Join(lines, "\n")
}
