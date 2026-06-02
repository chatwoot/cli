package intercom

import (
	"context"
	"sort"

	"github.com/chatwoot/cli/internal/importer"
)

// Source adapts the Intercom client to the importer.Source interface.
type Source struct {
	client *Client
}

// New builds an Intercom Source.
func New(baseURL, token string, opts ...Option) *Source {
	return &Source{client: NewClient(baseURL, token, opts...)}
}

// Name returns the stable provider key.
func (s *Source) Name() string { return "intercom" }

// Validate confirms credentials and returns the workspace id.
func (s *Source) Validate(ctx context.Context) (string, error) {
	me, err := s.client.Me(ctx)
	if err != nil {
		return "", err
	}
	if me.App.IDCode != "" {
		return me.App.IDCode, nil
	}
	return me.ID.String(), nil
}

// ListHelpCenters returns the workspace's help centers.
func (s *Source) ListHelpCenters(ctx context.Context) ([]importer.HelpCenter, error) {
	hcs, err := s.client.ListHelpCenters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]importer.HelpCenter, 0, len(hcs))
	for _, hc := range hcs {
		name := hc.DisplayName
		if name == "" {
			name = hc.Identifier
		}
		out = append(out, importer.HelpCenter{ID: hc.ID.String(), Name: name})
	}
	return out, nil
}

// Scan pulls the full IR graph for one help center.
func (s *Source) Scan(ctx context.Context, helpCenterID string) (*importer.Corpus, error) {
	collections, err := s.client.ListCollections(ctx)
	if err != nil {
		return nil, err
	}
	articles, err := s.client.ListArticles(ctx)
	if err != nil {
		return nil, err
	}
	admins, err := s.client.ListAdmins(ctx)
	if err != nil {
		return nil, err
	}

	// Collections belonging to the chosen help center.
	hcCollections := make([]collection, 0, len(collections))
	collectionSet := make(map[string]bool)
	for _, c := range collections {
		if c.HelpCenterID.String() == helpCenterID {
			hcCollections = append(hcCollections, c)
			collectionSet[c.ID.String()] = true
		}
	}

	// Articles in this help center: those whose parent collection belongs to it,
	// plus uncategorized articles (no parent). In a multi-help-center workspace
	// uncategorized articles cannot be attributed to a specific help center, so
	// they are associated with the chosen one.
	irArticles := make([]importer.Article, 0, len(articles))
	for _, a := range articles {
		parent := a.ParentID.String()
		if parent != "" && !collectionSet[parent] {
			continue
		}
		irArticles = append(irArticles, mapArticle(a, parent, collectionSet))
	}

	defaultLocale := deriveDefaultLocale(irArticles)
	locales := deriveLocales(irArticles, defaultLocale)

	irCollections := make([]importer.Collection, 0, len(hcCollections))
	for _, c := range hcCollections {
		irCollections = append(irCollections, mapCollection(c, collectionSet, defaultLocale))
	}

	authors := make(map[string]importer.Author, len(admins))
	for _, a := range admins {
		authors[a.ID.String()] = importer.Author{ID: a.ID.String(), Email: a.Email, Name: a.Name}
	}

	return &importer.Corpus{
		HelpCenter:  importer.HelpCenter{ID: helpCenterID, DefaultLocale: defaultLocale},
		Collections: irCollections,
		Articles:    irArticles,
		Authors:     authors,
		Locales:     locales,
	}, nil
}

func mapCollection(c collection, collectionSet map[string]bool, defaultLocale string) importer.Collection {
	names := map[string]string{}
	descriptions := map[string]string{}
	if c.Name != "" {
		names[defaultLocale] = c.Name
	}
	if c.Description != "" {
		descriptions[defaultLocale] = c.Description
	}
	for locale, content := range parseTranslatedContent(c.TranslatedContent) {
		if content.Name != "" {
			names[locale] = content.Name
		}
		if content.Description != "" {
			descriptions[locale] = content.Description
		}
	}

	// Only keep a parent link if the parent belongs to the same help center.
	parent := c.ParentID.String()
	if parent != "" && !collectionSet[parent] {
		parent = ""
	}

	return importer.Collection{
		ID:           c.ID.String(),
		ParentID:     parent,
		Names:        names,
		Descriptions: descriptions,
		Order:        c.Order,
	}
}

func mapArticle(a article, parent string, collectionSet map[string]bool) importer.Article {
	defaultLocale := a.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	variants := map[string]importer.ArticleVariant{}
	for locale, content := range parseTranslatedContent(a.TranslatedContent) {
		variants[locale] = importer.ArticleVariant{
			Locale:      locale,
			Title:       content.Title,
			Description: content.Description,
			BodyHTML:    content.Body,
			AuthorID:    content.AuthorID.String(),
			State:       content.State,
		}
	}
	// Ensure the default-locale (root) variant exists, built from the top-level
	// article fields when translated_content lacks it.
	if _, ok := variants[defaultLocale]; !ok {
		variants[defaultLocale] = importer.ArticleVariant{
			Locale:      defaultLocale,
			Title:       a.Title,
			Description: a.Description,
			BodyHTML:    a.Body,
			AuthorID:    a.AuthorID.String(),
			State:       a.State,
		}
	}

	collectionID := parent
	if collectionID != "" && !collectionSet[collectionID] {
		collectionID = ""
	}

	return importer.Article{
		ID:            a.ID.String(),
		CollectionID:  collectionID,
		DefaultLocale: defaultLocale,
		AuthorID:      a.AuthorID.String(),
		Variants:      variants,
		SourceURL:     a.URL,
	}
}

// deriveDefaultLocale picks the most common article default locale, falling
// back to "en".
func deriveDefaultLocale(articles []importer.Article) string {
	counts := map[string]int{}
	for _, a := range articles {
		if a.DefaultLocale != "" {
			counts[a.DefaultLocale]++
		}
	}
	best := ""
	bestN := 0
	for locale, n := range counts {
		// Tie-break deterministically by locale string.
		if n > bestN || (n == bestN && locale < best) {
			best, bestN = locale, n
		}
	}
	if best == "" {
		return "en"
	}
	return best
}

// deriveLocales returns the unique locale set across all article variants and
// default locales, with the corpus default locale first and the rest sorted.
func deriveLocales(articles []importer.Article, defaultLocale string) []string {
	seen := map[string]bool{defaultLocale: true}
	var rest []string
	for _, a := range articles {
		for locale := range a.Variants {
			if !seen[locale] {
				seen[locale] = true
				rest = append(rest, locale)
			}
		}
	}
	sort.Strings(rest)
	return append([]string{defaultLocale}, rest...)
}
