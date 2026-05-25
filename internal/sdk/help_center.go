package sdk

import (
	"fmt"
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
