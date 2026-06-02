package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type HelpCenterService struct {
	client *Client
}

type HelpCenterPortal struct {
	ID           int                    `json:"id"`
	Name         string                 `json:"name"`
	Slug         string                 `json:"slug"`
	CustomDomain string                 `json:"custom_domain"`
	HomepageLink string                 `json:"homepage_link"`
	PageTitle    string                 `json:"page_title"`
	Archived     bool                   `json:"archived"`
	Config       HelpCenterPortalConfig `json:"config"`
	Meta         HelpCenterPortalMeta   `json:"meta"`
}

type HelpCenterPortalConfig struct {
	AllowedLocales []HelpCenterPortalLocale `json:"allowed_locales"`
	DefaultLocale  string                   `json:"default_locale"`
	Layout         string                   `json:"layout"`
}

type HelpCenterPortalLocale struct {
	Code            string `json:"code"`
	ArticlesCount   int    `json:"articles_count"`
	CategoriesCount int    `json:"categories_count"`
	Draft           bool   `json:"draft"`
}

type HelpCenterPortalMeta struct {
	AllArticlesCount int    `json:"all_articles_count"`
	PublishedCount   int    `json:"published_count"`
	CategoriesCount  int    `json:"categories_count"`
	DefaultLocale    string `json:"default_locale"`
}

type HelpCenterPortalsResponse struct {
	Payload []HelpCenterPortal `json:"payload"`
	Meta    map[string]any     `json:"meta"`
}

type HelpCenterArticlesResponse struct {
	Payload []HelpCenterArticle    `json:"payload"`
	Meta    HelpCenterArticlesMeta `json:"meta"`
}

type HelpCenterArticlesMeta struct {
	ArticlesCount int `json:"articles_count"`
}

type HelpCenterArticle struct {
	ID            int                        `json:"id"`
	CategoryID    int                        `json:"category_id"`
	Title         string                     `json:"title"`
	Content       string                     `json:"content"`
	Description   string                     `json:"description"`
	Status        string                     `json:"status"`
	Position      int                        `json:"position"`
	AccountID     int                        `json:"account_id"`
	LastUpdatedAt string                     `json:"last_updated_at"`
	Slug          string                     `json:"slug"`
	Portal        *HelpCenterPublicPortal    `json:"portal"`
	Category      *HelpCenterArticleCategory `json:"category"`
	Views         int                        `json:"views"`
	Link          string                     `json:"link"`
}

type HelpCenterPublicPortal struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	CustomDomain string `json:"custom_domain"`
	PageTitle    string `json:"page_title"`
}

type HelpCenterArticleCategory struct {
	ID     int    `json:"id"`
	Slug   string `json:"slug"`
	Locale string `json:"locale"`
}

type HelpCenterArticlesOptions struct {
	PortalSlug   string
	Locale       string
	CategorySlug string
	Query        string
	Page         int
	PerPage      int
}

func (s *HelpCenterService) ListPortals() (*HelpCenterPortalsResponse, error) {
	var resp HelpCenterPortalsResponse
	if err := s.client.Get("/portals", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *HelpCenterService) ListArticles(opts HelpCenterArticlesOptions) (*HelpCenterArticlesResponse, error) {
	path, err := helpCenterArticlesPath(opts.PortalSlug, opts.Locale, opts.CategorySlug)
	if err != nil {
		return nil, err
	}

	params := url.Values{}
	if opts.Query != "" {
		params.Set("query", opts.Query)
	}
	if opts.Page > 1 || (opts.Page > 0 && opts.PerPage > 0) {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PerPage > 0 {
		params.Set("per_page", strconv.Itoa(opts.PerPage))
	}

	var resp HelpCenterArticlesResponse
	if err := s.client.GetRaw(path, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *HelpCenterService) GetArticle(portalSlug, articleSlug string) (*HelpCenterArticle, error) {
	if strings.TrimSpace(portalSlug) == "" {
		return nil, fmt.Errorf("portal slug is required")
	}
	if strings.TrimSpace(articleSlug) == "" {
		return nil, fmt.Errorf("article slug is required")
	}

	var article HelpCenterArticle
	path := fmt.Sprintf("/hc/%s/articles/%s.json", url.PathEscape(portalSlug), url.PathEscape(articleSlug))
	if err := s.client.GetRaw(path, nil, &article); err != nil {
		return nil, err
	}
	return &article, nil
}

func helpCenterArticlesPath(portalSlug, locale, categorySlug string) (string, error) {
	if strings.TrimSpace(portalSlug) == "" {
		return "", fmt.Errorf("portal slug is required")
	}
	if strings.TrimSpace(locale) == "" {
		return "", fmt.Errorf("locale is required")
	}

	portalSlug = url.PathEscape(portalSlug)
	locale = url.PathEscape(locale)
	if strings.TrimSpace(categorySlug) == "" {
		return fmt.Sprintf("/hc/%s/%s/articles.json", portalSlug, locale), nil
	}

	return fmt.Sprintf(
		"/hc/%s/%s/categories/%s/articles.json",
		portalSlug,
		locale,
		url.PathEscape(categorySlug),
	), nil
}

// ---------------------------------------------------------------------------
// Write API (authoring) — account-scoped, admin token required.
//
// Response wrappers differ per endpoint (verified against the Chatwoot
// controllers/jbuilder views): portal create/update return a BARE portal
// object, while category and article create/update wrap the record under a
// "payload" key.
// ---------------------------------------------------------------------------

// PortalConfigInput is the WRITE shape of a portal's config. Note that
// allowed_locales is a flat []string on write, even though the read API
// renders it as an array of objects.
type PortalConfigInput struct {
	AllowedLocales []string `json:"allowed_locales,omitempty"`
	DefaultLocale  string   `json:"default_locale,omitempty"`
	Layout         string   `json:"layout,omitempty"`
}

// PortalInput is the request body for creating/updating a portal.
type PortalInput struct {
	Name   string             `json:"name,omitempty"`
	Slug   string             `json:"slug,omitempty"`
	Config *PortalConfigInput `json:"config,omitempty"`
}

// CreatePortal creates a help center portal. The response is a bare portal
// object (no payload wrapper).
func (s *HelpCenterService) CreatePortal(req PortalInput) (*HelpCenterPortal, error) {
	body, err := json.Marshal(map[string]PortalInput{"portal": req})
	if err != nil {
		return nil, err
	}

	var portal HelpCenterPortal
	if err := s.client.Post("/portals", bytes.NewReader(body), &portal); err != nil {
		return nil, err
	}
	return &portal, nil
}

// UpdatePortal updates a portal by slug (used to reconcile allowed_locales).
// The response is a bare portal object.
func (s *HelpCenterService) UpdatePortal(slug string, req PortalInput) (*HelpCenterPortal, error) {
	if strings.TrimSpace(slug) == "" {
		return nil, fmt.Errorf("portal slug is required")
	}

	body, err := json.Marshal(map[string]PortalInput{"portal": req})
	if err != nil {
		return nil, err
	}

	var portal HelpCenterPortal
	path := fmt.Sprintf("/portals/%s", url.PathEscape(slug))
	if err := s.client.Patch(path, bytes.NewReader(body), &portal); err != nil {
		return nil, err
	}
	return &portal, nil
}

// HelpCenterCategory is a category record as returned by the write API.
type HelpCenterCategory struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Locale      string `json:"locale"`
	Description string `json:"description"`
	Position    int    `json:"position"`
	AccountID   int    `json:"account_id"`
}

type HelpCenterCategoriesResponse struct {
	Payload []HelpCenterCategory `json:"payload"`
	Meta    map[string]any       `json:"meta"`
}

type categoryEnvelope struct {
	Payload HelpCenterCategory `json:"payload"`
}

// CreateCategoryRequest is the request body for creating a category. slug is
// REQUIRED — unlike articles, the server does not auto-generate it.
type CreateCategoryRequest struct {
	Name                 string `json:"name"`
	Slug                 string `json:"slug"`
	Locale               string `json:"locale"`
	Description          string `json:"description,omitempty"`
	ParentCategoryID     int    `json:"parent_category_id,omitempty"`
	AssociatedCategoryID int    `json:"associated_category_id,omitempty"`
	Position             int    `json:"position,omitempty"`
}

// ListCategories lists a portal's categories, optionally filtered by locale.
func (s *HelpCenterService) ListCategories(portalSlug, locale string) (*HelpCenterCategoriesResponse, error) {
	if strings.TrimSpace(portalSlug) == "" {
		return nil, fmt.Errorf("portal slug is required")
	}

	params := url.Values{}
	if strings.TrimSpace(locale) != "" {
		params.Set("locale", locale)
	}

	var resp HelpCenterCategoriesResponse
	path := fmt.Sprintf("/portals/%s/categories", url.PathEscape(portalSlug))
	if err := s.client.Get(path, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateCategory creates a category under a portal. Returns the created
// category (unwrapped from the payload envelope).
func (s *HelpCenterService) CreateCategory(portalSlug string, req CreateCategoryRequest) (*HelpCenterCategory, error) {
	if strings.TrimSpace(portalSlug) == "" {
		return nil, fmt.Errorf("portal slug is required")
	}

	body, err := json.Marshal(map[string]CreateCategoryRequest{"category": req})
	if err != nil {
		return nil, err
	}

	var env categoryEnvelope
	path := fmt.Sprintf("/portals/%s/categories", url.PathEscape(portalSlug))
	if err := s.client.Post(path, bytes.NewReader(body), &env); err != nil {
		return nil, err
	}
	return &env.Payload, nil
}

type articleEnvelope struct {
	Payload HelpCenterArticle `json:"payload"`
}

// ArticleMetaInput is the optional SEO/meta object on an article.
type ArticleMetaInput struct {
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// CreateArticleRequest is the request body for creating/updating an article.
// title and author_id are required; content is required only when published
// (imports use draft). slug may be omitted — the server auto-generates it.
// associated_article_id is honored at create time and flattened to the root.
type CreateArticleRequest struct {
	Title               string            `json:"title,omitempty"`
	Content             string            `json:"content,omitempty"`
	Description         string            `json:"description,omitempty"`
	Slug                string            `json:"slug,omitempty"`
	Status              string            `json:"status,omitempty"`
	Locale              string            `json:"locale,omitempty"`
	AuthorID            int               `json:"author_id,omitempty"`
	Position            int               `json:"position,omitempty"`
	CategoryID          int               `json:"category_id,omitempty"`
	AssociatedArticleID int               `json:"associated_article_id,omitempty"`
	Meta                *ArticleMetaInput `json:"meta,omitempty"`
}

// CreateArticle creates an article under a portal. Returns the created article
// (unwrapped from the payload envelope).
func (s *HelpCenterService) CreateArticle(portalSlug string, req CreateArticleRequest) (*HelpCenterArticle, error) {
	if strings.TrimSpace(portalSlug) == "" {
		return nil, fmt.Errorf("portal slug is required")
	}

	body, err := json.Marshal(map[string]CreateArticleRequest{"article": req})
	if err != nil {
		return nil, err
	}

	var env articleEnvelope
	path := fmt.Sprintf("/portals/%s/articles", url.PathEscape(portalSlug))
	if err := s.client.Post(path, bytes.NewReader(body), &env); err != nil {
		return nil, err
	}
	return &env.Payload, nil
}

// UpdateArticle updates an article by id (used for re-link/repair). Returns the
// updated article.
func (s *HelpCenterService) UpdateArticle(portalSlug string, id int, req CreateArticleRequest) (*HelpCenterArticle, error) {
	if strings.TrimSpace(portalSlug) == "" {
		return nil, fmt.Errorf("portal slug is required")
	}

	body, err := json.Marshal(map[string]CreateArticleRequest{"article": req})
	if err != nil {
		return nil, err
	}

	var env articleEnvelope
	path := fmt.Sprintf("/portals/%s/articles/%d", url.PathEscape(portalSlug), id)
	if err := s.client.Patch(path, bytes.NewReader(body), &env); err != nil {
		return nil, err
	}
	return &env.Payload, nil
}

// UploadResult is the response from the /upload endpoint. blob_id is an
// ActiveStorage signed_id (a string).
type UploadResult struct {
	FileURL string `json:"file_url"`
	BlobID  string `json:"blob_id"`
}

// UploadImageExternalURL uploads an image by asking the server to fetch a
// remote URL (via SafeFetch). Returns the hosted file_url to embed in article
// content. Uses a multipart form with a single external_url field.
func (s *HelpCenterService) UploadImageExternalURL(externalURL string) (*UploadResult, error) {
	if strings.TrimSpace(externalURL) == "" {
		return nil, fmt.Errorf("external_url is required")
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("external_url", externalURL); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	headers := http.Header{"Content-Type": {w.FormDataContentType()}}
	resp, err := s.client.RequestRaw(http.MethodPost, "/upload", &buf, true, headers)
	if err != nil {
		return nil, err
	}

	var result UploadResult
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode upload response: %w", err)
	}
	return &result, nil
}
