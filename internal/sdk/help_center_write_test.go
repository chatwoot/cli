package sdk

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeBody decodes a request's JSON body into a generic map for assertions.
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode body %q: %v", string(raw), err)
	}
	return m
}

func TestCreatePortalSendsConfigAndDecodesBareResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/accounts/9/portals" {
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		body := decodeBody(t, r)
		portal, ok := body["portal"].(map[string]any)
		if !ok {
			t.Fatalf("body not wrapped under portal: %#v", body)
		}
		if portal["slug"] != "acme-support" {
			t.Errorf("slug = %v, want acme-support", portal["slug"])
		}
		cfg, ok := portal["config"].(map[string]any)
		if !ok {
			t.Fatalf("config missing: %#v", portal)
		}
		locales, ok := cfg["allowed_locales"].([]any)
		if !ok || len(locales) != 2 || locales[0] != "en" || locales[1] != "fr" {
			t.Errorf("allowed_locales = %#v, want [en fr] as strings", cfg["allowed_locales"])
		}

		w.Header().Set("Content-Type", "application/json")
		// Bare portal object — NO payload wrapper.
		_, _ = w.Write([]byte(`{"id": 7, "name": "Acme Support", "slug": "acme-support"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	portal, err := client.HelpCenter().CreatePortal(PortalInput{
		Name: "Acme Support",
		Slug: "acme-support",
		Config: &PortalConfigInput{
			AllowedLocales: []string{"en", "fr"},
			DefaultLocale:  "en",
		},
	})
	if err != nil {
		t.Fatalf("CreatePortal: %v", err)
	}
	if portal.ID != 7 || portal.Slug != "acme-support" {
		t.Fatalf("unexpected portal: %#v", portal)
	}
}

func TestUpdatePortalPatchesAllowedLocales(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/accounts/9/portals/acme-support" {
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		body := decodeBody(t, r)
		cfg := body["portal"].(map[string]any)["config"].(map[string]any)
		locales := cfg["allowed_locales"].([]any)
		if len(locales) != 3 {
			t.Errorf("allowed_locales len = %d, want 3", len(locales))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 7, "slug": "acme-support"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	if _, err := client.HelpCenter().UpdatePortal("acme-support", PortalInput{
		Config: &PortalConfigInput{AllowedLocales: []string{"en", "fr", "de"}},
	}); err != nil {
		t.Fatalf("UpdatePortal: %v", err)
	}
}

func TestCreateCategorySendsSlugAndLinksAndUnwrapsPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/accounts/9/portals/acme-support/categories" {
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		cat := decodeBody(t, r)["category"].(map[string]any)
		if cat["slug"] == nil || cat["slug"] == "" {
			t.Errorf("slug must be present, got %#v", cat["slug"])
		}
		if cat["locale"] != "fr" {
			t.Errorf("locale = %v, want fr", cat["locale"])
		}
		if cat["associated_category_id"] != float64(11) {
			t.Errorf("associated_category_id = %v, want 11", cat["associated_category_id"])
		}
		if cat["parent_category_id"] != float64(5) {
			t.Errorf("parent_category_id = %v, want 5", cat["parent_category_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload": {"id": 21, "slug": "getting-started", "locale": "fr"}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	cat, err := client.HelpCenter().CreateCategory("acme-support", CreateCategoryRequest{
		Name:                 "Commencer",
		Slug:                 "getting-started",
		Locale:               "fr",
		ParentCategoryID:     5,
		AssociatedCategoryID: 11,
	})
	if err != nil {
		t.Fatalf("CreateCategory: %v", err)
	}
	if cat.ID != 21 || cat.Locale != "fr" {
		t.Fatalf("unexpected category: %#v", cat)
	}
}

func TestListCategoriesPassesLocaleQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/9/portals/acme-support/categories" {
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if got := r.URL.Query().Get("locale"); got != "en" {
			t.Errorf("locale = %q, want en", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload": [{"id": 3, "slug": "faq", "locale": "en"}], "meta": {"categories_count": 1}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	resp, err := client.HelpCenter().ListCategories("acme-support", "en")
	if err != nil {
		t.Fatalf("ListCategories: %v", err)
	}
	if len(resp.Payload) != 1 || resp.Payload[0].Slug != "faq" {
		t.Fatalf("unexpected categories: %#v", resp)
	}
}

func TestCreateArticleSendsLinkLocaleStatusAndUnwrapsPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/accounts/9/portals/acme-support/articles" {
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		art := decodeBody(t, r)["article"].(map[string]any)
		if art["status"] != "draft" {
			t.Errorf("status = %v, want draft", art["status"])
		}
		if art["locale"] != "fr" {
			t.Errorf("locale = %v, want fr", art["locale"])
		}
		if art["associated_article_id"] != float64(100) {
			t.Errorf("associated_article_id = %v, want 100", art["associated_article_id"])
		}
		if art["category_id"] != float64(21) {
			t.Errorf("category_id = %v, want 21", art["category_id"])
		}
		if art["author_id"] != float64(2) {
			t.Errorf("author_id = %v, want 2", art["author_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload": {"id": 101, "title": "Configurer SSO", "status": "draft"}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	art, err := client.HelpCenter().CreateArticle("acme-support", CreateArticleRequest{
		Title:               "Configurer SSO",
		Content:             "<p>...</p>",
		Status:              "draft",
		Locale:              "fr",
		AuthorID:            2,
		CategoryID:          21,
		AssociatedArticleID: 100,
	})
	if err != nil {
		t.Fatalf("CreateArticle: %v", err)
	}
	if art.ID != 101 || art.Status != "draft" {
		t.Fatalf("unexpected article: %#v", art)
	}
}

func TestUpdateArticlePatchesByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/accounts/9/portals/acme-support/articles/101" {
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payload": {"id": 101, "title": "Configurer SSO"}}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	if _, err := client.HelpCenter().UpdateArticle("acme-support", 101, CreateArticleRequest{
		AssociatedArticleID: 100,
	}); err != nil {
		t.Fatalf("UpdateArticle: %v", err)
	}
}

func TestUploadImageExternalURLSendsMultipartField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/accounts/9/upload" {
			http.Error(w, "unexpected: "+r.Method+" "+r.URL.Path, http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		if got := r.FormValue("external_url"); got != "https://cdn.intercom.io/img.png" {
			t.Errorf("external_url = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"file_url": "https://app/rails/active_storage/blobs/x/img.png", "blob_id": "signed-abc"}`))
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "test-token", 9, WithHTTPClient(server.Client()))
	res, err := client.HelpCenter().UploadImageExternalURL("https://cdn.intercom.io/img.png")
	if err != nil {
		t.Fatalf("UploadImageExternalURL: %v", err)
	}
	if res.BlobID != "signed-abc" || res.FileURL == "" {
		t.Fatalf("unexpected upload result: %#v", res)
	}
}
