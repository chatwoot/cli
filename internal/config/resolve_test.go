package config

import (
	"errors"
	"strings"
	"testing"
)

func resolveFixture() *Config {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 42, Name: "Acme"}})
	cfg.SyncAccounts(appURL, 19, "Test Agent", []Membership{{ID: 42, Name: "Acme"}})
	cfg.SyncAccounts(stagingURL, 3, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 3, Name: "QA Sandbox"}})
	cfg.Default = "chatwoot"
	return cfg
}

func TestResolveSelectsAccount(t *testing.T) {
	cfg := resolveFixture()
	cases := []struct {
		selector string
		wantName string
	}{
		{"", "chatwoot"}, // default
		{"acme", "acme"}, // exact match beats prefix match on acme-test-agent
		{"chatwoot-staging", "chatwoot-staging"},
		{"qa", "qa-sandbox"},  // unique prefix
		{"@qa", "qa-sandbox"}, // a leading @ is accepted
		{"acme-t", "acme-test-agent"},
		{"42", "acme"}, // numeric = account ID for the default's login
	}
	for _, tc := range cases {
		got, err := cfg.Resolve(tc.selector)
		if err != nil {
			t.Errorf("Resolve(%q) error = %v", tc.selector, err)
			continue
		}
		if got.Name != tc.wantName {
			t.Errorf("Resolve(%q) = %q, want %q", tc.selector, got.Name, tc.wantName)
		}
		if got != cfg.Find(got.Name) {
			t.Errorf("Resolve(%q) returned a copy, want the stored account", tc.selector)
		}
	}
}

// `-a <id>` for an account the config doesn't know keeps working as before:
// same instance and login as the default, different account ID.
func TestResolveNumericUnregisteredAccount(t *testing.T) {
	cfg := resolveFixture()
	got, err := cfg.Resolve("99")
	if err != nil {
		t.Fatalf("Resolve(99) error = %v", err)
	}
	if got.Name != "" || got.ID != 99 || got.BaseURL != appURL || got.UserID != 7 {
		t.Fatalf("Resolve(99) = %#v, want an unregistered account 99 on the default login", got)
	}
	if cfg.Find("chatwoot").ID != 1 {
		t.Fatal("Resolve(99) modified the default account")
	}
}

func TestResolveErrors(t *testing.T) {
	cfg := resolveFixture()

	_, err := cfg.Resolve("ac")
	if !errors.Is(err, ErrAmbiguousAccount) {
		t.Fatalf("Resolve(ac) error = %v, want ErrAmbiguousAccount", err)
	}
	for _, want := range []string{"@ac", "acme", "acme-test-agent"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ambiguous error %q missing %q", err, want)
		}
	}

	if _, err := cfg.Resolve("nope"); !errors.Is(err, ErrUnknownAccount) {
		t.Fatalf("Resolve(nope) error = %v, want ErrUnknownAccount", err)
	}

	cfg.Default = ""
	_, err = cfg.Resolve("")
	if !errors.Is(err, ErrNoDefaultAccount) || !strings.Contains(err.Error(), "chatwoot use <name>") {
		t.Fatalf("Resolve() without default error = %v, want ErrNoDefaultAccount with a hint", err)
	}
	if _, err := cfg.Resolve("42"); !errors.Is(err, ErrNoDefaultAccount) {
		t.Fatalf("Resolve(42) without default error = %v, want ErrNoDefaultAccount", err)
	}

	var empty *Config
	if _, err := empty.Resolve(""); !errors.Is(err, ErrNoDefaultAccount) {
		t.Fatalf("nil config Resolve() error = %v, want ErrNoDefaultAccount", err)
	}
}

func TestResolveLinkSelector(t *testing.T) {
	cfg := resolveFixture()

	got, err := cfg.Resolve(stagingURL + "/app/accounts/3")
	if err != nil || got.Name != "qa-sandbox" {
		t.Fatalf("Resolve(staging link) = (%#v, %v), want qa-sandbox", got, err)
	}

	// Two logins see app #42; the default's login wins.
	got, err = cfg.Resolve(appURL + "/app/accounts/42/conversations/1")
	if err != nil || got.Name != "acme" {
		t.Fatalf("Resolve(app #42 link) = (%#v, %v), want acme", got, err)
	}

	// Known instance, unregistered account: unknown (a refresh may find it).
	if _, err := cfg.Resolve(appURL + "/app/accounts/999"); !errors.Is(err, ErrUnknownAccount) {
		t.Fatalf("Resolve(unknown account link) error = %v, want ErrUnknownAccount", err)
	}

	// Unknown instance: not logged in, carrying the instance to log in to.
	_, err = cfg.Resolve("https://eu.chatwoot.com/app/accounts/5/conversations/88")
	var notLoggedIn *NotLoggedInError
	if !errors.As(err, &notLoggedIn) || notLoggedIn.BaseURL != "https://eu.chatwoot.com" {
		t.Fatalf("Resolve(unknown instance) error = %v, want NotLoggedInError for eu.chatwoot.com", err)
	}
	if !strings.Contains(err.Error(), "chatwoot auth login https://eu.chatwoot.com") {
		t.Fatalf("error %q should say how to log in", err)
	}
}
