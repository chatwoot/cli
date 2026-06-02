package importer

import (
	"fmt"

	"github.com/chatwoot/cli/internal/sdk"
)

// chatwootSink implements Sink over the Chatwoot SDK client.
type chatwootSink struct {
	client *sdk.Client
}

// NewChatwootSink returns a Sink backed by the given Chatwoot client.
func NewChatwootSink(client *sdk.Client) Sink {
	return &chatwootSink{client: client}
}

// EnsurePortal reuses an existing portal by slug (adding any missing locales)
// or creates a new one. The created bool is true only when a portal is created.
func (s *chatwootSink) EnsurePortal(target PortalTarget, locales []string) (PortalRef, bool, error) {
	hc := s.client.HelpCenter()
	slug := target.Slug()

	portals, err := hc.ListPortals()
	if err != nil {
		return PortalRef{}, false, err
	}

	var existing *sdk.HelpCenterPortal
	for i := range portals.Payload {
		if portals.Payload[i].Slug == slug {
			existing = &portals.Payload[i]
			break
		}
	}

	defaultLocale := ""
	if len(locales) > 0 {
		defaultLocale = locales[0]
	}

	if existing != nil {
		have := map[string]bool{}
		for _, l := range existing.Config.AllowedLocales {
			have[l.Code] = true
		}
		missing := false
		for _, l := range locales {
			if !have[l] {
				missing = true
				break
			}
		}
		if missing {
			union := unionLocales(allowedCodes(existing.Config.AllowedLocales), locales)
			dflt := existing.Config.DefaultLocale
			if dflt == "" {
				dflt = defaultLocale
			}
			updated, err := hc.UpdatePortal(slug, sdk.PortalInput{
				Config: &sdk.PortalConfigInput{AllowedLocales: union, DefaultLocale: dflt},
			})
			if err != nil {
				return PortalRef{}, false, fmt.Errorf("add locales to portal %q: %w", slug, err)
			}
			return PortalRef{Slug: updated.Slug, ID: updated.ID}, false, nil
		}
		return PortalRef{Slug: existing.Slug, ID: existing.ID}, false, nil
	}

	if !target.IsCreate() {
		return PortalRef{}, false, fmt.Errorf("portal %q not found", slug)
	}

	created, err := hc.CreatePortal(sdk.PortalInput{
		Name: target.CreateName,
		Slug: slug,
		Config: &sdk.PortalConfigInput{
			AllowedLocales: locales,
			DefaultLocale:  defaultLocale,
		},
	})
	if err != nil {
		return PortalRef{}, false, err
	}
	return PortalRef{Slug: created.Slug, ID: created.ID}, true, nil
}

func (s *chatwootSink) CreateCategory(portalSlug string, req sdk.CreateCategoryRequest) (sdk.HelpCenterCategory, error) {
	cat, err := s.client.HelpCenter().CreateCategory(portalSlug, req)
	if err != nil {
		return sdk.HelpCenterCategory{}, err
	}
	return *cat, nil
}

func (s *chatwootSink) CreateArticle(portalSlug string, req sdk.CreateArticleRequest) (sdk.HelpCenterArticle, error) {
	art, err := s.client.HelpCenter().CreateArticle(portalSlug, req)
	if err != nil {
		return sdk.HelpCenterArticle{}, err
	}
	return *art, nil
}

func (s *chatwootSink) UploadImage(externalURL string) (string, error) {
	res, err := s.client.HelpCenter().UploadImageExternalURL(externalURL)
	if err != nil {
		return "", err
	}
	return res.FileURL, nil
}

func allowedCodes(locales []sdk.HelpCenterPortalLocale) []string {
	out := make([]string, 0, len(locales))
	for _, l := range locales {
		out = append(out, l.Code)
	}
	return out
}
