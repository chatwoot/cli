package config

import "testing"

func TestParseInstanceURL(t *testing.T) {
	cases := []struct {
		in          string
		wantBase    string
		wantAccount int
	}{
		{"staging.chatwoot.com", "https://staging.chatwoot.com", 0},
		{"  https://app.chatwoot.com/  ", "https://app.chatwoot.com", 0},
		{"https://App.Chatwoot.com/app/accounts/1/conversations/4521", "https://app.chatwoot.com", 1},
		{"http://localhost:3000/app/accounts/2/dashboard", "http://localhost:3000", 2},
		{"localhost:3000", "https://localhost:3000", 0},
		{"https://example.com/chatwoot/app/accounts/3", "https://example.com/chatwoot", 3},
		{"https://example.com/chatwoot", "https://example.com/chatwoot", 0},
		{"https://app.chatwoot.com/app/login", "https://app.chatwoot.com", 0},
		{"https://app.chatwoot.com/?utm=x#frag", "https://app.chatwoot.com", 0},
	}
	for _, tc := range cases {
		base, account, err := ParseInstanceURL(tc.in)
		if err != nil {
			t.Errorf("ParseInstanceURL(%q) error = %v", tc.in, err)
			continue
		}
		if base != tc.wantBase || account != tc.wantAccount {
			t.Errorf("ParseInstanceURL(%q) = (%q, %d), want (%q, %d)", tc.in, base, account, tc.wantBase, tc.wantAccount)
		}
	}

	for _, bad := range []string{"", "   ", "https://", "ftp://example.com", "http://"} {
		if _, _, err := ParseInstanceURL(bad); err == nil {
			t.Errorf("ParseInstanceURL(%q) succeeded, want error", bad)
		}
	}
}

func TestDisplayHost(t *testing.T) {
	if got := DisplayHost("https://app.chatwoot.com"); got != "app.chatwoot.com" {
		t.Fatalf("DisplayHost = %q", got)
	}
	if got := DisplayHost("http://localhost:3000"); got != "localhost:3000" {
		t.Fatalf("DisplayHost = %q", got)
	}
	if got := DisplayHost("https://example.com/chatwoot"); got != "example.com/chatwoot" {
		t.Fatalf("DisplayHost = %q", got)
	}
}

func TestParseLink(t *testing.T) {
	const app = "https://app.chatwoot.com"
	cases := []struct {
		in   string
		want Link
	}{
		{app + "/app/accounts/1/conversations/4521", Link{app, 1, "conv", 4521}},
		{app + "/app/accounts/1/inbox/5/conversations/4521", Link{app, 1, "conv", 4521}},
		{app + "/app/accounts/1/label/billing/conversations/4521?x=1", Link{app, 1, "conv", 4521}},
		{app + "/app/accounts/1/team/2/conversations/4521", Link{app, 1, "conv", 4521}},
		{app + "/app/accounts/1/mentions/conversations/4521", Link{app, 1, "conv", 4521}},
		{app + "/app/accounts/1/custom_view/3/conversations/4521", Link{app, 1, "conv", 4521}},
		{app + "/app/accounts/1/contacts/88", Link{app, 1, "contact", 88}},
		{app + "/app/accounts/1/settings/inboxes/5", Link{app, 1, "inbox", 5}},
		{app + "/app/accounts/1/inbox/5", Link{app, 1, "inbox", 5}},
		{app + "/app/accounts/1/dashboard", Link{app, 1, "", 0}},
		{app + "/app/accounts/1/conversations", Link{app, 1, "", 0}},
		{"http://localhost:3000/app/accounts/2/conversations/9", Link{"http://localhost:3000", 2, "conv", 9}},
	}
	for _, tc := range cases {
		got, ok := ParseLink(tc.in)
		if !ok {
			t.Errorf("ParseLink(%q) not recognized", tc.in)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseLink(%q) = %#v, want %#v", tc.in, got, tc.want)
		}
	}

	// Only full links that name an account are links; everything else is an
	// ordinary argument.
	for _, notLink := range []string{
		"acme", "@acme", "123", "app.chatwoot.com/app/accounts/1/conversations/2",
		app, app + "/app/login", "ftp://x/app/accounts/1",
	} {
		if _, ok := ParseLink(notLink); ok {
			t.Errorf("ParseLink(%q) recognized a link", notLink)
		}
	}
}

func TestLinkAccountSelector(t *testing.T) {
	l := Link{BaseURL: "https://app.chatwoot.com", AccountID: 1}
	if got := l.AccountSelector(); got != "https://app.chatwoot.com/app/accounts/1" {
		t.Fatalf("AccountSelector() = %q", got)
	}
}
