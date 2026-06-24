package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

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
			// Known limitation: a flag with a value (e.g. `-a 2`) breaks the
			// rewrite because the loop only advances past tokens starting
			// with '-'. Users typing id-first must put global flags after
			// the noun, or use verb-first form.
			name: "flag-with-value bails (known limitation)",
			in:   []string{"-a", "2", "conv", "123", "open"},
			want: []string{"-a", "2", "conv", "123", "open"},
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
			// profile is a string-id noun: a non-numeric name is rewritten.
			name: "profile name verb -> profile verb name",
			in:   []string{"profile", "work", "use"},
			want: []string{"profile", "use", "work"},
		},
		{
			name: "profile name remove -> profile remove name",
			in:   []string{"profile", "personal", "remove"},
			want: []string{"profile", "remove", "personal"},
		},
		{
			// Verb-first profile input is left alone (the verb check guards it).
			name: "profile verb-first passes through",
			in:   []string{"profile", "use", "work"},
			want: []string{"profile", "use", "work"},
		},
		{
			name: "profile unknown verb passes through",
			in:   []string{"profile", "work", "explode"},
			want: []string{"profile", "work", "explode"},
		},
		{
			// A verb in the id slot is already verb-first, even when the profile
			// name collides with a verb (showing a profile literally named "use").
			name: "profile verb in id slot is not rewritten",
			in:   []string{"profile", "show", "use"},
			want: []string{"profile", "show", "use"},
		},
		{
			// `profile work` (no verb) is too short to rewrite; Kong's default
			// subcommand routes it to `profile show work`.
			name: "profile name only passes through",
			in:   []string{"profile", "work"},
			want: []string{"profile", "work"},
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

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
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
	if post.AccountID != 1 {
		t.Fatalf("persisted account_id = %d, want original account 1", post.AccountID)
	}
	if post.UserID != 77 {
		t.Fatalf("persisted user_id = %d, want fetched profile user 77", post.UserID)
	}
}
