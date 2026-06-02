package importer

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/sdk"
)

// fakeSink records writes in order and assigns predictable ids.
type fakeSink struct {
	cats []sdk.CreateCategoryRequest
	arts []sdk.CreateArticleRequest
	ups  []string
	nCat int
	nArt int
}

func (f *fakeSink) EnsurePortal(target PortalTarget, locales []string) (PortalRef, bool, error) {
	return PortalRef{Slug: target.Slug(), ID: 1}, true, nil
}
func (f *fakeSink) CreateCategory(_ string, req sdk.CreateCategoryRequest) (sdk.HelpCenterCategory, error) {
	f.nCat++
	f.cats = append(f.cats, req)
	return sdk.HelpCenterCategory{ID: f.nCat, Slug: req.Slug, Locale: req.Locale}, nil
}
func (f *fakeSink) CreateArticle(_ string, req sdk.CreateArticleRequest) (sdk.HelpCenterArticle, error) {
	f.nArt += 100
	f.arts = append(f.arts, req)
	return sdk.HelpCenterArticle{ID: f.nArt}, nil
}
func (f *fakeSink) UploadImage(url string) (string, error) {
	f.ups = append(f.ups, url)
	return "https://app/rails/active_storage/blobs/zz/a.png", nil
}

func sampleCorpus() *Corpus {
	enBody := `<p>Hi</p><img src="https://cdn.intercom.io/a.png"><div><iframe src="https://www.youtube.com/embed/abc"></iframe></div>`
	return &Corpus{
		HelpCenter: HelpCenter{ID: "hc1", DefaultLocale: "en"},
		Collections: []Collection{
			{ID: "c1", Names: map[string]string{"en": "Getting Started", "fr": "Commencer"}},
		},
		Articles: []Article{
			{
				ID: "a1", CollectionID: "c1", DefaultLocale: "en", AuthorID: "500",
				Variants: map[string]ArticleVariant{
					"en": {Locale: "en", Title: "Setup", BodyHTML: enBody, AuthorID: "500"},
					"fr": {Locale: "fr", Title: "Configuration", BodyHTML: "<p>FR</p>", AuthorID: "500"},
				},
			},
		},
		Authors: map[string]Author{"500": {ID: "500", Email: "ada@acme.com"}},
		Locales: []string{"en", "fr"},
	}
}

func runImport(t *testing.T, corpus *Corpus, sel Selections, st *State) (*fakeSink, *Result, *State) {
	t.Helper()
	if st == nil {
		st = NewState("intercom", "ws", "hc1")
	}
	resolver := NewAuthorResolver(corpus.Authors, map[string]int{"ada@acme.com": 42}, 1)
	plan := Plan(corpus, sel, resolver, st, nil)

	sink := &fakeSink{}
	statePath := filepath.Join(t.TempDir(), "state.json")
	res, err := Execute(context.Background(), plan, ExecuteOptions{
		Sink:      sink,
		State:     st,
		StatePath: statePath,
		Embeds:    DefaultEmbedRegistry(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return sink, res, st
}

func catByLocale(cats []sdk.CreateCategoryRequest, locale string) (sdk.CreateCategoryRequest, bool) {
	for _, c := range cats {
		if c.Locale == locale {
			return c, true
		}
	}
	return sdk.CreateCategoryRequest{}, false
}

func artByLocale(arts []sdk.CreateArticleRequest, locale string) (sdk.CreateArticleRequest, bool) {
	for _, a := range arts {
		if a.Locale == locale {
			return a, true
		}
	}
	return sdk.CreateArticleRequest{}, false
}

func TestExecuteLinksTranslationsAndOrders(t *testing.T) {
	corpus := sampleCorpus()
	sel := Selections{
		SourceHCID: "hc1",
		Target:     PortalTarget{CreateName: "Acme Support", CreateSlug: "acme-support"},
		Locales:    []string{"en", "fr"},
	}
	sink, res, st := runImport(t, corpus, sel, nil)

	// --- Categories: en (root) before fr (variant); fr links to en. ---
	if len(sink.cats) != 2 {
		t.Fatalf("categories created = %d, want 2", len(sink.cats))
	}
	if sink.cats[0].Locale != "en" {
		t.Errorf("first category locale = %q, want en (root first)", sink.cats[0].Locale)
	}
	enCat, _ := catByLocale(sink.cats, "en")
	frCat, _ := catByLocale(sink.cats, "fr")
	if enCat.Slug != frCat.Slug {
		t.Errorf("per-locale categories should share a slug: %q vs %q", enCat.Slug, frCat.Slug)
	}
	if enCat.AssociatedCategoryID != 0 {
		t.Errorf("root category should have no associated id, got %d", enCat.AssociatedCategoryID)
	}
	if frCat.AssociatedCategoryID != 1 {
		t.Errorf("fr category associated_category_id = %d, want 1 (en root)", frCat.AssociatedCategoryID)
	}

	// --- Articles: en root before fr variant; fr links to en root. ---
	if len(sink.arts) != 2 {
		t.Fatalf("articles created = %d, want 2", len(sink.arts))
	}
	if sink.arts[0].Locale != "en" {
		t.Errorf("first article locale = %q, want en (root first)", sink.arts[0].Locale)
	}
	enArt, _ := artByLocale(sink.arts, "en")
	frArt, _ := artByLocale(sink.arts, "fr")
	if enArt.Status != "draft" || frArt.Status != "draft" {
		t.Errorf("articles must be draft: en=%q fr=%q", enArt.Status, frArt.Status)
	}
	if enArt.AssociatedArticleID != 0 {
		t.Errorf("root article should have no associated id, got %d", enArt.AssociatedArticleID)
	}
	if frArt.AssociatedArticleID != 100 {
		t.Errorf("fr article associated_article_id = %d, want 100 (en root)", frArt.AssociatedArticleID)
	}

	// --- Variant placed in MATCHING-locale category (locale inheritance!). ---
	if enArt.CategoryID != 1 {
		t.Errorf("en article category = %d, want 1 (en category)", enArt.CategoryID)
	}
	if frArt.CategoryID != 2 {
		t.Errorf("fr article category = %d, want 2 (fr category, not en)", frArt.CategoryID)
	}

	// --- Author matched by email. ---
	if enArt.AuthorID != 42 {
		t.Errorf("author = %d, want 42 (matched)", enArt.AuthorID)
	}

	// --- Image re-hosted and embed rewritten in the en body. ---
	if !strings.Contains(enArt.Content, "rails/active_storage") {
		t.Errorf("image not swapped: %q", enArt.Content)
	}
	if strings.Contains(enArt.Content, "<iframe") || !strings.Contains(enArt.Content, "youtube.com/watch?v=abc") {
		t.Errorf("embed not rewritten to bare URL: %q", enArt.Content)
	}

	// --- Result + state. ---
	if res.CategoriesCreated != 2 || res.ArticlesCreated != 2 {
		t.Errorf("result counts cats=%d arts=%d, want 2/2", res.CategoriesCreated, res.ArticlesCreated)
	}
	if ref, ok := st.ArticleRef("a1", "fr"); !ok || ref.ID != 200 {
		t.Errorf("state fr article = %#v ok=%v, want id 200", ref, ok)
	}
}

func TestExecuteUncategorizedKeepsLocale(t *testing.T) {
	corpus := &Corpus{
		HelpCenter: HelpCenter{ID: "hc1", DefaultLocale: "en"},
		Articles: []Article{
			{ID: "a1", CollectionID: "", DefaultLocale: "en", AuthorID: "x",
				Variants: map[string]ArticleVariant{"en": {Locale: "en", Title: "Loose", BodyHTML: "<p>x</p>"}}},
		},
		Authors: map[string]Author{},
		Locales: []string{"en"},
	}
	sel := Selections{SourceHCID: "hc1", Target: PortalTarget{CreateName: "P", CreateSlug: "p"}, Locales: []string{"en"}}
	sink, res, _ := runImport(t, corpus, sel, nil)

	if len(sink.cats) != 0 {
		t.Errorf("no categories expected, got %d", len(sink.cats))
	}
	if len(sink.arts) != 1 {
		t.Fatalf("articles = %d, want 1", len(sink.arts))
	}
	if sink.arts[0].CategoryID != 0 {
		t.Errorf("uncategorized article should have no category, got %d", sink.arts[0].CategoryID)
	}
	if sink.arts[0].Locale != "en" {
		t.Errorf("uncategorized article must still send explicit locale, got %q", sink.arts[0].Locale)
	}
	if res.UncategorizedCount != 0 {
		// CollectionID was empty (genuinely uncategorized at source), not a failed
		// category, so this is not counted as a fallback.
		t.Errorf("UncategorizedCount = %d, want 0 for source-uncategorized", res.UncategorizedCount)
	}
}

func TestExecuteResumeSkipsCreatedItems(t *testing.T) {
	corpus := sampleCorpus()
	sel := Selections{
		SourceHCID: "hc1",
		Target:     PortalTarget{CreateName: "Acme", CreateSlug: "acme-support"},
		Locales:    []string{"en", "fr"},
	}
	// Pre-seed state as if a prior run created the en category and en root article.
	st := NewState("intercom", "ws", "hc1")
	st.SetCategory("c1", "en", ItemRef{ID: 1, Slug: "getting-started"})
	st.SetArticle("a1", "en", ItemRef{ID: 100})

	sink, res, _ := runImport(t, corpus, sel, st)

	// en category + en article already present -> only fr category + fr article created.
	if len(sink.cats) != 1 || sink.cats[0].Locale != "fr" {
		t.Errorf("expected only fr category created, got %#v", sink.cats)
	}
	if len(sink.arts) != 1 || sink.arts[0].Locale != "fr" {
		t.Errorf("expected only fr article created, got %#v", sink.arts)
	}
	if res.CategoriesSkipped != 1 || res.ArticlesSkipped != 1 {
		t.Errorf("skipped cats=%d arts=%d, want 1/1", res.CategoriesSkipped, res.ArticlesSkipped)
	}
	// fr variant still links to the pre-existing en root id from state.
	if sink.arts[0].AssociatedArticleID != 100 {
		t.Errorf("fr article associated id = %d, want 100 (resumed root)", sink.arts[0].AssociatedArticleID)
	}
}

func TestPlanDisambiguatesDuplicateCategorySlugs(t *testing.T) {
	corpus := &Corpus{
		HelpCenter: HelpCenter{ID: "hc1", DefaultLocale: "en"},
		Collections: []Collection{
			{ID: "c1", Names: map[string]string{"en": "FAQ"}},
			{ID: "c2", Names: map[string]string{"en": "FAQ"}},
		},
		Articles: []Article{
			{ID: "a1", CollectionID: "c1", DefaultLocale: "en", Variants: map[string]ArticleVariant{"en": {Locale: "en", Title: "x", BodyHTML: "<p>x</p>"}}},
			{ID: "a2", CollectionID: "c2", DefaultLocale: "en", Variants: map[string]ArticleVariant{"en": {Locale: "en", Title: "y", BodyHTML: "<p>y</p>"}}},
		},
		Authors: map[string]Author{},
		Locales: []string{"en"},
	}
	resolver := NewAuthorResolver(corpus.Authors, map[string]int{}, 1)
	plan := Plan(corpus, Selections{SourceHCID: "hc1", Target: PortalTarget{CreateSlug: "p"}, Locales: []string{"en"}}, resolver, NewState("i", "w", "hc1"), nil)

	slugByColl := map[string]string{}
	for _, pc := range plan.Categories {
		slugByColl[pc.CollectionID] = pc.Slug
	}
	if slugByColl["c1"] == "" || slugByColl["c2"] == "" {
		t.Fatalf("missing slugs: %#v", slugByColl)
	}
	if slugByColl["c1"] == slugByColl["c2"] {
		t.Fatalf("duplicate-named collections must get distinct slugs, both = %q", slugByColl["c1"])
	}
}

func TestPlanAvoidsReservedCategorySlugs(t *testing.T) {
	corpus := &Corpus{
		HelpCenter:  HelpCenter{ID: "hc1", DefaultLocale: "en"},
		Collections: []Collection{{ID: "c1", Names: map[string]string{"en": "Getting Started"}}},
		Articles: []Article{
			{ID: "a1", CollectionID: "c1", DefaultLocale: "en", Variants: map[string]ArticleVariant{"en": {Locale: "en", Title: "x", BodyHTML: "<p>x</p>"}}},
		},
		Authors: map[string]Author{},
		Locales: []string{"en"},
	}
	resolver := NewAuthorResolver(corpus.Authors, map[string]int{}, 1)
	// "getting-started" already exists in the portal.
	plan := Plan(corpus, Selections{SourceHCID: "hc1", Target: PortalTarget{ExistingSlug: "p"}, Locales: []string{"en"}}, resolver, NewState("i", "w", "hc1"), []string{"getting-started"})

	if len(plan.Categories) != 1 {
		t.Fatalf("categories = %d, want 1", len(plan.Categories))
	}
	if plan.Categories[0].Slug == "getting-started" {
		t.Fatalf("slug should avoid the reserved one, got %q", plan.Categories[0].Slug)
	}
}
