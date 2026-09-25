package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/chatwoot/cli/internal/cmd"
	"github.com/chatwoot/cli/internal/config"
	kongcompletion "github.com/jotaen/kong-completion"
	"golang.org/x/term"
)

var version = "dev"

// id-first dispatch: users say `chatwoot conv 123 reply "hi"` but Kong
// can't have arg:"" siblings to cmd:"". rewriteIDFirstGrammar swaps the id
// and verb tokens so Kong sees its preferred verb-first form.
var (
	convVerbs    = []string{"view", "messages", "reply", "resolve", "open", "pending", "snooze", "assign", "unassign", "label", "priority", "contact"}
	contactVerbs = []string{"view", "conversations"}
	inboxVerbs   = []string{"view"}

	contextNouns = map[string][]string{
		"conv":         convVerbs,
		"conversation": convVerbs,
		"contact":      contactVerbs,
		"inbox":        inboxVerbs,
	}

	// valueFlags are global flags whose value is the next token, so the arg
	// rewriters must skip both when looking for the noun.
	valueFlags = []string{"-o", "--output", "-a", "--account"}

	helpVerbSwap = regexp.MustCompile(`\b(view|messages|reply|resolve|open|pending|snooze|assign|unassign|label|priority|contact|conversations)\s+<id>`)
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"--help"}
	}
	args = normalizeArgs(args)

	var cli cmd.CLI
	parser, err := newParser(&cli)
	if err != nil {
		panic(err)
	}

	// Enable shell completions (must be called before Parse)
	kongcompletion.Register(parser)

	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	// Commands that don't require authentication. `me` and `whoami` are
	// aliases of `auth status` — they load config themselves and report
	// "not logged in" gracefully.
	cmdStr := ctx.Command()
	skipAuth := strings.HasPrefix(cmdStr, "auth") ||
		strings.HasPrefix(cmdStr, "accounts") ||
		strings.HasPrefix(cmdStr, "use") ||
		strings.HasPrefix(cmdStr, "config") ||
		strings.HasPrefix(cmdStr, "completion") ||
		cmdStr == "me" ||
		cmdStr == "whoami" ||
		cmdStr == "version"

	app, err := cmd.NewApp(&cli, skipAuth, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if notice := cmd.TargetNotice(app, cmdStr, &cli); notice != "" {
		fmt.Fprintln(os.Stderr, notice)
	}

	if err := ctx.Run(app); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", cmd.ExplainError(app, err))
		os.Exit(1)
	}

	// Notices go to stderr and only to a person at a terminal, never into
	// --json/--quiet output or a pipe.
	interactive := cli.Output == "text" && !cli.Quiet && term.IsTerminal(int(os.Stderr.Fd()))
	app.Finish(os.Stderr, interactive)
}

func newParser(cli *cmd.CLI, extra ...kong.Option) (*kong.Kong, error) {
	options := []kong.Option{
		kong.Name("chatwoot"),
		kong.Description("Work with your Chatwoot inbox from the terminal."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.Help(idFirstHelpPrinter),
		// Top-level help lists commands, not every verb; `chatwoot conv --help`
		// shows the verbs.
		kong.ConfigureHelp(kong.HelpOptions{NoExpandSubcommands: true}),
	}
	return kong.New(cli, append(options, extra...)...)
}

// normalizeArgs turns the user-facing grammar into what Kong parses.
func normalizeArgs(args []string) []string {
	return rewriteIDFirstGrammar(rewriteAccountShorthand(rewriteLink(args)))
}

// rewriteLink lets a pasted dashboard link stand in for the noun and id:
// `chatwoot <conversation link> reply "hi"` becomes
// `chatwoot --account=<its account> conv <id> reply "hi"`. The link names its
// account, so it replaces any other account selector. A link to another page
// only selects the account (listing conversations when nothing follows).
func rewriteLink(args []string) []string {
	i := nounIndex(args)
	j := i
	if j < len(args) && len(args[j]) > 1 && strings.HasPrefix(args[j], "@") {
		j++
	}
	if j >= len(args) {
		return args
	}
	link, ok := config.ParseLink(args[j])
	if !ok {
		return args
	}

	out := make([]string, 0, len(args)+2)
	for k := 0; k < i; k++ {
		switch {
		case args[k] == "-a" || args[k] == "--account":
			k++ // drop the flag and its value
		case strings.HasPrefix(args[k], "--account="):
		default:
			out = append(out, args[k])
		}
	}
	out = append(out, "--account="+link.AccountSelector())

	rest := args[j+1:]
	switch {
	case link.Noun != "":
		out = append(out, link.Noun, strconv.Itoa(link.ID))
	case len(rest) == 0:
		out = append(out, "convs")
	}
	return append(out, rest...)
}

// nounIndex returns the index of the first token that is not a global flag or
// a global flag's value.
func nounIndex(args []string) int {
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		if slices.Contains(valueFlags, args[i]) {
			i++
		}
		i++
	}
	return i
}

// rewriteAccountShorthand turns a leading `@name` into `--account=name`. Only
// the token where the noun would start counts, so `@` inside message text
// (`conv 1 reply "@john hi"`) is never treated as an account.
func rewriteAccountShorthand(args []string) []string {
	i := nounIndex(args)
	if i >= len(args) || len(args[i]) < 2 || !strings.HasPrefix(args[i], "@") {
		return args
	}
	out := slices.Clone(args)
	out[i] = "--account=" + strings.TrimPrefix(args[i], "@")
	return out
}

// rewriteIDFirstGrammar swaps `<noun> <id> <verb>` to `<noun> <verb> <id>`
// when the args match a known context-noun grammar. Other shapes pass through
// unchanged, so verb-first input still works.
func rewriteIDFirstGrammar(args []string) []string {
	i := nounIndex(args)
	if i >= len(args) {
		return args
	}
	verbs, ok := contextNouns[args[i]]
	if !ok || i+2 >= len(args) {
		return args
	}
	if _, err := strconv.Atoi(args[i+1]); err != nil {
		return args
	}
	if !slices.Contains(verbs, args[i+2]) {
		return args
	}
	out := slices.Clone(args)
	out[i+1], out[i+2] = out[i+2], out[i+1]
	return out
}

// idFirstHelpPrinter runs Kong's default printer into a buffer, then swaps
// `<verb> <id>` → `<id> <verb>` so help reads in the user-facing order.
func idFirstHelpPrinter(o kong.HelpOptions, ctx *kong.Context) error {
	var buf bytes.Buffer
	orig := ctx.Stdout
	ctx.Stdout = &buf
	err := kong.DefaultHelpPrinter(o, ctx)
	ctx.Stdout = orig
	if err != nil {
		return err
	}
	_, werr := fmt.Fprint(orig, helpVerbSwap.ReplaceAllString(buf.String(), `<id> $1`))
	return werr
}
