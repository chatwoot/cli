package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/zalando/go-keyring"
)

func setupTestEnv(t *testing.T) {
	t.Helper()
	keyring.MockInit()
	if err := keyring.DeleteAll("chatwoot-cli"); err != nil {
		t.Fatalf("keyring.DeleteAll: %v", err)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv(config.APIKeyEnv, "test-token")
}

func TestAssignMeWithAccountOverrideDoesNotPersistOverride(t *testing.T) {
	setupTestEnv(t)

	var sawAssignment bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/profile":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":99,"name":"Ada Lovelace","email":"ada@example.com"}`))
		case "/api/v1/accounts/2/conversations/123/assignments":
			sawAssignment = true
			var body struct {
				AssigneeID int `json:"assignee_id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode assignment body: %v", err)
			}
			if body.AssigneeID != 99 {
				t.Errorf("assignee_id = %d, want 99", body.AssigneeID)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":99,"account_id":2,"name":"Ada Lovelace"}`))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	app, err := NewApp(&CLI{Output: "text", Account: 2}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.Client.AccountID != 2 {
		t.Fatalf("client account = %d, want override account 2", app.Client.AccountID)
	}

	if err := (&ConvAssignCmd{ID: 123, Agent: "me"}).Run(app); err != nil {
		t.Fatalf("ConvAssignCmd.Run: %v", err)
	}
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
	if post.UserID != 99 {
		t.Fatalf("persisted user_id = %d, want fetched profile user 99", post.UserID)
	}
}

func TestConvContactFetchesSenderContact(t *testing.T) {
	setupTestEnv(t)

	var sawContactGet bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/accounts/1/conversations/123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":123,"meta":{"sender":{"id":456,"name":"Grace Hopper"}}}`))
		case "/api/v1/accounts/1/contacts/456":
			sawContactGet = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"payload":{"id":456,"name":"Grace Hopper","email":"grace@example.com","phone_number":"+15555550101"}}`))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	if err := (&ConvContactCmd{ID: 123}).Run(app); err != nil {
		t.Fatalf("ConvContactCmd.Run: %v", err)
	}
	if !sawContactGet {
		t.Fatal("contact endpoint was not called — sender ID not resolved from conversation")
	}
}

func TestConvContactErrorsWhenNoSender(t *testing.T) {
	setupTestEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/accounts/1/conversations/777" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":777,"meta":{}}`))
			return
		}
		if strings.Contains(r.URL.Path, "/contacts/") {
			t.Errorf("contacts endpoint should not be called when sender is missing: %s", r.URL.Path)
		}
		http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}

	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	err = (&ConvContactCmd{ID: 777}).Run(app)
	if err == nil {
		t.Fatal("expected error for conversation with no sender, got nil")
	}
	if !strings.Contains(err.Error(), "no associated contact") {
		t.Fatalf("error %q should mention missing contact", err.Error())
	}
}
