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
	List     HCsCmd        `cmd:"" help:"List help centers."`
	Default  HCDefaultCmd  `cmd:"" help:"Show, set, or clear the default help center."`
	Articles HCArticlesCmd `cmd:"" help:"List or search public help center articles."`
	Article  HCArticleCmd  `cmd:"" help:"Get a public help center article."`
}

type HCDefaultCmd struct {
	Slug  string `arg:"" optional:"" help:"Help center portal slug to set as default."`
	Clear bool   `help:"Clear the configured default help center."`
}

func (c *HCDefaultCmd) Run(app *App) error {
	if c.Clear {
		if app.Config == nil {
			return fmt.Errorf("config is not loaded")
		}
		app.Config.HelpCenter = config.HelpCenterConfig{}
		if err := config.Save(app.Config); err != nil {
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

	if app.Config == nil {
		return fmt.Errorf("config is not loaded")
	}
	app.Config.HelpCenter = config.HelpCenterConfig{
		DefaultPortalSlug: portal.Slug,
		DefaultLocale:     portalDefaultLocale(portal),
	}
	if err := config.Save(app.Config); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(app.Printer.Writer, "Default help center set to %s", portal.Slug)
	if app.Config.HelpCenter.DefaultLocale != "" {
		_, _ = fmt.Fprintf(app.Printer.Writer, " (%s)", app.Config.HelpCenter.DefaultLocale)
	}
	_, _ = fmt.Fprintln(app.Printer.Writer)
	return nil
}

type HCArticlesCmd struct {
	PortalSlug   string `name:"portal" help:"Help center portal slug. Defaults to 'chatwoot hc default'."`
	Locale       string `help:"Article locale, for example en. Defaults to the configured help center locale."`
	CategorySlug string `name:"category" help:"Restrict results to a category slug."`
	Query        string `help:"Search query."`
	Page         int    `short:"p" default:"1" help:"Page number."`
	PerPage      int    `help:"Results per page, capped by Chatwoot at 100."`
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
	PortalSlug  string `name:"portal" help:"Help center portal slug. Defaults to 'chatwoot hc default'."`
	ArticleSlug string `arg:"" help:"Article slug."`
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
	if app.Config == nil ||
		strings.TrimSpace(app.Config.HelpCenter.DefaultPortalSlug) == "" {
		_, _ = fmt.Fprintln(app.Printer.Writer, "No default help center set.")
		return nil
	}

	app.Printer.PrintDetail([]output.KeyValue{
		{Key: "Portal", Value: app.Config.HelpCenter.DefaultPortalSlug},
		{Key: "Locale", Value: app.Config.HelpCenter.DefaultLocale},
	})
	return nil
}

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
	if app.Config != nil && strings.TrimSpace(app.Config.HelpCenter.DefaultPortalSlug) != "" {
		return strings.TrimSpace(app.Config.HelpCenter.DefaultPortalSlug), nil
	}
	return "", fmt.Errorf("no default help center set. Run 'chatwoot hc default <slug>' or pass --portal")
}

func resolveHelpCenterLocale(app *App, explicitLocale string) (string, error) {
	if strings.TrimSpace(explicitLocale) != "" {
		return strings.TrimSpace(explicitLocale), nil
	}
	if app.Config != nil && strings.TrimSpace(app.Config.HelpCenter.DefaultLocale) != "" {
		return strings.TrimSpace(app.Config.HelpCenter.DefaultLocale), nil
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
