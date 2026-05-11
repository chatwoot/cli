package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/chatwoot/cli/internal/cmd"
	"github.com/chatwoot/cli/internal/update"
	kongcompletion "github.com/jotaen/kong-completion"
	"golang.org/x/term"
)

// updateWait caps how long main blocks at exit waiting for the background
// release-check to finish. The fetch was started in parallel with the
// command, so this is mostly a tail latency for unlucky slow networks.
const updateWait = 1500 * time.Millisecond

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

	helpVerbSwap = regexp.MustCompile(`\b(view|messages|reply|resolve|open|pending|snooze|assign|unassign|label|priority|contact|conversations)\s+<id>`)
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"--help"}
	}
	args = rewriteIDFirstGrammar(args)

	var cli cmd.CLI
	parser := kong.Must(&cli,
		kong.Name("chatwoot"),
		kong.Description("CLI for Chatwoot."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.Help(idFirstHelpPrinter),
	)

	// Enable shell completions (must be called before Parse)
	kongcompletion.Register(parser)

	ctx, err := parser.Parse(args)
	parser.FatalIfErrorf(err)

	// Commands that don't require authentication. `me` and `whoami` are
	// aliases of `auth status` — they load config themselves and report
	// "not logged in" gracefully.
	cmdStr := ctx.Command()
	skipAuth := strings.HasPrefix(cmdStr, "auth") ||
		strings.HasPrefix(cmdStr, "config") ||
		strings.HasPrefix(cmdStr, "completion") ||
		cmdStr == "me" ||
		cmdStr == "whoami" ||
		cmdStr == "version"

	// Start the outdated-version check in the background so its network
	// round-trip overlaps with the command. shouldShowNotice gates both
	// the fetch and the eventual notice — there's no point fetching for
	// quiet/JSON/CSV runs where we'd never print anything.
	notice := shouldShowNotice(&cli, cmdStr, version)
	var waitRefresh = func() {}
	if notice {
		waitRefresh = update.StartRefresh(updateWait)
	}

	app, err := cmd.NewApp(&cli, skipAuth, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	runErr := ctx.Run(app)

	if notice {
		waitRefresh()
		printOutdatedNotice(os.Stderr, version, !cli.NoColor && term.IsTerminal(int(os.Stderr.Fd())))
	}

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", runErr)
		os.Exit(1)
	}
}

// shouldShowNotice decides whether the outdated-version banner is worth
// running for this invocation. We bail out for machine-readable output,
// quiet mode, dev builds, and commands where a banner would be noise
// (the version command does its own --check, completion is shell-eval'd).
func shouldShowNotice(cli *cmd.CLI, cmdStr, version string) bool {
	if cli.Quiet || cli.Output != "text" {
		return false
	}
	if version == "" || version == "dev" {
		return false
	}
	if strings.HasPrefix(cmdStr, "version") || strings.HasPrefix(cmdStr, "completion") {
		return false
	}
	return true
}

// printOutdatedNotice writes a short upgrade hint when the cached latest
// tag is newer than the running version. Failures (missing cache, fetch
// never completed, parse error) silently skip the notice — this is a
// nice-to-have, not a correctness path. color asks for ANSI dim styling
// and should only be true when w is an interactive terminal that hasn't
// opted out via --no-color.
func printOutdatedNotice(w io.Writer, current string, color bool) {
	cache, err := update.LoadCache()
	if err != nil || cache == nil {
		return
	}
	if !update.IsOutdated(current, cache.LatestVersion) {
		return
	}
	fmt.Fprint(w, update.FormatNotice(current, cache.LatestVersion, color))
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
