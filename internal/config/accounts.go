package config

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Membership is one account a login can see, as reported by the profile
// endpoint.
type Membership struct {
	ID   int
	Name string
}

// SyncAccounts reconciles the accounts registered for one login (base URL +
// user) with its current memberships. New memberships are registered under a
// generated name, lost ones are removed, and existing names never change —
// except provisional ones from migration, which nobody has typed yet.
//
// An empty membership list (older Chatwoot versions omit it) removes nothing.
func (c *Config) SyncAccounts(baseURL string, userID int, userName string, memberships []Membership) (added, removed []Account) {
	baseURL = normalizeBaseURL(baseURL)
	byID := make(map[int]Membership, len(memberships))
	for _, m := range memberships {
		byID[m.ID] = m
	}

	kept := c.Accounts[:0:0]
	seen := map[int]bool{}
	for _, acct := range c.Accounts {
		// Accounts migrated from v1 may not know their user yet; the login that
		// owns their token adopts them.
		if acct.BaseURL != baseURL || (acct.UserID != userID && acct.UserID != 0) {
			kept = append(kept, acct)
			continue
		}
		m, member := byID[acct.ID]
		if !member && len(memberships) > 0 {
			removed = append(removed, acct)
			continue
		}
		acct.UserID = userID
		acct.UserName = userName
		if member {
			acct.AccountName = m.Name
			seen[acct.ID] = true
		}
		kept = append(kept, acct)
	}
	c.Accounts = kept

	for _, acct := range removed {
		if c.Default == acct.Name {
			c.Default = ""
		}
	}

	// Name provisional accounts now that their real names are known.
	for i := range c.Accounts {
		acct := &c.Accounts[i]
		if !acct.Provisional || acct.BaseURL != baseURL || acct.UserID != userID {
			continue
		}
		acct.Provisional = false
		if acct.AccountName == "" {
			continue
		}
		old := acct.Name
		acct.Name = "" // free the provisional name so it can't clash with itself
		acct.Name = c.uniqueName(slugAccountName(acct.AccountName, acct.ID), baseURL, userName, acct.ID)
		if c.Default == old {
			c.Default = acct.Name
		}
	}

	for _, m := range memberships {
		if seen[m.ID] {
			continue
		}
		acct := Account{
			Name:        c.uniqueName(slugAccountName(m.Name, m.ID), baseURL, userName, m.ID),
			BaseURL:     baseURL,
			ID:          m.ID,
			UserID:      userID,
			UserName:    userName,
			AccountName: m.Name,
		}
		c.Accounts = append(c.Accounts, acct)
		added = append(added, acct)
	}
	return added, removed
}

// MergeAccounts registers memberships for one login without removing any
// account. It is for logins whose membership list is incomplete, such as the
// single account typed in on Chatwoot versions whose profile omits the list.
func (c *Config) MergeAccounts(baseURL string, userID int, userName string, memberships []Membership) (added []Account) {
	baseURL = normalizeBaseURL(baseURL)
	for _, m := range memberships {
		if acct := c.FindByID(baseURL, userID, m.ID); acct != nil {
			acct.UserName = userName
			if m.Name != "" {
				acct.AccountName = m.Name
			}
			continue
		}
		acct := Account{
			Name:        c.uniqueName(slugAccountName(m.Name, m.ID), baseURL, userName, m.ID),
			BaseURL:     baseURL,
			ID:          m.ID,
			UserID:      userID,
			UserName:    userName,
			AccountName: m.Name,
		}
		c.Accounts = append(c.Accounts, acct)
		added = append(added, acct)
	}
	return added
}

// uniqueName picks a free name for a new account. A clash with another
// instance's account adds this instance's host label (chatwoot-staging); a
// clash on the same instance means a second user, so it adds the user's name
// (acme-test-agent).
func (c *Config) uniqueName(base, baseURL, userName string, id int) string {
	if c.Find(base) == nil {
		return base
	}
	var candidates []string
	if c.Find(base).BaseURL != baseURL {
		candidates = append(candidates, base+"-"+hostLabel(baseURL))
	}
	if user := slugName(userName); user != "" {
		candidates = append(candidates, base+"-"+user)
	}
	candidates = append(candidates, base+"-"+hostLabel(baseURL), fmt.Sprintf("%s-%d", base, id))
	for _, cand := range candidates {
		if c.Find(cand) == nil {
			return cand
		}
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s-%d-%d", base, id, n)
		if c.Find(cand) == nil {
			return cand
		}
	}
}

// Rename gives an account a new name, moving the default with it.
func (c *Config) Rename(oldName, newName string) error {
	acct := c.Find(oldName)
	if acct == nil {
		return fmt.Errorf("no account named %q", oldName)
	}
	if err := validateName(newName); err != nil {
		return err
	}
	if newName == oldName {
		return nil
	}
	if c.Find(newName) != nil {
		return fmt.Errorf("an account named %q already exists", newName)
	}
	acct.Name = newName
	acct.Provisional = false
	if c.Default == oldName {
		c.Default = newName
	}
	return nil
}

// SetDefault makes the named account the default.
func (c *Config) SetDefault(name string) error {
	if c.Find(name) == nil {
		return fmt.Errorf("no account named %q", name)
	}
	c.Default = name
	return nil
}

// RemoveBaseURL drops every account on an instance and returns them.
func (c *Config) RemoveBaseURL(baseURL string) []Account {
	baseURL = normalizeBaseURL(baseURL)
	var removed []Account
	kept := c.Accounts[:0:0]
	for _, acct := range c.Accounts {
		if acct.BaseURL == baseURL {
			removed = append(removed, acct)
			if c.Default == acct.Name {
				c.Default = ""
			}
			continue
		}
		kept = append(kept, acct)
	}
	c.Accounts = kept
	return removed
}

// UserIDs returns the distinct user IDs with accounts on an instance.
func (c *Config) UserIDs(baseURL string) []int {
	baseURL = normalizeBaseURL(baseURL)
	var ids []int
	for _, acct := range c.Accounts {
		if acct.BaseURL == baseURL && acct.UserID != 0 && !slices.Contains(ids, acct.UserID) {
			ids = append(ids, acct.UserID)
		}
	}
	return ids
}

// validateName enforces names that are easy to type after @ and can never be
// confused with a numeric account ID.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("account name cannot be empty")
	}
	if _, err := strconv.Atoi(name); err == nil {
		return fmt.Errorf("account name %q cannot be only digits (it would read as an account ID)", name)
	}
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("account name %q cannot start with '-'", name)
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' {
			return fmt.Errorf("account name %q may only contain letters, digits, and '-'", name)
		}
	}
	return nil
}

// slugAccountName turns an account name into an @-friendly name. Names that
// end up empty or all digits fall back to account-<id>.
func slugAccountName(name string, id int) string {
	slug := slugName(name)
	if _, err := strconv.Atoi(slug); slug == "" || err == nil {
		return fmt.Sprintf("account-%d", id)
	}
	return slug
}

func slugName(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
			continue
		}
		dash = true
	}
	return b.String()
}

// hostLabel is the first DNS label of an instance's host (staging for
// staging.chatwoot.com), or the whole host for IPs and single-label hosts.
func hostLabel(baseURL string) string {
	host := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	if strings.Count(host, ".") == 3 && strings.IndexFunc(host, unicode.IsLetter) < 0 {
		return strings.ReplaceAll(host, ".", "-")
	}
	if label, _, ok := strings.Cut(host, "."); ok {
		return label
	}
	return host
}

// normalizeBaseURL is the canonical form every base URL is stored and compared
// in: no trailing slash, lowercase scheme and host (paths keep their case).
func normalizeBaseURL(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		return baseURL
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + u.EscapedPath()
}
