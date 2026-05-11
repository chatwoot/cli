package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/zalando/go-keyring"
)

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
