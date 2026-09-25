package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/output"
	"github.com/chatwoot/cli/internal/sdk"
)

// HCsCmd is `chatwoot hcs` — list help center portals on the account.
type HCsCmd struct{}

func (c *HCsCmd) Run(app *App) error {
	resp, err := app.Client.HelpCenter().ListPortals()
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No help centers found.")
		return nil
	}

	headers := []string{"ID", "Name", "Slug", "Default Locale", "Locales", "Articles", "Categories"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, portal := range resp.Payload {
		rows = append(rows, []string{
			strconv.Itoa(portal.ID),
			portal.Name,
			portal.Slug,
			portalDefaultLocale(portal),
			portalLocales(portal.Config.AllowedLocales),
			strconv.Itoa(portalArticlesCount(portal)),
			strconv.Itoa(portal.Meta.CategoriesCount),
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

type HCCmd struct {
	List     HCsCmd        `cmd:"" help:"List help centers (same as 'chatwoot hcs')."`
	Default  HCDefaultCmd  `cmd:"" help:"Choose which help center to search by default, or show the current one."`
	Articles HCArticlesCmd `cmd:"" help:"List or search published articles."`
	Article  HCArticleCmd  `cmd:"" help:"Read one article."`
}

type HCDefaultCmd struct {
	Slug  string `arg:"" optional:"" help:"The help center's slug (see 'chatwoot hcs'). Leave out to see the current default."`
	Clear bool   `help:"Forget the default help center."`
}

func (c *HCDefaultCmd) Run(app *App) error {
	if c.Clear {
		if !app.registered() {
			return errUnregisteredAccount
		}
		app.Account.HelpCenter = config.HelpCenterConfig{}
		if err := saveConfig(app.Config); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(app.Printer.Writer, "Cleared default help center.")
		return nil
	}

	if strings.TrimSpace(c.Slug) == "" {
		return renderHelpCenterDefault(app)
	}

	portal, err := findHelpCenterPortal(app, c.Slug)
	if err != nil {
		return err
	}

	if !app.registered() {
		return errUnregisteredAccount
	}
	app.Account.HelpCenter = config.HelpCenterConfig{
		DefaultPortalSlug: portal.Slug,
		DefaultLocale:     portalDefaultLocale(portal),
	}
	if err := saveConfig(app.Config); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(app.Printer.Writer, "Default help center set to %s", portal.Slug)
	if app.Account.HelpCenter.DefaultLocale != "" {
		_, _ = fmt.Fprintf(app.Printer.Writer, " (%s)", app.Account.HelpCenter.DefaultLocale)
	}
	_, _ = fmt.Fprintln(app.Printer.Writer)
	return nil
}

type HCArticlesCmd struct {
	PortalSlug   string `name:"portal" help:"Which help center to search. Uses your default if you leave it out."`
	Locale       string `help:"Which language, like en or fr. Uses your default help center's language if you leave it out."`
	CategorySlug string `name:"category" help:"Only articles in this category."`
	Query        string `help:"Words to search for."`
	Page         int    `short:"p" default:"1" help:"Which page of results to show."`
	PerPage      int    `help:"How many articles per page (up to 100)."`
}

func (c *HCArticlesCmd) Help() string {
	return `Set a default help center first with 'chatwoot hc default <slug>', or pass
--portal and --locale each time.

Examples:
  chatwoot hc articles --query "api channel"
  chatwoot hc articles --category getting-started
  chatwoot hc articles --portal docs --locale fr`
}

func (c *HCArticlesCmd) Run(app *App) error {
	portalSlug, err := resolveHelpCenterPortal(app, c.PortalSlug)
	if err != nil {
		return err
	}
	locale, err := resolveHelpCenterLocale(app, c.Locale)
	if err != nil {
		return err
	}

	resp, err := app.Client.HelpCenter().ListArticles(sdk.HelpCenterArticlesOptions{
		PortalSlug:   portalSlug,
		Locale:       locale,
		CategorySlug: c.CategorySlug,
		Query:        c.Query,
		Page:         c.Page,
		PerPage:      c.PerPage,
	})
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(resp)
		return nil
	}

	if len(resp.Payload) == 0 {
		fmt.Println("No articles found.")
		return nil
	}

	headers := []string{"ID", "Title", "Category", "Slug", "Link", "Snippet"}
	rows := make([][]string, 0, len(resp.Payload))
	for _, article := range resp.Payload {
		rows = append(rows, []string{
			strconv.Itoa(article.ID),
			article.Title,
			articleCategory(article),
			article.Slug,
			article.Link,
			articleSnippet(article),
		})
	}

	app.Printer.PrintTable(headers, rows)
	return nil
}

type HCArticleCmd struct {
	PortalSlug  string `name:"portal" help:"Which help center it's in. Uses your default if you leave it out."`
	ArticleSlug string `arg:"" help:"The article's slug, from 'chatwoot hc articles'."`
}

func (c *HCArticleCmd) Run(app *App) error {
	portalSlug, err := resolveHelpCenterPortal(app, c.PortalSlug)
	if err != nil {
		return err
	}

	article, err := app.Client.HelpCenter().GetArticle(portalSlug, c.ArticleSlug)
	if err != nil {
		return err
	}

	if app.Printer.Format == "json" && !app.Printer.Quiet {
		app.Printer.PrintJSON(article)
		return nil
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "ID", Value: strconv.Itoa(article.ID)},
		{Key: "Title", Value: article.Title},
		{Key: "Slug", Value: article.Slug},
		{Key: "Category", Value: articleCategory(*article)},
		{Key: "Views", Value: strconv.Itoa(article.Views)},
		{Key: "Link", Value: article.Link},
		{Key: "Description", Value: article.Description},
		{Key: "Content", Value: truncate(strings.TrimSpace(article.Content), 500)},
	})
	return nil
}

func renderHelpCenterDefault(app *App) error {
	if app.Account == nil ||
		strings.TrimSpace(app.Account.HelpCenter.DefaultPortalSlug) == "" {
		_, _ = fmt.Fprintln(app.Printer.Writer, "No default help center set.")
		return nil
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Portal", Value: app.Account.HelpCenter.DefaultPortalSlug},
		{Key: "Locale", Value: app.Account.HelpCenter.DefaultLocale},
	})
	return nil
}

// errUnregisteredAccount is returned when saving a per-account setting for an
// ad-hoc `-a <id>` account that is not in the config.
var errUnregisteredAccount = fmt.Errorf("this account is not registered; run 'chatwoot accounts --refresh' first")

func findHelpCenterPortal(app *App, slug string) (sdk.HelpCenterPortal, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return sdk.HelpCenterPortal{}, fmt.Errorf("help center slug is required")
	}

	resp, err := app.Client.HelpCenter().ListPortals()
	if err != nil {
		return sdk.HelpCenterPortal{}, err
	}
	for _, portal := range resp.Payload {
		if portal.Slug == slug {
			return portal, nil
		}
	}
	return sdk.HelpCenterPortal{}, fmt.Errorf("no help center matched %q", slug)
}

func resolveHelpCenterPortal(app *App, explicitPortal string) (string, error) {
	if strings.TrimSpace(explicitPortal) != "" {
		return strings.TrimSpace(explicitPortal), nil
	}
	if app.Account != nil && strings.TrimSpace(app.Account.HelpCenter.DefaultPortalSlug) != "" {
		return strings.TrimSpace(app.Account.HelpCenter.DefaultPortalSlug), nil
	}
	return "", fmt.Errorf("no default help center set. Run 'chatwoot hc default <slug>' or pass --portal")
}

func resolveHelpCenterLocale(app *App, explicitLocale string) (string, error) {
	if strings.TrimSpace(explicitLocale) != "" {
		return strings.TrimSpace(explicitLocale), nil
	}
	if app.Account != nil && strings.TrimSpace(app.Account.HelpCenter.DefaultLocale) != "" {
		return strings.TrimSpace(app.Account.HelpCenter.DefaultLocale), nil
	}
	return "", fmt.Errorf("no default help center locale set. Pass --locale or reset the default with 'chatwoot hc default <slug>'")
}

func portalDefaultLocale(portal sdk.HelpCenterPortal) string {
	if portal.Config.DefaultLocale != "" {
		return portal.Config.DefaultLocale
	}
	return portal.Meta.DefaultLocale
}

func portalLocales(locales []sdk.HelpCenterPortalLocale) string {
	if len(locales) == 0 {
		return ""
	}
	codes := make([]string, 0, len(locales))
	for _, locale := range locales {
		codes = append(codes, locale.Code)
	}
	return strings.Join(codes, ", ")
}

func portalArticlesCount(portal sdk.HelpCenterPortal) int {
	if portal.Meta.PublishedCount > 0 {
		return portal.Meta.PublishedCount
	}
	return portal.Meta.AllArticlesCount
}

func articleCategory(article sdk.HelpCenterArticle) string {
	if article.Category != nil && article.Category.Slug != "" {
		return article.Category.Slug
	}
	if article.CategoryID > 0 {
		return strconv.Itoa(article.CategoryID)
	}
	return ""
}

func articleSnippet(article sdk.HelpCenterArticle) string {
	if article.Description != "" {
		return truncate(strings.TrimSpace(article.Description), 120)
	}
	return truncate(strings.TrimSpace(article.Content), 120)
}
