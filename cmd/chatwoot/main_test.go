package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/chatwoot/cli/internal/cmd"
	"github.com/chatwoot/cli/internal/config"
	"github.com/zalando/go-keyring"
)

func TestRewriteIDFirstGrammar(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "conv id verb -> conv verb id",
			in:   []string{"conv", "123", "reply", "hi"},
			want: []string{"conv", "reply", "123", "hi"},
		},
		{
			name: "contact verb on conv (the new branch verb)",
			in:   []string{"conv", "456", "contact"},
			want: []string{"conv", "contact", "456"},
		},
		{
			name: "long noun form",
			in:   []string{"conversation", "789", "resolve"},
			want: []string{"conversation", "resolve", "789"},
		},
		{
			// Boolean flags before the noun are skipped fine.
			name: "boolean flag before noun",
			in:   []string{"-q", "conv", "123", "open"},
			want: []string{"-q", "conv", "open", "123"},
		},
		{
			// Global flags that take a value are skipped with their value.
			name: "flag with value before noun",
			in:   []string{"-a", "acme", "conv", "123", "open"},
			want: []string{"-a", "acme", "conv", "open", "123"},
		},
		{
			name: "long flags with values before noun",
			in:   []string{"--account", "acme", "--output", "json", "conv", "123", "open"},
			want: []string{"--account", "acme", "--output", "json", "conv", "open", "123"},
		},
		{
			name: "flag=value before noun",
			in:   []string{"--account=acme", "conv", "123", "open"},
			want: []string{"--account=acme", "conv", "open", "123"},
		},
		{
			name: "verb-first input passes through",
			in:   []string{"conv", "reply", "123", "hi"},
			want: []string{"conv", "reply", "123", "hi"},
		},
		{
			name: "unknown noun passes through",
			in:   []string{"label", "42", "delete"},
			want: []string{"label", "42", "delete"},
		},
		{
			name: "unknown verb passes through (not a known conv verb)",
			in:   []string{"conv", "123", "explode"},
			want: []string{"conv", "123", "explode"},
		},
		{
			name: "non-numeric second arg passes through",
			in:   []string{"conv", "abc", "reply"},
			want: []string{"conv", "abc", "reply"},
		},
		{
			name: "too few args passes through",
			in:   []string{"conv", "123"},
			want: []string{"conv", "123"},
		},
		{
			name: "empty args passes through",
			in:   []string{},
			want: []string{},
		},
		{
			name: "only flags passes through",
			in:   []string{"-h"},
			want: []string{"-h"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteIDFirstGrammar(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("rewriteIDFirstGrammar(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "@name becomes --account",
			in:   []string{"@acme", "convs"},
			want: []string{"--account=acme", "convs"},
		},
		{
			name: "@name with id-first grammar",
			in:   []string{"@staging", "conv", "123", "reply", "hi"},
			want: []string{"--account=staging", "conv", "reply", "123", "hi"},
		},
		{
			name: "@name after global flags",
			in:   []string{"-o", "json", "-q", "@acme", "convs"},
			want: []string{"-o", "json", "-q", "--account=acme", "convs"},
		},
		{
			// Only a leading @ selects an account; message text is never touched.
			name: "@ in message text is left alone",
			in:   []string{"conv", "123", "reply", "@john thanks"},
			want: []string{"conv", "reply", "123", "@john thanks"},
		},
		{
			name: "bare @ is left alone",
			in:   []string{"@", "convs"},
			want: []string{"@", "convs"},
		},
		{
			name: "no args",
			in:   []string{},
			want: []string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeArgs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normalizeArgs(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeArgsRoutesLinks(t *testing.T) {
	const (
		conv = "https://app.chatwoot.com/app/accounts/1/inbox/5/conversations/4521"
		acct = "--account=https://app.chatwoot.com/app/accounts/1"
	)
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"conversation link views it", []string{conv}, []string{acct, "conv", "4521"}},
		{"conversation link with verb", []string{conv, "reply", "on it"}, []string{acct, "conv", "reply", "4521", "on it"}},
		{"global flags kept", []string{"-o", "json", conv}, []string{"-o", "json", acct, "conv", "4521"}},
		{"link wins over -a", []string{"-a", "acme", conv, "resolve"}, []string{acct, "conv", "resolve", "4521"}},
		{"link wins over @name", []string{"@acme", conv}, []string{acct, "conv", "4521"}},
		{
			"contact link",
			[]string{"https://app.chatwoot.com/app/accounts/1/contacts/88", "conversations"},
			[]string{acct, "contact", "conversations", "88"},
		},
		{
			"inbox settings link",
			[]string{"https://app.chatwoot.com/app/accounts/1/settings/inboxes/5"},
			[]string{acct, "inbox", "5"},
		},
		{
			"other dashboard page selects the account",
			[]string{"https://app.chatwoot.com/app/accounts/1/dashboard", "labels"},
			[]string{acct, "labels"},
		},
		{
			"other dashboard page alone lists conversations",
			[]string{"https://app.chatwoot.com/app/accounts/1/dashboard"},
			[]string{acct, "convs"},
		},
		{
			"link inside message text is left alone",
			[]string{"conv", "1", "reply", conv},
			[]string{"conv", "reply", "1", conv},
		},
		{"non-dashboard URL is left alone", []string{"https://example.com"}, []string{"https://example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeArgs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("normalizeArgs(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseAccountFromFlagOrEnvironment(t *testing.T) {
	parse := func(args ...string) string {
		t.Helper()
		var cli cmd.CLI
		parser, err := newParser(&cli)
		if err != nil {
			t.Fatalf("newParser: %v", err)
		}
		if _, err := parser.Parse(normalizeArgs(args)); err != nil {
			t.Fatalf("Parse(%v): %v", args, err)
		}
		return cli.Account
	}

	t.Setenv("CHATWOOT_ACCOUNT", "")
	if got := parse("version"); got != "" {
		t.Fatalf("no selector: Account = %q, want empty", got)
	}

	t.Setenv("CHATWOOT_ACCOUNT", "acme")
	if got := parse("version"); got != "acme" {
		t.Fatalf("CHATWOOT_ACCOUNT: Account = %q, want acme", got)
	}
	if got := parse("@staging", "version"); got != "staging" {
		t.Fatalf("@staging over env: Account = %q, want staging", got)
	}
	if got := parse("-a", "42", "version"); got != "42" {
		t.Fatalf("-a 42 over env: Account = %q, want 42", got)
	}
}

func TestRewriteIDFirstGrammarDoesNotMutateInput(t *testing.T) {
	in := []string{"conv", "123", "reply", "hi"}
	original := append([]string{}, in...)
	_ = rewriteIDFirstGrammar(in)
	if !reflect.DeepEqual(in, original) {
		t.Errorf("input mutated: got %v, want %v", in, original)
	}
}

func TestAssignMeAccountOverrideSmoke(t *testing.T) {
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "test-token")

	var sawAssignment bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/profile":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":77,"name":"Grace Hopper","email":"grace@example.com"}`))
		case "/api/v1/accounts/2/conversations/123/assignments":
			sawAssignment = true
			var body struct {
				AssigneeID int `json:"assignee_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode assignment body: %v", err)
			}
			if body.AssigneeID != 77 {
				t.Errorf("assignee_id = %d, want 77", body.AssigneeID)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":77,"account_id":2,"name":"Grace Hopper"}`))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := config.Save(&config.Config{
		Default:  "test",
		Accounts: []config.Account{{Name: "test", BaseURL: server.URL, ID: 1}},
	}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	oldArgs := os.Args
	os.Args = []string{"chatwoot", "-a", "2", "conv", "assign", "123", "--agent", "me"}
	t.Cleanup(func() { os.Args = oldArgs })

	main()

	if !sawAssignment {
		t.Fatal("assignment endpoint was not called")
	}
	post, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if def := post.DefaultAccount(); def.ID != 1 || def.UserID != 77 {
		t.Fatalf("default account = %#v, want original account 1 with fetched user 77", def)
	}
}

func TestParseAccountsCommands(t *testing.T) {
	cases := []struct {
		args        []string
		wantCommand string
		check       func(*cmd.CLI) bool
	}{
		{[]string{"accounts"}, "accounts list", func(c *cmd.CLI) bool { return !c.Accounts.List.Refresh }},
		{[]string{"accounts", "--refresh"}, "accounts list", func(c *cmd.CLI) bool { return c.Accounts.List.Refresh }},
		{[]string{"accounts", "rename", "a", "b"}, "accounts rename <old> <new>", func(c *cmd.CLI) bool {
			return c.Accounts.Rename.Old == "a" && c.Accounts.Rename.New == "b"
		}},
		{[]string{"use", "acme"}, "use <name>", func(c *cmd.CLI) bool { return c.Use.Name == "acme" }},
		{[]string{"use"}, "use", func(c *cmd.CLI) bool { return c.Use.Name == "" }},
		{[]string{"auth", "login", "staging.chatwoot.com"}, "auth login <url>", func(c *cmd.CLI) bool {
			return c.Auth.Login.URL == "staging.chatwoot.com"
		}},
		{[]string{"auth", "logout", "staging.chatwoot.com"}, "auth logout <url>", func(c *cmd.CLI) bool {
			return c.Auth.Logout.URL == "staging.chatwoot.com"
		}},
	}
	for _, tc := range cases {
		var cli cmd.CLI
		parser, err := newParser(&cli)
		if err != nil {
			t.Fatalf("newParser: %v", err)
		}
		ctx, err := parser.Parse(normalizeArgs(tc.args))
		if err != nil {
			t.Fatalf("Parse(%v): %v", tc.args, err)
		}
		if ctx.Command() != tc.wantCommand || !tc.check(&cli) {
			t.Errorf("Parse(%v) = %q, want %q with expected fields", tc.args, ctx.Command(), tc.wantCommand)
		}
	}
}
