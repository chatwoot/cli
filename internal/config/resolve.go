package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrNoDefaultAccount = errors.New("no default account")
	ErrUnknownAccount   = errors.New("unknown account")
	ErrAmbiguousAccount = errors.New("ambiguous account")
)

// Resolve picks the account a command runs against. The selector is what the
// user typed after @ or -a (a leading @ is ignored):
//
//   - empty: the default account
//   - digits: that account ID on the default account's login, as `-a <id>`
//     meant before accounts had names. An ID the config doesn't know returns
//     an unregistered copy (Name "") of the default with the ID swapped.
//   - otherwise: an exact name, or a prefix matching exactly one name.
func (c *Config) Resolve(selector string) (*Account, error) {
	selector = strings.TrimPrefix(strings.TrimSpace(selector), "@")

	if selector == "" {
		if def := c.DefaultAccount(); def != nil {
			return def, nil
		}
		return nil, noDefaultError()
	}

	if id, err := strconv.Atoi(selector); err == nil {
		def := c.DefaultAccount()
		if def == nil {
			return nil, noDefaultError()
		}
		if def.ID == id {
			return def, nil
		}
		if acct := c.FindByID(def.BaseURL, def.UserID, id); acct != nil {
			return acct, nil
		}
		adhoc := *def
		adhoc.Name = ""
		adhoc.ID = id
		adhoc.AccountName = ""
		adhoc.HelpCenter = HelpCenterConfig{}
		adhoc.Provisional = false
		return &adhoc, nil
	}

	if acct := c.Find(selector); acct != nil {
		return acct, nil
	}
	var matches []*Account
	if c != nil {
		for i := range c.Accounts {
			if strings.HasPrefix(c.Accounts[i].Name, selector) {
				matches = append(matches, &c.Accounts[i])
			}
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("%w: no account matches @%s (see: chatwoot accounts)", ErrUnknownAccount, selector)
	default:
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Name
		}
		return nil, fmt.Errorf("%w: @%s matches %s — be more specific", ErrAmbiguousAccount, selector, strings.Join(names, ", "))
	}
}

func noDefaultError() error {
	return fmt.Errorf("%w. Pick one: chatwoot use <name>   (see: chatwoot accounts)", ErrNoDefaultAccount)
}
