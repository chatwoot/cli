package intercom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeIntercom serves canned responses for the Source mapping test.
func fakeIntercom(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admins":
			_, _ = w.Write([]byte(`{"type":"admin.list","admins":[
				{"id":"500","name":"Ada","email":"ada@acme.com"}
			]}`))
		case "/help_center/collections":
			_, _ = w.Write([]byte(`{"type":"list","data":[
				{"id":"10","name":"Getting Started","help_center_id":"hc1","parent_id":null,
				 "translated_content":{"type":"group_translated_content","fr":{"type":"group_content","name":"Commencer"}}},
				{"id":"11","name":"Other HC Collection","help_center_id":"hc2","parent_id":null}
			],"pages":{"next":null},"total_count":2}`))
		case "/articles":
			_, _ = w.Write([]byte(`{"type":"list","data":[
				{"id":"1001","parent_id":10,"parent_type":"collection","title":"Setup","description":"d",
				 "body":"<p>EN body</p>","author_id":500,"state":"published","default_locale":"en","url":"https://h/en/articles/1001-setup",
				 "translated_content":{"type":"article_translated_content",
				    "fr":{"type":"article_content","title":"Configuration","description":"df","body":"<p>FR body</p>","author_id":500,"state":"published"}}},
				{"id":"2002","parent_id":99,"title":"Other HC article","body":"x","default_locale":"en"}
			],"pages":{"next":null},"total_count":2}`))
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
		}
	}))
}

func TestScanMapsToIR(t *testing.T) {
	server := fakeIntercom(t)
	t.Cleanup(server.Close)

	src := New(server.URL, "tok", WithHTTPClient(server.Client()))
	corpus, err := src.Scan(context.Background(), "hc1")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	// Only hc1's collection is kept.
	if len(corpus.Collections) != 1 || corpus.Collections[0].ID != "10" {
		t.Fatalf("collections = %#v, want only id 10", corpus.Collections)
	}
	coll := corpus.Collections[0]
	if coll.Names["en"] != "Getting Started" || coll.Names["fr"] != "Commencer" {
		t.Errorf("collection names = %#v, want en+fr", coll.Names)
	}

	// Article 2002 belongs to collection 99 (not in hc1) -> excluded.
	if len(corpus.Articles) != 1 || corpus.Articles[0].ID != "1001" {
		t.Fatalf("articles = %#v, want only 1001", corpus.Articles)
	}
	art := corpus.Articles[0]
	if art.CollectionID != "10" {
		t.Errorf("article collection = %q, want 10", art.CollectionID)
	}
	if art.DefaultLocale != "en" {
		t.Errorf("default locale = %q, want en", art.DefaultLocale)
	}
	if len(art.Variants) != 2 {
		t.Fatalf("variants = %#v, want en+fr", art.Variants)
	}
	if art.Variants["en"].BodyHTML != "<p>EN body</p>" {
		t.Errorf("en body = %q", art.Variants["en"].BodyHTML)
	}
	if art.Variants["fr"].Title != "Configuration" {
		t.Errorf("fr title = %q, want Configuration", art.Variants["fr"].Title)
	}

	// author_id (number 500) resolves to admin id "500".
	if a, ok := corpus.Authors["500"]; !ok || a.Email != "ada@acme.com" {
		t.Errorf("authors = %#v, want 500 -> ada@acme.com", corpus.Authors)
	}

	// Locales: default first.
	if len(corpus.Locales) != 2 || corpus.Locales[0] != "en" || corpus.Locales[1] != "fr" {
		t.Errorf("locales = %#v, want [en fr]", corpus.Locales)
	}
}

func TestValidateReturnsWorkspaceID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me" {
			http.Error(w, "unexpected", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"type":"admin","id":"42","email":"me@acme.com","app":{"id_code":"ws_abc"}}`))
	}))
	t.Cleanup(server.Close)

	src := New(server.URL, "tok", WithHTTPClient(server.Client()))
	ws, err := src.Validate(context.Background())
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if ws != "ws_abc" {
		t.Fatalf("workspace = %q, want ws_abc", ws)
	}
}
