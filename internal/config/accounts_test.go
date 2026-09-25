package config

import (
	"reflect"
	"testing"
)

const (
	appURL     = "https://app.chatwoot.com"
	stagingURL = "https://staging.chatwoot.com"
)

func names(accounts []Account) []string {
	out := make([]string, 0, len(accounts))
	for _, a := range accounts {
		out = append(out, a.Name)
	}
	return out
}

func TestSlugAccountName(t *testing.T) {
	cases := []struct {
		name string
		id   int
		want string
	}{
		{"Acme", 1, "acme"},
		{"Globex Inc.", 2, "globex-inc"},
		{"  Spaces   and__underscores ", 3, "spaces-and-underscores"},
		{"Café Crème", 4, "café-crème"},
		{"1234", 5, "account-5"},
		{"", 6, "account-6"},
		{"!!!", 7, "account-7"},
	}
	for _, tc := range cases {
		if got := slugAccountName(tc.name, tc.id); got != tc.want {
			t.Errorf("slugAccountName(%q, %d) = %q, want %q", tc.name, tc.id, got, tc.want)
		}
	}
}

func TestSyncAccountsRegistersEveryMembership(t *testing.T) {
	cfg := &Config{}
	added, removed := cfg.SyncAccounts(appURL+"/", 7, "Shivam Mishra", []Membership{
		{ID: 1, Name: "Chatwoot"},
		{ID: 42, Name: "Acme"},
		{ID: 77, Name: "Globex Inc"},
	})

	if want := []string{"chatwoot", "acme", "globex-inc"}; !reflect.DeepEqual(names(added), want) {
		t.Fatalf("added = %v, want %v", names(added), want)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", names(removed))
	}
	acme := cfg.Find("acme")
	if acme == nil || acme.BaseURL != appURL || acme.ID != 42 || acme.UserID != 7 ||
		acme.UserName != "Shivam Mishra" || acme.AccountName != "Acme" {
		t.Fatalf("acme = %#v", acme)
	}
}

func TestSyncAccountsQualifiesNameClashes(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 42, Name: "Acme"}})

	// Same name on another host gets the host label.
	added, _ := cfg.SyncAccounts(stagingURL, 3, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}})
	if want := []string{"chatwoot-staging"}; !reflect.DeepEqual(names(added), want) {
		t.Fatalf("other host: added = %v, want %v", names(added), want)
	}

	// Same account seen by a second user on the same host gets the user name.
	added, _ = cfg.SyncAccounts(appURL, 19, "Test Agent", []Membership{{ID: 42, Name: "Acme"}})
	if want := []string{"acme-test-agent"}; !reflect.DeepEqual(names(added), want) {
		t.Fatalf("second user: added = %v, want %v", names(added), want)
	}

	// Every generated name is unique.
	seen := map[string]bool{}
	for _, a := range cfg.Accounts {
		if seen[a.Name] {
			t.Fatalf("duplicate name %q in %v", a.Name, names(cfg.Accounts))
		}
		seen[a.Name] = true
	}
}

func TestSyncAccountsFallsBackWhenQualifiedNameIsTaken(t *testing.T) {
	cfg := &Config{Accounts: []Account{
		{Name: "acme", BaseURL: appURL, ID: 1, UserID: 7},
		{Name: "acme-staging", BaseURL: appURL, ID: 2, UserID: 7},
	}}
	added, _ := cfg.SyncAccounts(stagingURL, 3, "Shivam", []Membership{{ID: 9, Name: "Acme"}})
	if len(added) != 1 || added[0].Name == "acme" || added[0].Name == "acme-staging" {
		t.Fatalf("added = %v, want a unique fallback name", names(added))
	}
}

func TestSyncAccountsKeepsNamesStable(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 42, Name: "Acme"}})
	if err := cfg.Rename("acme", "work"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}

	// The account is renamed upstream; our name must not move.
	added, removed := cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 42, Name: "Acme Corp"}})
	if len(added) != 0 || len(removed) != 0 {
		t.Fatalf("re-sync changed membership: added %v removed %v", names(added), names(removed))
	}
	work := cfg.Find("work")
	if work == nil || work.AccountName != "Acme Corp" {
		t.Fatalf("work = %#v, want name kept and AccountName refreshed", work)
	}
}

func TestSyncAccountsRemovesLostMemberships(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 77, Name: "Globex"}})
	cfg.SyncAccounts(stagingURL, 3, "Shivam", []Membership{{ID: 5, Name: "QA"}})
	cfg.Default = "globex"

	added, removed := cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 91, Name: "Initech"}})
	if want := []string{"initech"}; !reflect.DeepEqual(names(added), want) {
		t.Fatalf("added = %v, want %v", names(added), want)
	}
	if want := []string{"globex"}; !reflect.DeepEqual(names(removed), want) {
		t.Fatalf("removed = %v, want %v", names(removed), want)
	}
	if cfg.Default != "" {
		t.Fatalf("Default = %q, want cleared after the default account was removed", cfg.Default)
	}
	// Other logins are untouched.
	if cfg.Find("qa") == nil {
		t.Fatal("sync of one login removed another login's account")
	}
}

func TestSyncAccountsWithoutMembershipListRemovesNothing(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}})
	_, removed := cfg.SyncAccounts(appURL, 7, "Shivam", nil)
	if len(removed) != 0 || cfg.Find("chatwoot") == nil {
		t.Fatalf("an empty membership list (older Chatwoot) must not remove accounts, removed %v", names(removed))
	}
}

func TestSyncAccountsAdoptsProvisionalMigratedAccount(t *testing.T) {
	cfg := migrateV1(legacyConfig{BaseURL: appURL, AccountID: 1})

	added, removed := cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 42, Name: "Acme"}})
	if want := []string{"acme"}; !reflect.DeepEqual(names(added), want) {
		t.Fatalf("added = %v, want %v (the migrated account is not new)", names(added), want)
	}
	if len(removed) != 0 {
		t.Fatalf("removed = %v, want none", names(removed))
	}
	def := cfg.DefaultAccount()
	if def == nil || def.Name != "chatwoot" || def.Provisional || def.UserID != 7 {
		t.Fatalf("default = %#v, want the migrated account named chatwoot and adopted by user 7", def)
	}
}

func TestSyncAccountsDoesNotAdoptOtherUsersAccounts(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 42, Name: "Acme"}})
	_, removed := cfg.SyncAccounts(appURL, 19, "Test Agent", []Membership{{ID: 5, Name: "Other"}})
	if len(removed) != 0 || cfg.Find("acme") == nil {
		t.Fatalf("syncing user 19 removed user 7's account: %v", names(removed))
	}
}

func TestRename(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}, {ID: 42, Name: "Acme"}})
	cfg.Default = "chatwoot"

	if err := cfg.Rename("chatwoot", "cw"); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if cfg.Default != "cw" || cfg.Find("cw") == nil || cfg.Find("chatwoot") != nil {
		t.Fatalf("rename did not move the account and default: %#v", cfg)
	}

	for _, bad := range []string{"acme", "", "42", "has space", "@at", "-dash"} {
		if err := cfg.Rename("cw", bad); err == nil {
			t.Errorf("Rename(cw, %q) succeeded, want error", bad)
		}
	}
	if err := cfg.Rename("missing", "x"); err == nil {
		t.Error("Rename of an unknown account succeeded")
	}
}

func TestSetDefault(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 42, Name: "Acme"}})
	if err := cfg.SetDefault("acme"); err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}
	if cfg.DefaultAccount().Name != "acme" {
		t.Fatalf("default = %#v", cfg.DefaultAccount())
	}
	if err := cfg.SetDefault("nope"); err == nil {
		t.Fatal("SetDefault of an unknown account succeeded")
	}
}

func TestRemoveBaseURL(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 1, Name: "Chatwoot"}})
	cfg.SyncAccounts(stagingURL, 3, "Shivam", []Membership{{ID: 5, Name: "QA"}})
	cfg.Default = "chatwoot"

	removed := cfg.RemoveBaseURL(appURL + "/")
	if want := []string{"chatwoot"}; !reflect.DeepEqual(names(removed), want) {
		t.Fatalf("removed = %v, want %v", names(removed), want)
	}
	if cfg.Default != "" || cfg.Find("qa") == nil {
		t.Fatalf("after RemoveBaseURL: %#v", cfg)
	}
}

func TestFindByID(t *testing.T) {
	cfg := &Config{}
	cfg.SyncAccounts(appURL, 7, "Shivam", []Membership{{ID: 42, Name: "Acme"}})
	cfg.SyncAccounts(appURL, 19, "Test Agent", []Membership{{ID: 42, Name: "Acme"}})
	cfg.SyncAccounts(stagingURL, 3, "Shivam", []Membership{{ID: 42, Name: "Acme"}})

	if got := cfg.FindByID(appURL+"/", 19, 42); got == nil || got.Name != "acme-test-agent" {
		t.Fatalf("FindByID(app, 19, 42) = %#v", got)
	}
	if got := cfg.FindByID(stagingURL, 3, 42); got == nil || got.BaseURL != stagingURL {
		t.Fatalf("FindByID(staging, 3, 42) = %#v", got)
	}
	if got := cfg.FindByID(appURL, 7, 1); got != nil {
		t.Fatalf("FindByID for a missing account = %#v, want nil", got)
	}
}
