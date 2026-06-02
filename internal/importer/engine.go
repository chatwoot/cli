package importer

import (
	"context"
	"fmt"
	"sort"

	"github.com/chatwoot/cli/internal/sdk"
)

// Sink performs the Chatwoot writes. It is an interface so Execute can be
// tested with a fake. The concrete implementation (chatwootSink) wraps
// *sdk.Client.
type Sink interface {
	// EnsurePortal creates the target portal (or reuses an existing one) and
	// ensures the given locales are allowed. Returns the portal ref and whether
	// it was newly created.
	EnsurePortal(target PortalTarget, locales []string) (PortalRef, bool, error)
	CreateCategory(portalSlug string, req sdk.CreateCategoryRequest) (sdk.HelpCenterCategory, error)
	CreateArticle(portalSlug string, req sdk.CreateArticleRequest) (sdk.HelpCenterArticle, error)
	UploadImage(externalURL string) (string, error)
}

// ExecuteOptions configures Execute.
type ExecuteOptions struct {
	Sink      Sink
	State     *State
	StatePath string // persisted after each create; "" disables persistence
	Embeds    EmbedRegistry
	Log       func(string)
}

// Plan builds the ordered, resolved set of writes from a scanned corpus and the
// user's selections. It performs no network calls — author resolution uses the
// pre-built resolver, and skip flags come from the loaded state.
func Plan(corpus *Corpus, sel Selections, resolver *AuthorResolver, st *State) *ImportPlan {
	if st == nil {
		st = NewState("", "", "")
	}
	defaultLocale := corpus.HelpCenter.DefaultLocale
	if defaultLocale == "" {
		defaultLocale = "en"
	}

	cats, needed := planCategories(corpus, sel, st, defaultLocale)
	articles := planArticles(corpus, sel, resolver, st)

	portalLocales := unionLocales([]string{defaultLocale}, mapKeys(needed), sel.Locales)

	return &ImportPlan{
		Portal:        sel.Target,
		DefaultLocale: defaultLocale,
		PortalLocales: portalLocales,
		Categories:    cats,
		Articles:      articles,
		AuthorStats:   AuthorStats{Matched: resolver.Matched, Fallback: resolver.Fallback},
	}
}

func planCategories(corpus *Corpus, sel Selections, st *State, defaultLocale string) ([]PlannedCategory, map[string]bool) {
	selSet := toSet(sel.Locales)
	collByID := make(map[string]Collection, len(corpus.Collections))
	for _, c := range corpus.Collections {
		collByID[c.ID] = c
	}

	need := map[string]map[string]bool{}
	addNeed := func(coll, loc string) {
		if coll == "" || loc == "" {
			return
		}
		if need[coll] == nil {
			need[coll] = map[string]bool{}
		}
		need[coll][loc] = true
	}

	// Direct needs: every article contributes its root locale plus any selected
	// variant locales to its collection.
	for _, a := range corpus.Articles {
		if a.CollectionID == "" {
			continue
		}
		addNeed(a.CollectionID, a.DefaultLocale)
		for loc := range a.Variants {
			if loc != a.DefaultLocale && selSet[loc] {
				addNeed(a.CollectionID, loc)
			}
		}
	}

	order := topoSortCollections(corpus.Collections, collByID)

	// Propagate needs up to ancestors (children first) so parent categories
	// exist in every locale a descendant needs (for parent_category_id links).
	for i := len(order) - 1; i >= 0; i-- {
		c := order[i]
		if c.ParentID == "" || collByID[c.ParentID].ID == "" {
			continue
		}
		for loc := range need[c.ID] {
			addNeed(c.ParentID, loc)
		}
	}

	// Ensure the default locale exists wherever any locale is needed, so each
	// collection has a root category to anchor its locale variants.
	for coll := range need {
		addNeed(coll, defaultLocale)
	}

	var planned []PlannedCategory
	for _, c := range order {
		locs := need[c.ID]
		if len(locs) == 0 {
			continue
		}
		slug := Slugify(c.Name(defaultLocale, defaultLocale))
		parent := ""
		if c.ParentID != "" && collByID[c.ParentID].ID != "" {
			parent = c.ParentID
		}
		for _, loc := range orderedLocales(locs, defaultLocale) {
			_, done := st.CategoryRef(c.ID, loc)
			planned = append(planned, PlannedCategory{
				CollectionID:       c.ID,
				ParentCollectionID: parent,
				Locale:             loc,
				IsRoot:             loc == defaultLocale,
				Name:               c.Name(loc, defaultLocale),
				Description:        c.Descriptions[loc],
				Slug:               slug,
				Skip:               done,
			})
		}
	}

	allNeeded := map[string]bool{}
	for _, locs := range need {
		for loc := range locs {
			allNeeded[loc] = true
		}
	}
	return planned, allNeeded
}

func planArticles(corpus *Corpus, sel Selections, resolver *AuthorResolver, st *State) []PlannedArticle {
	selSet := toSet(sel.Locales)
	var roots, variants []PlannedArticle

	for _, a := range corpus.Articles {
		rootLoc := a.DefaultLocale
		rv := a.Variants[rootLoc]
		_, rootDone := st.ArticleRef(a.ID, rootLoc)
		roots = append(roots, PlannedArticle{
			ArticleID:    a.ID,
			CollectionID: a.CollectionID,
			Locale:       rootLoc,
			RootLocale:   rootLoc,
			IsRoot:       true,
			Title:        rv.Title,
			Description:  rv.Description,
			BodyHTML:     rv.BodyHTML,
			AuthorID:     resolver.Resolve(firstNonEmpty(rv.AuthorID, a.AuthorID)),
			Skip:         rootDone,
		})

		for _, loc := range sortedKeys(a.Variants) {
			if loc == rootLoc || !selSet[loc] {
				continue
			}
			v := a.Variants[loc]
			_, done := st.ArticleRef(a.ID, loc)
			variants = append(variants, PlannedArticle{
				ArticleID:    a.ID,
				CollectionID: a.CollectionID,
				Locale:       loc,
				RootLocale:   rootLoc,
				IsRoot:       false,
				Title:        v.Title,
				Description:  v.Description,
				BodyHTML:     v.BodyHTML,
				AuthorID:     resolver.Resolve(firstNonEmpty(v.AuthorID, a.AuthorID)),
				Skip:         done,
			})
		}
	}
	return append(roots, variants...)
}

// Execute performs the planned writes: ensure portal, create categories
// (parent-before-child, root-locale-first), then create article roots followed
// by locale variants. State is persisted after each successful create so a
// crash resumes cleanly. Only a portal-ensure failure aborts; per-item failures
// are recorded and reported.
func Execute(ctx context.Context, plan *ImportPlan, opts ExecuteOptions) (*Result, error) {
	logf := func(format string, args ...any) {
		if opts.Log != nil {
			opts.Log(fmt.Sprintf(format, args...))
		}
	}
	st := opts.State
	res := &Result{AuthorStats: plan.AuthorStats}

	saveState := func() {
		if opts.StatePath != "" {
			_ = st.Save(opts.StatePath)
		}
	}

	// 1) Portal.
	ref, created, err := opts.Sink.EnsurePortal(plan.Portal, plan.PortalLocales)
	if err != nil {
		return nil, fmt.Errorf("ensure portal: %w", err)
	}
	st.TargetPortal = ref
	st.Locales = plan.PortalLocales
	res.PortalSlug = ref.Slug
	res.PortalCreated = created
	portalSlug := ref.Slug
	saveState()
	logf("portal ready: %s", portalSlug)

	// 2) Categories.
	for _, pc := range plan.Categories {
		if _, done := st.CategoryRef(pc.CollectionID, pc.Locale); done {
			res.CategoriesSkipped++
			continue
		}
		req := sdk.CreateCategoryRequest{
			Name:        pc.Name,
			Slug:        pc.Slug,
			Locale:      pc.Locale,
			Description: pc.Description,
		}
		if pc.ParentCollectionID != "" {
			if pref, ok := st.CategoryRef(pc.ParentCollectionID, pc.Locale); ok {
				req.ParentCategoryID = pref.ID
			}
		}
		if !pc.IsRoot {
			if root, ok := st.CategoryRef(pc.CollectionID, plan.DefaultLocale); ok {
				req.AssociatedCategoryID = root.ID
			}
		}
		cat, err := opts.Sink.CreateCategory(portalSlug, req)
		if err != nil {
			st.RecordFailure(categoryKey(pc.CollectionID, pc.Locale), "category", err.Error())
			res.Failures = append(res.Failures, st.Failures[categoryKey(pc.CollectionID, pc.Locale)])
			saveState()
			continue
		}
		st.SetCategory(pc.CollectionID, pc.Locale, ItemRef{ID: cat.ID, Slug: cat.Slug})
		res.CategoriesCreated++
		saveState()
	}

	// 3) Articles (roots first, then variants — guaranteed by plan ordering).
	uploadCache := map[string]string{}
	uploader := func(src string) (string, error) {
		if v, ok := uploadCache[src]; ok {
			return v, nil
		}
		u, err := opts.Sink.UploadImage(src)
		if err == nil && u != "" {
			uploadCache[src] = u
		}
		return u, err
	}

	for _, pa := range plan.Articles {
		if _, done := st.ArticleRef(pa.ArticleID, pa.Locale); done {
			res.ArticlesSkipped++
			continue
		}

		// Variants need their root to exist first.
		var associated int
		if !pa.IsRoot {
			root, ok := st.ArticleRef(pa.ArticleID, pa.RootLocale)
			if !ok {
				key := articleKey(pa.ArticleID, pa.Locale)
				st.RecordFailure(key, "article", "root article not created; skipped variant")
				res.Failures = append(res.Failures, st.Failures[key])
				saveState()
				continue
			}
			associated = root.ID
		}

		tr := TransformBody(pa.BodyHTML, uploader, opts.Embeds)
		res.ImagesSwapped += tr.ImagesSwapped
		res.ImagesFailed += tr.ImagesFailed
		res.EmbedsRewritten += tr.EmbedsRewritten

		req := sdk.CreateArticleRequest{
			Title:               pa.Title,
			Content:             tr.HTML,
			Description:         pa.Description,
			Status:              "draft",
			Locale:              pa.Locale,
			AuthorID:            pa.AuthorID,
			AssociatedArticleID: associated,
		}
		if pa.CollectionID != "" {
			if cref, ok := st.CategoryRef(pa.CollectionID, pa.Locale); ok {
				req.CategoryID = cref.ID
			} else {
				// Category missing (its create failed); import uncategorized.
				// Locale is still sent explicitly so the article keeps its
				// language (an uncategorized article is not locale-forced).
				res.UncategorizedCount++
			}
		}

		art, err := opts.Sink.CreateArticle(portalSlug, req)
		if err != nil {
			key := articleKey(pa.ArticleID, pa.Locale)
			st.RecordFailure(key, "article", err.Error())
			res.Failures = append(res.Failures, st.Failures[key])
			saveState()
			continue
		}
		st.SetArticle(pa.ArticleID, pa.Locale, ItemRef{ID: art.ID, Slug: art.Slug})
		res.ArticlesCreated++
		saveState()
	}

	logf("done: %d categories, %d articles created", res.CategoriesCreated, res.ArticlesCreated)
	return res, nil
}

// ---- ordering / helper utilities ----

// topoSortCollections returns collections with parents before children. Any
// nodes left in a cycle are appended as roots (defensive; cycles shouldn't
// occur in real data).
func topoSortCollections(collections []Collection, byID map[string]Collection) []Collection {
	childrenOf := map[string][]string{}
	indeg := map[string]int{}
	for _, c := range collections {
		indeg[c.ID] = 0
	}
	for _, c := range collections {
		if c.ParentID != "" && byID[c.ParentID].ID != "" {
			childrenOf[c.ParentID] = append(childrenOf[c.ParentID], c.ID)
			indeg[c.ID]++
		}
	}

	var queue []string
	for _, c := range collections {
		if indeg[c.ID] == 0 {
			queue = append(queue, c.ID)
		}
	}
	sort.Strings(queue)

	var order []Collection
	seen := map[string]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		order = append(order, byID[id])
		kids := childrenOf[id]
		sort.Strings(kids)
		for _, k := range kids {
			indeg[k]--
			if indeg[k] == 0 {
				queue = append(queue, k)
			}
		}
	}
	// Append any unvisited (cycle) collections deterministically.
	if len(order) < len(collections) {
		var leftover []Collection
		for _, c := range collections {
			if !seen[c.ID] {
				leftover = append(leftover, c)
			}
		}
		sort.Slice(leftover, func(i, j int) bool { return leftover[i].ID < leftover[j].ID })
		order = append(order, leftover...)
	}
	return order
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it] = true
	}
	return s
}

func mapKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// orderedLocales returns the locales in set with defaultLocale first, then the
// rest sorted for determinism.
func orderedLocales(set map[string]bool, defaultLocale string) []string {
	var rest []string
	for loc := range set {
		if loc != defaultLocale {
			rest = append(rest, loc)
		}
	}
	sort.Strings(rest)
	if set[defaultLocale] {
		return append([]string{defaultLocale}, rest...)
	}
	return rest
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
