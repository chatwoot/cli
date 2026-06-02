// Package intercom is a small, hand-wrapped client for the Intercom REST API
// (help centers, collections, articles, admins) plus a Source implementation
// that maps Intercom data into the provider-neutral importer IR. It depends
// only on the standard library — no Intercom SDK.
package intercom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.intercom.io"
	defaultVersion = "2.11"
	defaultPerPage = 150
	maxRetries     = 5
)

// Client is an Intercom REST client with bearer auth, version header, cursor
// pagination, and bounded retry/backoff on 429 and 5xx responses.
type Client struct {
	baseURL    string
	token      string
	version    string
	httpClient *http.Client
	sleep      func(time.Duration)
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient injects a custom *http.Client (used in tests).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithVersion overrides the Intercom-Version header.
func WithVersion(v string) Option {
	return func(c *Client) {
		if strings.TrimSpace(v) != "" {
			c.version = v
		}
	}
}

// withSleep overrides the backoff sleep (used in tests to avoid real waits).
func withSleep(fn func(time.Duration)) Option {
	return func(c *Client) { c.sleep = fn }
}

// NewClient builds an Intercom client. An empty baseURL defaults to
// https://api.intercom.io.
func NewClient(baseURL, token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimSuffix(strings.TrimSpace(baseURL), "/"),
		token:      token,
		version:    defaultVersion,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		sleep:      time.Sleep,
	}
	if c.baseURL == "" {
		c.baseURL = defaultBaseURL
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Intercom-Version", c.version)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("intercom request failed: %w", err)
			if attempt < maxRetries {
				c.sleep(backoff(attempt, 0))
				continue
			}
			return lastErr
		}

		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read intercom response: %w", readErr)
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("intercom API %d: %s", resp.StatusCode, snippet(body))
			if attempt < maxRetries {
				c.sleep(backoff(attempt, retryAfter(resp.Header)))
				continue
			}
			return lastErr
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("intercom API %d: %s", resp.StatusCode, snippet(body))
		}
		if out == nil {
			return nil
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode intercom response: %w", err)
		}
		return nil
	}
	return lastErr
}

// fetchList fetches every page of an Intercom cursor-paginated list endpoint.
func fetchList[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var all []T
	startingAfter := ""
	guard := 0
	for {
		q := url.Values{}
		q.Set("per_page", strconv.Itoa(defaultPerPage))
		if startingAfter != "" {
			q.Set("starting_after", startingAfter)
		}

		var env listEnvelope[T]
		if err := c.get(ctx, path, q, &env); err != nil {
			return nil, err
		}
		all = append(all, env.Data...)

		next := env.Pages.nextCursor()
		if next == "" {
			break
		}
		startingAfter = next

		// Safety guard against a misbehaving cursor that never terminates.
		guard++
		if guard > 10000 {
			break
		}
	}
	return all, nil
}

// Me returns the authenticated admin and workspace info (used for validation).
func (c *Client) Me(ctx context.Context) (*meResponse, error) {
	var me meResponse
	if err := c.get(ctx, "/me", nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}

// ListAdmins returns all teammates.
func (c *Client) ListAdmins(ctx context.Context) ([]admin, error) {
	var list adminList
	if err := c.get(ctx, "/admins", nil, &list); err != nil {
		return nil, err
	}
	return list.Admins, nil
}

// ListHelpCenters returns all help centers.
func (c *Client) ListHelpCenters(ctx context.Context) ([]helpCenter, error) {
	return fetchList[helpCenter](ctx, c, "/help_center/help_centers")
}

// ListCollections returns all collections across help centers.
func (c *Client) ListCollections(ctx context.Context) ([]collection, error) {
	return fetchList[collection](ctx, c, "/help_center/collections")
}

// ListArticles returns all articles in the workspace.
func (c *Client) ListArticles(ctx context.Context) ([]article, error) {
	return fetchList[article](ctx, c, "/articles")
}

// backoff returns the wait before a retry: honors a server-provided
// Retry-After (seconds) when present, else exponential (0.5s * 2^attempt)
// capped at 30s.
func backoff(attempt int, retryAfterSecs int) time.Duration {
	if retryAfterSecs > 0 {
		return time.Duration(retryAfterSecs) * time.Second
	}
	d := 500 * time.Millisecond * time.Duration(1<<attempt)
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

func retryAfter(h http.Header) int {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return secs
	}
	return 0
}

func snippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
