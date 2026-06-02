// Package importer is a source-agnostic engine for importing Help Center
// content into Chatwoot. Providers (Intercom today; Crisp/Zendesk later)
// implement the Source interface and map their data into the provider-neutral
// intermediate representation (IR) defined here. The engine then plans and
// executes the writes against Chatwoot via a Sink.
//
// The engine is intentionally non-interactive: all prompting lives in the CLI
// layer, which passes the user's choices in as a Selections value.
package importer

// HelpCenter is a selectable source help center.
type HelpCenter struct {
	ID            string
	Name          string
	DefaultLocale string
}

// Collection is a provider-neutral category/section. Nesting is expressed via
// ParentID. Names/Descriptions are keyed by locale and must include the
// collection's default-locale name.
type Collection struct {
	ID           string
	ParentID     string
	Names        map[string]string
	Descriptions map[string]string
	Order        int
}

// Name returns the best name for a locale, falling back to the default locale
// then any available name.
func (c Collection) Name(locale, defaultLocale string) string {
	if n, ok := c.Names[locale]; ok && n != "" {
		return n
	}
	if n, ok := c.Names[defaultLocale]; ok && n != "" {
		return n
	}
	for _, n := range c.Names {
		if n != "" {
			return n
		}
	}
	return c.ID
}

// Article carries the default-locale body as the root plus per-locale variants.
type Article struct {
	ID            string
	CollectionID  string
	DefaultLocale string
	AuthorID      string
	Variants      map[string]ArticleVariant
	SourceURL     string
}

// ArticleVariant is one localized rendition of an article.
type ArticleVariant struct {
	Locale      string
	Title       string
	Description string
	BodyHTML    string
	AuthorID    string // per-locale author override; falls back to Article.AuthorID
	State       string // provider state, kept for reporting; imports always write draft
}

// Author is a provider teammate/admin used for author matching.
type Author struct {
	ID    string
	Email string
	Name  string
}

// Corpus is the fully-scanned source graph for one help center.
type Corpus struct {
	HelpCenter  HelpCenter
	Collections []Collection
	Articles    []Article
	Authors     map[string]Author
	Locales     []string // derived during scan; default locale first
}

// PortalTarget describes the chosen Chatwoot portal: either an existing slug
// or a new portal to create.
type PortalTarget struct {
	ExistingSlug string
	CreateName   string
	CreateSlug   string
}

// IsCreate reports whether a new portal should be created.
func (t PortalTarget) IsCreate() bool { return t.ExistingSlug == "" }

// Slug returns the portal slug (existing or to-be-created).
func (t PortalTarget) Slug() string {
	if t.ExistingSlug != "" {
		return t.ExistingSlug
	}
	return t.CreateSlug
}

// Selections is the user's interactive choices, built by the CLI layer and
// passed to Plan.
type Selections struct {
	SourceHCID string
	Target     PortalTarget
	Locales    []string // locales chosen for import; gates non-root variants
}

// ---------------------------------------------------------------------------
// Plan — the resolved, ordered set of writes Execute will perform. Built by
// Plan() with no network calls (author resolution uses pre-fetched maps).
// ---------------------------------------------------------------------------

// PlannedCategory is one category create (a collection in a specific locale).
type PlannedCategory struct {
	CollectionID       string
	ParentCollectionID string
	Locale             string
	IsRoot             bool // root = the collection's default-locale category
	Name               string
	Description        string
	Slug               string
	Skip               bool // already recorded in state
}

// PlannedArticle is one article create (an article variant in a locale).
type PlannedArticle struct {
	ArticleID    string
	CollectionID string
	Locale       string
	RootLocale   string // the article's default locale (the root variant's locale)
	IsRoot       bool   // root = the article's default-locale variant
	Title        string
	Description  string
	BodyHTML     string // raw; Execute transforms images/embeds before writing
	AuthorID     int    // resolved Chatwoot user id
	Skip         bool
}

// ImportPlan is the full ordered plan plus summary metadata.
type ImportPlan struct {
	Portal        PortalTarget
	DefaultLocale string   // corpus default locale (the per-collection root locale)
	PortalLocales []string // effective allowed locales to ensure on the portal
	Categories    []PlannedCategory
	Articles      []PlannedArticle
	AuthorStats   AuthorStats
}

// AuthorStats summarizes author matching for the plan/summary.
type AuthorStats struct {
	Matched  int
	Fallback int
}

// Result is the outcome of Execute, for the final summary.
type Result struct {
	PortalSlug         string
	PortalCreated      bool
	CategoriesCreated  int
	CategoriesSkipped  int
	ArticlesCreated    int
	ArticlesSkipped    int
	ImagesSwapped      int
	ImagesFailed       int
	EmbedsRewritten    int
	UncategorizedCount int
	Failures           []FailureRec
	AuthorStats        AuthorStats
}
