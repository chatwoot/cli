package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/alecthomas/kong"
	"github.com/chatwoot/cli/internal/cmd"
)

type helpExit struct{}

// helpFor renders `chatwoot <args> --help` the way a user sees it.
func helpFor(t *testing.T, args ...string) string {
	t.Helper()
	var out bytes.Buffer
	var cli cmd.CLI
	parser, err := newParser(&cli,
		kong.Writers(&out, &out),
		kong.Exit(func(int) { panic(helpExit{}) }),
	)
	if err != nil {
		t.Fatalf("newParser: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(helpExit); !ok {
					panic(r)
				}
			}
		}()
		_, _ = parser.Parse(normalizeArgs(append(args, "--help")))
	}()
	return out.String()
}

// Help text is written for people: every command, argument, and flag says
// something, as a sentence.
func TestEveryCommandArgAndFlagHasHelp(t *testing.T) {
	var cli cmd.CLI
	parser, err := newParser(&cli)
	if err != nil {
		t.Fatalf("newParser: %v", err)
	}

	checkSentence := func(what, help string) {
		t.Helper()
		help = strings.TrimSpace(help)
		if help == "" {
			t.Errorf("%s has no help text", what)
			return
		}
		if first := []rune(help)[0]; !unicode.IsUpper(first) && !unicode.IsDigit(first) {
			t.Errorf("%s help should start with a capital letter: %q", what, help)
		}
		if !strings.HasSuffix(help, ".") && !strings.HasSuffix(help, ")") {
			t.Errorf("%s help should end with a period: %q", what, help)
		}
	}

	var walk func(node *kong.Node)
	walk = func(node *kong.Node) {
		// Shell completion is provided by a library with its own wording.
		if node.Name == "completion" {
			return
		}
		name := node.Path()
		if node.Type != kong.ApplicationNode {
			checkSentence("command "+name, node.Help)
		}
		for _, arg := range node.Positional {
			checkSentence("argument "+name+" <"+arg.Name+">", arg.Help)
		}
		for _, flag := range node.Flags {
			checkSentence("flag "+name+" --"+flag.Name, flag.Help)
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(parser.Model.Node)
}

func TestRootHelpTeachesTheGrammar(t *testing.T) {
	out := helpFor(t)
	for _, want := range []string{
		`chatwoot conv 123 reply "`, // id before verb
		"chatwoot @acme",            // pick an account
		"/app/accounts/",            // paste a link
		"Conversations",             // commands are grouped
		"Accounts and login",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("root help missing %q:\n%s", want, out)
		}
	}
}

func TestCommandHelpHasExamples(t *testing.T) {
	cases := map[string][]string{
		"convs":       {"Examples:", "chatwoot convs --assignee all"},
		"conv":        {"Examples:", "chatwoot conv 123 reply"},
		"conv reply":  {"Examples:", "--private", "customer"},
		"conv assign": {"Examples:", "--agent me", "--team"},
		"conv snooze": {"Examples:", "--until 7d"},
		"conv label":  {"Examples:", "replaces", "billing,urgent"},
		"contacts":    {"Examples:", "--search"},
		"accounts":    {"Examples:", "chatwoot accounts --refresh"},
		"use":         {"Examples:", "CHATWOOT_ACCOUNT"},
		"auth login":  {"Examples:", "chatwoot auth login staging.chatwoot.com", "Older Chatwoot versions"},
		"auth logout": {"Examples:", "chatwoot auth logout staging.chatwoot.com"},
		"hc articles": {"Examples:", "--query"},
		"api":         {"Examples:", "chatwoot api /conversations/123"},
	}
	for command, wants := range cases {
		out := helpFor(t, strings.Fields(command)...)
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("chatwoot %s --help missing %q:\n%s", command, want, out)
			}
		}
	}
}

// Usage lines read id-first, matching how commands are typed.
func TestSubcommandHelpUsesIDFirstOrder(t *testing.T) {
	out := helpFor(t, "conv", "reply")
	if !strings.Contains(out, "<id> reply <text>") {
		t.Fatalf("conv reply usage should read id-first:\n%s", out)
	}
}

// Help fits an 80-column terminal, for every command.
func TestHelpFitsEightyColumns(t *testing.T) {
	var cli cmd.CLI
	parser, err := newParser(&cli)
	if err != nil {
		t.Fatalf("newParser: %v", err)
	}
	var paths [][]string
	var walk func(node *kong.Node, path []string)
	walk = func(node *kong.Node, path []string) {
		if node.Name == "completion" {
			return
		}
		paths = append(paths, path)
		for _, child := range node.Children {
			walk(child, append(append([]string{}, path...), child.Name))
		}
	}
	walk(parser.Model.Node, nil)

	for _, path := range paths {
		for _, line := range strings.Split(helpFor(t, path...), "\n") {
			if n := len([]rune(line)); n > 80 {
				t.Errorf("chatwoot %s --help has a %d-column line:\n%s", strings.Join(path, " "), n, line)
			}
		}
	}
}

func TestLayoutHelpSpacesHeadersAndDedentsSections(t *testing.T) {
	in := "Usage: chatwoot convs\n\nList conversations.\n\nExamples:\n\n    chatwoot convs    Yours\n\n" +
		"Flags:\n  -h, --help      Help.\n      --no-color  Plain.\n                  More.\n\n" +
		"Conversations\n  convs [flags]\n    List conversations.\n"
	want := "Usage: chatwoot convs\n\nList conversations.\n\nExamples:\n\nchatwoot convs    Yours\n\n" +
		"Flags:\n\n-h, --help      Help.\n    --no-color  Plain.\n                More.\n\n" +
		"Conversations\n\nconvs [flags]\n  List conversations.\n"
	if got := layoutHelp(in); got != want {
		t.Fatalf("layoutHelp() =\n%s\nwant\n%s", got, want)
	}
}

// Real help output: a blank line after every header, and entries flush left
// with their descriptions two spaces in.
func TestHelpSectionLayout(t *testing.T) {
	for _, args := range [][]string{nil, {"conv"}, {"conv", "reply"}, {"accounts"}} {
		lines := strings.Split(helpFor(t, args...), "\n")
		for i, line := range lines[:len(lines)-1] {
			if helpHeader.MatchString(line) && lines[i+1] != "" {
				t.Errorf("chatwoot %v --help: no blank line after header %q", args, line)
			}
		}
		out := strings.Join(lines, "\n")
		for _, bad := range []string{"\n  convs", "\n  -h, --help", "\n    chatwoot convs", "\n    List conversations."} {
			if strings.Contains(out, bad) {
				t.Errorf("chatwoot %v --help still indents %q", args, strings.TrimSpace(bad))
			}
		}
	}
	root := helpFor(t)
	for _, want := range []string{"\nconvs (conversations) [flags]\n  List conversations.", "\n-h, --help ", "\nchatwoot convs "} {
		if !strings.Contains(root, want) {
			t.Errorf("root help missing %q:\n%s", want, root)
		}
	}
}

func TestColorizeHelpPaintsWithoutChangingText(t *testing.T) {
	plain := helpFor(t, "conv", "reply")
	colored := colorizeHelp(plain)

	if !strings.Contains(colored, "\x1b[") {
		t.Fatal("colorizeHelp added no color")
	}
	if got := stripANSI(colored); got != plain {
		t.Fatalf("colorizing changed the text:\n%s", got)
	}
	for _, want := range []string{
		helpStyle.header + "Examples:" + ansiReset,
		helpStyle.header + "Arguments:" + ansiReset,
		helpStyle.placeholder + "<id>" + ansiReset,
		helpStyle.flag + "--private" + ansiReset,
	} {
		if !strings.Contains(colored, want) {
			t.Errorf("colored help missing %q", want)
		}
	}

	root := colorizeHelp(helpFor(t))
	for _, want := range []string{
		helpStyle.header + "Conversations" + ansiReset,
		helpStyle.command + "convs",
		helpStyle.note + "Your open conversations" + ansiReset, // example descriptions
	} {
		if !strings.Contains(root, want) {
			t.Errorf("colored root help missing %q", want)
		}
	}
}

func TestHelpColorOnlyForInteractiveTerminals(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	if !wantHelpColor(true, nil) {
		t.Fatal("a terminal should get color")
	}
	if wantHelpColor(false, nil) {
		t.Fatal("a pipe or file must stay plain")
	}
	if wantHelpColor(true, []string{"convs", "--no-color", "--help"}) {
		t.Fatal("--no-color must turn color off")
	}
	t.Setenv("NO_COLOR", "1")
	if wantHelpColor(true, nil) {
		t.Fatal("NO_COLOR must turn color off")
	}
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "dumb")
	if wantHelpColor(true, nil) {
		t.Fatal("TERM=dumb must stay plain")
	}
}

func stripANSI(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "")
}
