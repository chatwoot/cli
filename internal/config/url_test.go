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
