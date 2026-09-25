package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseInstanceURL accepts whatever a user types or pastes for their Chatwoot
// instance — a bare host, a base URL, or any dashboard link — and returns the
// base URL plus the account ID when the link names one (/app/accounts/<id>).
// A path before /app/ is kept, for instances served under a subpath.
func ParseInstanceURL(raw string) (baseURL string, accountID int, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, fmt.Errorf("a Chatwoot URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0, fmt.Errorf("invalid Chatwoot URL %q: %w", raw, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", 0, fmt.Errorf("invalid Chatwoot URL %q: must start with http:// or https://", raw)
	}
	if u.Host == "" {
		return "", 0, fmt.Errorf("invalid Chatwoot URL %q: missing host", raw)
	}

	path := u.Path
	if i := strings.Index(path+"/", "/app/"); i >= 0 {
		rest := strings.Split(strings.Trim(path[i:], "/"), "/")
		if len(rest) >= 3 && rest[1] == "accounts" {
			if id, err := strconv.Atoi(rest[2]); err == nil && id > 0 {
				accountID = id
			}
		}
		path = path[:i]
	}

	base := u.Scheme + "://" + strings.ToLower(u.Host) + strings.TrimRight(path, "/")
	return base, accountID, nil
}

// DisplayHost is the base URL without its scheme, for compact listings.
func DisplayHost(baseURL string) string {
	if _, rest, ok := strings.Cut(baseURL, "://"); ok {
		return rest
	}
	return baseURL
}

// Link is a Chatwoot dashboard link resolved to the CLI resource it shows.
type Link struct {
	BaseURL   string
	AccountID int
	Noun      string // "conv", "contact", "inbox", or "" for other pages
	ID        int
}

// ParseLink recognizes a full dashboard link (http(s)://…/app/accounts/<id>/…)
// and maps it to a CLI noun: any …/conversations/<id> route (including inbox,
// label, team, mention, and custom-view scoped ones), contacts/<id>, and
// inbox/<id> or settings/inboxes/<id>.
func ParseLink(raw string) (Link, bool) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return Link{}, false
	}
	baseURL, accountID, err := ParseInstanceURL(raw)
	if err != nil || accountID == 0 {
		return Link{}, false
	}
	link := Link{BaseURL: baseURL, AccountID: accountID}

	u, err := url.Parse(raw)
	if err != nil {
		return Link{}, false
	}
	_, after, _ := strings.Cut(u.Path, fmt.Sprintf("/app/accounts/%d", accountID))
	segments := strings.Split(strings.Trim(after, "/"), "/")

	idAfter := func(i int) int {
		if i+1 < len(segments) {
			if id, err := strconv.Atoi(segments[i+1]); err == nil && id > 0 {
				return id
			}
		}
		return 0
	}
	for i := len(segments) - 1; i >= 0; i-- {
		if segments[i] == "conversations" {
			if id := idAfter(i); id > 0 {
				link.Noun, link.ID = "conv", id
				return link, true
			}
		}
	}
	if len(segments) > 0 {
		switch {
		case segments[0] == "contacts" && idAfter(0) > 0:
			link.Noun, link.ID = "contact", idAfter(0)
		case segments[0] == "inbox" && idAfter(0) > 0:
			link.Noun, link.ID = "inbox", idAfter(0)
		case len(segments) > 1 && segments[0] == "settings" && segments[1] == "inboxes" && idAfter(1) > 0:
			link.Noun, link.ID = "inbox", idAfter(1)
		}
	}
	return link, true
}

// AccountSelector is the account part of the link, in the form Resolve takes.
func (l Link) AccountSelector() string {
	return fmt.Sprintf("%s/app/accounts/%d", l.BaseURL, l.AccountID)
}
