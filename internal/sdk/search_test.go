package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The global /search endpoint is the dashboard's Api::V1::Accounts::SearchController.
// It is not part of Chatwoot's published OpenAPI spec (testdata/application_swagger.json
// only documents /contacts/search), so it cannot be exercised through the swagger
// contract harness in contract_test.go. These tests assert the request shape and
// response decoding against a plain httptest server instead.

func TestSearchGlobalSendsQueryParamsAndDecodesAllBuckets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/1/search" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		for key, want := range map[string]string{
			"q":    "ada",
			"page": "2",
		} {
			if got := q.Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"payload": {
				"conversations": [{
					"id": 462,
					"account_id": 1,
					"created_at": 1700000000,
					"contact": {"id": 5, "name": "Ada Lovelace"},
					"inbox": {"id": 2, "name": "Email", "channel_type": "Channel::Email"}
				}],
				"contacts": [{"id": 5, "name": "Ada Lovelace", "email": "ada@example.com", "phone_number": "+100", "last_activity_at": 1700000000}],
				"messages": [{"id": 9, "content": "hello ada", "conversation_id": 462, "created_at": 1700000000, "sender": {"name": "Ada Lovelace"}}],
				"articles": [{"id": 3, "title": "Ada guide", "locale": "en", "status": "published", "updated_at": 1700000000}]
			}
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "api-key", 1, WithHTTPClient(server.Client()))

	resp, err := client.Search().Global(SearchOptions{Query: "ada", Page: 2})
	if err != nil {
		t.Fatalf("Global returned error: %v", err)
	}

	p := resp.Payload
	if len(p.Conversations) != 1 || p.Conversations[0].ID != 462 {
		t.Fatalf("conversations = %#v", p.Conversations)
	}
	if p.Conversations[0].Contact == nil || p.Conversations[0].Contact.Name != "Ada Lovelace" {
		t.Fatalf("conversation contact = %#v", p.Conversations[0].Contact)
	}
	if len(p.Contacts) != 1 || p.Contacts[0].Email != "ada@example.com" {
		t.Fatalf("contacts = %#v", p.Contacts)
	}
	if len(p.Messages) != 1 || p.Messages[0].ConversationID != 462 {
		t.Fatalf("messages = %#v", p.Messages)
	}
	if len(p.Articles) != 1 || p.Articles[0].Title != "Ada guide" {
		t.Fatalf("articles = %#v", p.Articles)
	}
}

// An optional zero page must be omitted so the server applies its own default
// rather than receiving page=0.
func TestSearchGlobalOmitsZeroValuedParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("q") != "ada" {
			t.Errorf("q = %q, want ada", q.Get("q"))
		}
		for _, key := range []string{"page"} {
			if q.Has(key) {
				t.Errorf("param %s should be omitted, raw query: %s", key, r.URL.RawQuery)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload": {"conversations": [], "contacts": [], "messages": [], "articles": []}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "api-key", 1, WithHTTPClient(server.Client()))

	if _, err := client.Search().Global(SearchOptions{Query: "ada"}); err != nil {
		t.Fatalf("Global returned error: %v", err)
	}
}
