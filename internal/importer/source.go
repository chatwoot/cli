package importer

import "context"

// Source is the provider abstraction. Intercom is the first implementation;
// Crisp/Zendesk can be added later as new packages without touching the engine.
type Source interface {
	// Name is a stable provider key used in the state-file id (e.g. "intercom").
	Name() string

	// Validate confirms credentials work and returns a stable workspace
	// identifier used for state keying.
	Validate(ctx context.Context) (workspaceID string, err error)

	// ListHelpCenters returns the selectable source help centers. Providers
	// without a multi-help-center concept return a single synthetic entry.
	ListHelpCenters(ctx context.Context) ([]HelpCenter, error)

	// Scan pulls the full IR graph for one help center: collections (with
	// parents), articles (with per-locale variants + author ids), authors, and
	// the derived locale set. Implementations handle pagination and rate
	// limiting internally.
	Scan(ctx context.Context, helpCenterID string) (*Corpus, error)
}
