package main

import (
	"bytes"
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
		"auth login":  {"Examples:", "chatwoot auth login staging.chatwoot.com"},
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
