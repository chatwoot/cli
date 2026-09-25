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
