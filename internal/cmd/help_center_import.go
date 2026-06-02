package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/importer"
	"github.com/chatwoot/cli/internal/importer/intercom"
	"github.com/chatwoot/cli/internal/prompt"
)

// HCImportCmd groups per-provider import subcommands. Adding a provider later
// (Crisp, Zendesk) is a new subcommand here plus a new importer.Source.
type HCImportCmd struct {
	Intercom HCImportIntercomCmd `cmd:"" help:"Import help center content from Intercom."`
}

// HCImportIntercomCmd is `chatwoot hc import intercom` — an interactive importer
// that pulls collections/articles from Intercom into a Chatwoot portal.
type HCImportIntercomCmd struct {
	Token   string `required:"" help:"Intercom access token (sent as a Bearer token)."`
	BaseURL string `name:"base-url" default:"https://api.intercom.io" help:"Intercom API base URL."`
}

// portalOption is an existing Chatwoot portal offered as a target.
type portalOption struct {
	Name string
	Slug string
}

// intercomImportDeps are the injected dependencies of the import orchestration,
// so it can be driven by fakes in tests (no TTY, no network).
type intercomImportDeps struct {
	source          importer.Source
	prompter        prompt.Prompter
	sink            importer.Sink
	existingPortals []portalOption
	agentEmails     map[string]int
	fallbackAuthor  int
	stateDir        string
	out             io.Writer
}

// Run wires real dependencies (TTY check, Intercom client, term prompter,
// Chatwoot sink, agents, current user) and runs the import.
func (c *HCImportIntercomCmd) Run(app *App) error {
	if !prompt.IsInteractive() {
		return fmt.Errorf("chatwoot hc import is interactive and requires a terminal (stdin is not a TTY)")
	}
	if app.Client == nil {
		return fmt.Errorf("not authenticated. Run 'chatwoot auth login'")
	}

	w := app.Printer.Writer

	// Fallback author = the token owner (admin). Articles require an author.
	fallbackAuthor, err := resolveAgent(app, "me")
	if err != nil {
		return fmt.Errorf("cannot resolve the current user as the fallback author: %w", err)
	}
	if fallbackAuthor == 0 {
		return fmt.Errorf("cannot determine the current user id; article author is required")
	}

	agentEmails, err := chatwootAgentEmails(app)
	if err != nil {
		return err
	}

	portals, err := app.Client.HelpCenter().ListPortals()
	if err != nil {
		return fmt.Errorf("failed to list Chatwoot portals: %w", err)
	}
	existing := make([]portalOption, 0, len(portals.Payload))
	for _, p := range portals.Payload {
		existing = append(existing, portalOption{Name: p.Name, Slug: p.Slug})
	}

	dir, err := config.ConfigDir()
	if err != nil {
		return err
	}

	deps := intercomImportDeps{
		source:          intercom.New(c.BaseURL, c.Token),
		prompter:        prompt.NewTermPrompter(os.Stdin, w),
		sink:            importer.NewChatwootSink(app.Client),
		existingPortals: existing,
		agentEmails:     agentEmails,
		fallbackAuthor:  fallbackAuthor,
		stateDir:        filepath.Join(dir, "imports"),
		out:             w,
	}

	_, err = runIntercomImport(context.Background(), deps)
	return err
}

// runIntercomImport is the testable orchestration: validate → select source →
// select target → scan → select locales → plan → confirm → execute.
func runIntercomImport(ctx context.Context, deps intercomImportDeps) (*importer.Result, error) {
	w := deps.out

	fmt.Fprintln(w, "chatwoot hc import · Intercom → Chatwoot")
	workspaceID, err := deps.source.Validate(ctx)
	if err != nil {
		return nil, fmt.Errorf("intercom authentication failed: %w", err)
	}
	fmt.Fprintf(w, "✓ Intercom token valid (workspace %s)\n", workspaceID)

	sourceHC, err := selectSourceHelpCenter(ctx, deps.prompter, w, deps.source)
	if err != nil {
		return nil, err
	}

	target, err := selectTargetPortal(deps.prompter, w, deps.existingPortals, sourceHC.Name)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(w, "Scanning %q…\n", sourceHC.Name)
	corpus, err := deps.source.Scan(ctx, sourceHC.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to scan source help center: %w", err)
	}
	fmt.Fprintf(w, "✓ Found %d collections, %d articles, locales: %s\n",
		len(corpus.Collections), len(corpus.Articles), strings.Join(corpus.Locales, ", "))

	locales, err := selectLocales(deps.prompter, corpus.Locales)
	if err != nil {
		return nil, err
	}

	resolver := importer.NewAuthorResolver(corpus.Authors, deps.agentEmails, deps.fallbackAuthor)

	key := importer.StateKey(deps.source.Name(), workspaceID, sourceHC.ID, target.Slug())
	statePath := importer.StatePath(deps.stateDir, key)
	st, err := importer.LoadState(statePath)
	if err != nil {
		return nil, err
	}
	if st == nil {
		st = importer.NewState(deps.source.Name(), workspaceID, sourceHC.ID)
	}

	sel := importer.Selections{SourceHCID: sourceHC.ID, Target: target, Locales: locales}
	plan := importer.Plan(corpus, sel, resolver, st)

	renderPlanSummary(w, plan, target)
	ok, err := deps.prompter.Confirm("Proceed?")
	if err != nil {
		return nil, err
	}
	if !ok {
		fmt.Fprintln(w, "Aborted. Nothing was written.")
		return nil, nil
	}

	res, err := importer.Execute(ctx, plan, importer.ExecuteOptions{
		Sink:      deps.sink,
		State:     st,
		StatePath: statePath,
		Embeds:    importer.DefaultEmbedRegistry(),
		Log:       func(msg string) { fmt.Fprintf(w, "  • %s\n", msg) },
	})
	if err != nil {
		return nil, err
	}

	renderImportResult(w, res)
	return res, nil
}

func selectSourceHelpCenter(ctx context.Context, pr prompt.Prompter, w io.Writer, src importer.Source) (importer.HelpCenter, error) {
	hcs, err := src.ListHelpCenters(ctx)
	if err != nil {
		return importer.HelpCenter{}, fmt.Errorf("failed to list source help centers: %w", err)
	}
	switch len(hcs) {
	case 0:
		return importer.HelpCenter{}, fmt.Errorf("no help centers found in the source workspace")
	case 1:
		fmt.Fprintf(w, "✓ Source help center: %s\n", hcs[0].Name)
		return hcs[0], nil
	default:
		labels := make([]string, len(hcs))
		for i, hc := range hcs {
			labels[i] = hc.Name
		}
		idx, err := pr.SelectOne("Source help center", labels)
		if err != nil {
			return importer.HelpCenter{}, err
		}
		return hcs[idx], nil
	}
}

func selectTargetPortal(pr prompt.Prompter, w io.Writer, existing []portalOption, sourceName string) (importer.PortalTarget, error) {
	labels := []string{fmt.Sprintf("Create a new portal from %q", sourceName)}
	for _, p := range existing {
		labels = append(labels, fmt.Sprintf("%s (%s)", p.Name, p.Slug))
	}

	idx, err := pr.SelectOne("Target portal in Chatwoot", labels)
	if err != nil {
		return importer.PortalTarget{}, err
	}
	if idx == 0 {
		name, err := pr.Input("New portal name", sourceName)
		if err != nil {
			return importer.PortalTarget{}, err
		}
		slug, err := pr.Input("New portal slug", importer.Slugify(name))
		if err != nil {
			return importer.PortalTarget{}, err
		}
		return importer.PortalTarget{CreateName: name, CreateSlug: importer.Slugify(slug)}, nil
	}
	return importer.PortalTarget{ExistingSlug: existing[idx-1].Slug}, nil
}

func selectLocales(pr prompt.Prompter, available []string) ([]string, error) {
	if len(available) <= 1 {
		return available, nil
	}
	idxs, err := pr.SelectMany("Locales to import", available, true)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(idxs))
	for _, i := range idxs {
		out = append(out, available[i])
	}
	return out, nil
}

func chatwootAgentEmails(app *App) (map[string]int, error) {
	agents, err := app.Client.Agents().List()
	if err != nil {
		return nil, fmt.Errorf("failed to list Chatwoot agents: %w", err)
	}
	byEmail := make(map[string]int, len(agents))
	for _, a := range agents {
		if a.Email != "" {
			byEmail[strings.ToLower(a.Email)] = a.ID
		}
	}
	return byEmail, nil
}

func renderPlanSummary(w io.Writer, plan *importer.ImportPlan, target importer.PortalTarget) {
	catCreate, catSkip := countPlanned(len(plan.Categories), func(i int) bool { return plan.Categories[i].Skip })
	artCreate, artSkip := countPlanned(len(plan.Articles), func(i int) bool { return plan.Articles[i].Skip })

	portalDesc := fmt.Sprintf("existing %q", target.Slug())
	if target.IsCreate() {
		portalDesc = fmt.Sprintf("create %q (%s)", target.CreateName, target.CreateSlug)
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Plan")
	fmt.Fprintf(w, "  Portal      %s\n", portalDesc)
	fmt.Fprintf(w, "  Locales     %s\n", strings.Join(plan.PortalLocales, ", "))
	fmt.Fprintf(w, "  Categories  %d to create, %d already imported\n", catCreate, catSkip)
	fmt.Fprintf(w, "  Articles    %d to create, %d already imported (all draft)\n", artCreate, artSkip)
	fmt.Fprintf(w, "  Authors     %d matched by email, %d fall back to current user\n", plan.AuthorStats.Matched, plan.AuthorStats.Fallback)
	fmt.Fprintln(w, "  Images & embeds: best effort")
	fmt.Fprintln(w, "")
}

func countPlanned(n int, isSkip func(int) bool) (create, skip int) {
	for i := 0; i < n; i++ {
		if isSkip(i) {
			skip++
		} else {
			create++
		}
	}
	return create, skip
}

func renderImportResult(w io.Writer, res *importer.Result) {
	portalState := "existing"
	if res.PortalCreated {
		portalState = "created"
	}

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Done.")
	fmt.Fprintf(w, "  Portal       %s (%s)\n", res.PortalSlug, portalState)
	fmt.Fprintf(w, "  Categories   %d created, %d skipped\n", res.CategoriesCreated, res.CategoriesSkipped)
	fmt.Fprintf(w, "  Articles     %d created, %d skipped, %d failed\n", res.ArticlesCreated, res.ArticlesSkipped, len(res.Failures))
	fmt.Fprintf(w, "  Images       %d re-hosted, %d failed\n", res.ImagesSwapped, res.ImagesFailed)
	fmt.Fprintf(w, "  Embeds       %d converted\n", res.EmbedsRewritten)
	fmt.Fprintf(w, "  Authors      %d matched, %d fell back to current user\n", res.AuthorStats.Matched, res.AuthorStats.Fallback)
	fmt.Fprintln(w, "  Status       imported as draft — review & publish in Chatwoot")

	if res.UncategorizedCount > 0 {
		fmt.Fprintf(w, "\n  ⚠ %d article(s) were imported uncategorized because their category failed to create.\n", res.UncategorizedCount)
	}

	if len(res.Failures) > 0 {
		fmt.Fprintf(w, "\n  ⚠ %d item(s) failed:\n", len(res.Failures))
		limit := len(res.Failures)
		if limit > 20 {
			limit = 20
		}
		for _, f := range res.Failures[:limit] {
			fmt.Fprintf(w, "    - [%s] %s\n", f.Kind, f.Error)
		}
		if len(res.Failures) > 20 {
			fmt.Fprintf(w, "    … and %d more (see the import state file)\n", len(res.Failures)-20)
		}
		fmt.Fprintln(w, "  Re-run the same command to retry failed items.")
	}
}
