package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
)

func TestApiCmdCallsAccountScopedEndpoint(t *testing.T) {
	setupTestEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/1/conversations/123" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.Header.Get("api_access_token") != "test-token" {
			t.Errorf("api_access_token = %q, want test-token", r.Header.Get("api_access_token"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"status":"open"}`))
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	var out bytes.Buffer
	app.Printer.Writer = &out

	if err := (&ApiCmd{Path: "/conversations/123"}).Run(app); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got["id"] != float64(123) || got["status"] != "open" {
		t.Fatalf("output = %#v, want conversation JSON", got)
	}
}

func TestApiCmdUsesExactAPIPath(t *testing.T) {
	setupTestEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/profile" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"name":"Ada"}`))
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.Printer.Writer = &bytes.Buffer{}

	if err := (&ApiCmd{Path: "/api/v1/profile"}).Run(app); err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestApiCmdSendsMethodBodyAndHeaders(t *testing.T) {
	setupTestEnv(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/api/v1/accounts/1/conversations/123/custom_attributes" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("X-Test") != "yes" {
			t.Errorf("X-Test header = %q, want yes", r.Header.Get("X-Test"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		if strings.TrimSpace(string(body)) != `{"priority":"urgent"}` {
			t.Errorf("body = %q", string(body))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1, WritesEnabled: true}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.Printer.Writer = &bytes.Buffer{}

	err = (&ApiCmd{
		Method: "patch",
		Path:   "conversations/123/custom_attributes",
		Data:   `{"priority":"urgent"}`,
		Header: []string{"X-Test: yes"},
	}).Run(app)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
}

func TestApiCmdBlocksMutatingMethodsWhenWritesDisabled(t *testing.T) {
	setupTestEnv(t)

	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	app, err := NewApp(&CLI{Output: "text"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.Printer.Writer = &bytes.Buffer{}

	err = (&ApiCmd{
		Method: "patch",
		Path:   "conversations/123/custom_attributes",
		Data:   `{"priority":"urgent"}`,
	}).Run(app)
	if err == nil {
		t.Fatal("expected writes disabled error")
	}
	if !strings.Contains(err.Error(), "chatwoot config writes on") {
		t.Fatalf("error %q should mention how to enable writes", err.Error())
	}
	if called {
		t.Fatal("mutating raw API request should not reach the network when writes are disabled")
	}
}

func TestNormalizeAPIPathRejectsAbsoluteURLs(t *testing.T) {
	if _, _, err := normalizeAPIPath("https://example.com/api/v1/profile", false); err == nil {
		t.Fatal("expected absolute URL error")
	}
}
