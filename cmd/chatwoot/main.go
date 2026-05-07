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
	"github.com/chatwoot/cli/internal/sdk"
	"github.com/chatwoot/cli/internal/tui"
	"github.com/willabides/kongplete"
)

var version = "dev"

// id-first dispatch: users say `chatwoot conv 123 reply "hi"` but Kong
// can't have arg:"" siblings to cmd:"". rewriteIDFirstGrammar swaps the id
// and verb tokens so Kong sees its preferred verb-first form.
var (
	convVerbs    = []string{"view", "messages", "reply", "resolve", "open", "pending", "snooze", "assign", "unassign", "label", "priority"}
	contactVerbs = []string{"view", "conversations"}
	inboxVerbs   = []string{"view"}

	contextNouns = map[string][]string{
		"conv":         convVerbs,
		"conversation": convVerbs,
		"contact":      contactVerbs,
		"inbox":        inboxVerbs,
	}

	helpVerbSwap = regexp.MustCompile(`\b(view|messages|reply|resolve|open|pending|snooze|assign|unassign|label|priority|conversations)\s+<id>`)
)

func main() {
	// No args → launch interactive TUI
	if len(os.Args) == 1 {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if cfg == nil || !cfg.IsValid() {
			fmt.Fprintln(os.Stderr, "Not authenticated. Run: chatwoot auth login")
			os.Exit(1)
		}
		apiKey, _, err := config.ResolveAPIKey(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: not authenticated: %v\n", err)
			os.Exit(1)
		}
		client := sdk.NewClient(cfg.BaseURL, apiKey, cfg.AccountID)
		if err := tui.Run(client, cfg.AccountID, version); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	args := rewriteIDFirstGrammar(os.Args[1:])

	var cli cmd.CLI
	parser := kong.Must(&cli,
		kong.Name("chatwoot"),
		kong.Description("CLI and interactive TUI for Chatwoot."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.Help(idFirstHelpPrinter),
	)

	// Enable shell completions (must be called before Parse)
	kongplete.Complete(parser)

	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	// Commands that don't require authentication
	cmdStr := ctx.Command()
	skipAuth := strings.HasPrefix(cmdStr, "auth") ||
		strings.HasPrefix(cmdStr, "config") ||
		strings.HasPrefix(cmdStr, "install-completions")

	app, err := cmd.NewApp(&cli, skipAuth)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if err := ctx.Run(app); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// rewriteIDFirstGrammar swaps `<noun> <id> <verb>` to `<noun> <verb> <id>`
// when the args match a known context-noun grammar. Other shapes pass through
// unchanged, so verb-first input still works.
func rewriteIDFirstGrammar(args []string) []string {
	i := 0
	for i < len(args) && strings.HasPrefix(args[i], "-") {
		i++
	}
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
