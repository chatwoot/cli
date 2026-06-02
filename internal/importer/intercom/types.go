package intercom

import (
	"encoding/json"
	"net/url"
	"strings"
)

// flexID normalizes Intercom ids that arrive as either JSON strings or numbers
// (e.g. collection ids are strings, but an article's parent_id/author_id are
// numbers) into a single string representation so they can be compared.
type flexID string

func (f *flexID) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		*f = ""
		return nil
	}
	if s[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*f = flexID(str)
		return nil
	}
	*f = flexID(s)
	return nil
}

func (f flexID) String() string { return string(f) }

// listEnvelope is Intercom's standard cursor-paginated list response.
type listEnvelope[T any] struct {
	Type       string `json:"type"`
	Data       []T    `json:"data"`
	Pages      *pages `json:"pages"`
	TotalCount int    `json:"total_count"`
}

type pages struct {
	Next       json.RawMessage `json:"next"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalPages int             `json:"total_pages"`
}

// nextCursor extracts the starting_after cursor from pages.next, which may be
// either an object {page, starting_after} (v2.x) or a bare URL string.
func (p *pages) nextCursor() string {
	if p == nil || len(p.Next) == 0 || string(p.Next) == "null" {
		return ""
	}
	var obj struct {
		StartingAfter string `json:"starting_after"`
	}
	if err := json.Unmarshal(p.Next, &obj); err == nil && obj.StartingAfter != "" {
		return obj.StartingAfter
	}
	var s string
	if err := json.Unmarshal(p.Next, &s); err == nil && s != "" {
		// next may be a full URL or a bare query string. Parse it and read the
		// decoded starting_after value, so percent-encoded cursors (%2F, %3D…)
		// are not re-encoded when we put them back on the next request.
		if u, err := url.Parse(s); err == nil && u.RawQuery != "" {
			if sa := u.Query().Get("starting_after"); sa != "" {
				return sa
			}
		}
		if vals, err := url.ParseQuery(s); err == nil {
			if sa := vals.Get("starting_after"); sa != "" {
				return sa
			}
		}
	}
	return ""
}

// meResponse is GET /me — the authenticated admin plus workspace (app) info.
type meResponse struct {
	Type  string `json:"type"`
	ID    flexID `json:"id"`
	Email string `json:"email"`
	App   struct {
		IDCode string `json:"id_code"`
		Name   string `json:"name"`
	} `json:"app"`
}

// adminList is GET /admins (note: uses "admins", not the standard "data").
type adminList struct {
	Type   string  `json:"type"`
	Admins []admin `json:"admins"`
}

type admin struct {
	ID    flexID `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type helpCenter struct {
	ID          flexID `json:"id"`
	DisplayName string `json:"display_name"`
	Identifier  string `json:"identifier"`
	WorkspaceID string `json:"workspace_id"`
}

type collection struct {
	ID                flexID          `json:"id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	ParentID          flexID          `json:"parent_id"`
	HelpCenterID      flexID          `json:"help_center_id"`
	Order             int             `json:"order"`
	TranslatedContent json.RawMessage `json:"translated_content"`
}

type article struct {
	ID                flexID          `json:"id"`
	ParentID          flexID          `json:"parent_id"`
	ParentType        string          `json:"parent_type"`
	Title             string          `json:"title"`
	Description       string          `json:"description"`
	Body              string          `json:"body"`
	AuthorID          flexID          `json:"author_id"`
	State             string          `json:"state"`
	URL               string          `json:"url"`
	DefaultLocale     string          `json:"default_locale"`
	TranslatedContent json.RawMessage `json:"translated_content"`
}

// localizedContent is one locale entry inside a translated_content object.
type localizedContent struct {
	Name        string `json:"name"`  // collections
	Title       string `json:"title"` // articles
	Description string `json:"description"`
	Body        string `json:"body"`
	AuthorID    flexID `json:"author_id"`
	State       string `json:"state"`
}

// parseTranslatedContent unmarshals an Intercom translated_content object,
// skipping its non-locale "type" key, into a locale -> content map.
func parseTranslatedContent(raw json.RawMessage) map[string]localizedContent {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var byKey map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byKey); err != nil {
		return nil
	}
	out := make(map[string]localizedContent)
	for key, val := range byKey {
		if key == "type" {
			continue
		}
		var c localizedContent
		if err := json.Unmarshal(val, &c); err != nil {
			continue
		}
		out[key] = c
	}
	return out
}
