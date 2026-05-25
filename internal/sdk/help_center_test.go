package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelpCenterListPortalsCallsAccountEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/9/portals" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.Header.Get("api_access_token") != "test-token" {
			t.Errorf("api_access_token = %q, want test-token", r.Header.Get("api_access_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"payload": [{
				"id": 1,
				"name": "Chatwoot User Guide",
				"slug": "user-guide",
				"config": {
					"default_locale": "en",
					"allowed_locales": [{"code": "en", "articles_count": 12, "categories_count": 3}]
				},
				"meta": {"published_count": 12, "categories_count": 3}
			}],
			"meta": {"portals_count": 1}
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	resp, err := client.HelpCenter().ListPortals()
	if err != nil {
		t.Fatalf("ListPortals returned error: %v", err)
	}
	if len(resp.Payload) != 1 || resp.Payload[0].Slug != "user-guide" {
		t.Fatalf("unexpected portals response: %#v", resp)
	}
}

func TestHelpCenterListArticlesBuildsPublicSearchURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hc/user-guide/en/categories/channels/articles.json" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("query"); got != "api channel" {
			t.Errorf("query = %q, want api channel", got)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page = %q, want 2", got)
		}
		if got := r.URL.Query().Get("per_page"); got != "10" {
			t.Errorf("per_page = %q, want 10", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"payload": [{
				"id": 123,
				"category_id": 4,
				"title": "How to create an API channel inbox",
				"content": "You can create an account from settings.",
				"link": "hc/user-guide/articles/1677839703-how-to-create-an-api-channel-inbox"
			}],
			"meta": {"articles_count": 60}
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	resp, err := client.HelpCenter().ListArticles(HelpCenterArticlesOptions{
		PortalSlug:   "user-guide",
		Locale:       "en",
		CategorySlug: "channels",
		Query:        "api channel",
		Page:         2,
		PerPage:      10,
	})
	if err != nil {
		t.Fatalf("ListArticles returned error: %v", err)
	}
	if resp.Meta.ArticlesCount != 60 {
		t.Fatalf("articles_count = %d, want 60", resp.Meta.ArticlesCount)
	}
	if len(resp.Payload) != 1 || resp.Payload[0].Title != "How to create an API channel inbox" {
		t.Fatalf("unexpected articles response: %#v", resp)
	}
}

func TestHelpCenterListArticlesOmitsDefaultPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hc/user-guide/en/articles.json" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if r.URL.RawQuery != "" {
			t.Errorf("raw query = %q, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload": [], "meta": {"articles_count": 0}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	if _, err := client.HelpCenter().ListArticles(HelpCenterArticlesOptions{
		PortalSlug: "user-guide",
		Locale:     "en",
		Page:       1,
	}); err != nil {
		t.Fatalf("ListArticles returned error: %v", err)
	}
}

func TestHelpCenterGetArticleBuildsPublicArticleURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hc/user-guide/articles/1677839703-how-to-create-an-api-channel-inbox.json" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 123,
			"category_id": 4,
			"title": "How to create an API channel inbox",
			"slug": "1677839703-how-to-create-an-api-channel-inbox",
			"content": "Full article body",
			"views": 25,
			"link": "hc/user-guide/articles/1677839703-how-to-create-an-api-channel-inbox"
		}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	article, err := client.HelpCenter().GetArticle("user-guide", "1677839703-how-to-create-an-api-channel-inbox")
	if err != nil {
		t.Fatalf("GetArticle returned error: %v", err)
	}
	if article.ID != 123 || article.Slug != "1677839703-how-to-create-an-api-channel-inbox" {
		t.Fatalf("unexpected article: %#v", article)
	}
}
