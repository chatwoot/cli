package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	BaseURL       string
	APIKey        string
	AccountID     int
	Verbose       bool
	WritesEnabled bool
	httpClient    *http.Client
}

type RawResponse struct {
	StatusCode int
	Status     string
	Header     http.Header
	Body       []byte
}

type ClientOption func(*Client)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

func WithVerbose(verbose bool) ClientOption {
	return func(c *Client) {
		c.Verbose = verbose
	}
}

func WithWritesEnabled(enabled bool) ClientOption {
	return func(c *Client) {
		c.WritesEnabled = enabled
	}
}

func NewClient(baseURL, apiKey string, accountID int, opts ...ClientOption) *Client {
	c := &Client{
		BaseURL:   strings.TrimSuffix(baseURL, "/"),
		APIKey:    apiKey,
		AccountID: accountID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) apiPath(path string) string {
	return fmt.Sprintf("%s/api/v1/accounts/%d%s", c.BaseURL, c.AccountID, path)
}

func (c *Client) request(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, c.apiPath(path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("api_access_token", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (c *Client) rawRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.BaseURL, path), body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("api_access_token", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	return req, nil
}

func (c *Client) do(req *http.Request, v interface{}) error {
	if err := c.requireWritesEnabled(req.Method); err != nil {
		return err
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", req.Method, req.URL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< %s\n", resp.Status)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if v == nil {
		return nil
	}

	if c.Verbose {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response: %w", err)
		}
		fmt.Fprintf(os.Stderr, "< %s\n", redactSensitiveJSON(body))
		if err := json.Unmarshal(body, v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

func (c *Client) doRaw(req *http.Request) (*RawResponse, error) {
	if err := c.requireWritesEnabled(req.Method); err != nil {
		return nil, err
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", req.Method, req.URL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< %s\n", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	return &RawResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Header:     resp.Header.Clone(),
		Body:       body,
	}, nil
}

func (c *Client) requireWritesEnabled(method string) error {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return nil
	}
	if c.WritesEnabled {
		return nil
	}
	return fmt.Errorf("writes are disabled. Run 'chatwoot config writes on' to enable write commands")
}

func redactSensitiveJSON(body []byte) string {
	var value interface{}
	if err := json.Unmarshal(body, &value); err != nil {
		return string(body)
	}

	redactSensitiveFields(value)

	redacted, err := json.Marshal(value)
	if err != nil {
		return string(body)
	}
	return string(redacted)
}

func redactSensitiveFields(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if isSensitiveField(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactSensitiveFields(child)
		}
	case []interface{}:
		for _, child := range typed {
			redactSensitiveFields(child)
		}
	}
}

func isSensitiveField(key string) bool {
	switch strings.ToLower(key) {
	case "access_token", "api_access_token", "pubsub_token":
		return true
	default:
		return false
	}
}

func (c *Client) Get(path string, params url.Values, v interface{}) error {
	fullPath := path
	if len(params) > 0 {
		fullPath = fmt.Sprintf("%s?%s", path, params.Encode())
	}

	req, err := c.request(http.MethodGet, fullPath, nil)
	if err != nil {
		return err
	}

	return c.do(req, v)
}

// GetRaw makes a GET request to a non-account-scoped path (e.g. /api/v1/profile).
func (c *Client) GetRaw(path string, params url.Values, v interface{}) error {
	fullPath := path
	if len(params) > 0 {
		fullPath = fmt.Sprintf("%s?%s", fullPath, params.Encode())
	}

	req, err := c.rawRequest(http.MethodGet, fullPath, nil)
	if err != nil {
		return err
	}

	return c.do(req, v)
}

// RequestRaw makes an authenticated request and returns the response body without decoding it.
// Account-scoped paths are resolved under /api/v1/accounts/{account_id}; exact paths are
// resolved relative to BaseURL.
func (c *Client) RequestRaw(method, path string, body io.Reader, accountScoped bool, headers http.Header) (*RawResponse, error) {
	var req *http.Request
	var err error
	if accountScoped {
		req, err = c.request(method, path, body)
	} else {
		req, err = c.rawRequest(method, path, body)
	}
	if err != nil {
		return nil, err
	}

	for key, values := range headers {
		req.Header.Del(key)
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return c.doRaw(req)
}

func (c *Client) Post(path string, body io.Reader, v interface{}) error {
	req, err := c.request(http.MethodPost, path, body)
	if err != nil {
		return err
	}

	return c.do(req, v)
}

func (c *Client) Patch(path string, body io.Reader, v interface{}) error {
	req, err := c.request(http.MethodPatch, path, body)
	if err != nil {
		return err
	}

	return c.do(req, v)
}

func (c *Client) Delete(path string, v interface{}) error {
	req, err := c.request(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}

	return c.do(req, v)
}

// Conversations returns the conversations service
func (c *Client) Conversations() *ConversationsService {
	return &ConversationsService{client: c}
}

// Messages returns the messages service for a conversation
func (c *Client) Messages(conversationID int) *MessagesService {
	return &MessagesService{client: c, conversationID: conversationID}
}

// Labels returns the labels service for a conversation
func (c *Client) Labels(conversationID int) *LabelsService {
	return &LabelsService{client: c, conversationID: conversationID}
}

// AccountLabels returns the account-level labels service.
func (c *Client) AccountLabels() *AccountLabelsService {
	return &AccountLabelsService{client: c}
}

// Contacts returns the contacts service
func (c *Client) Contacts() *ContactsService {
	return &ContactsService{client: c}
}

// Inboxes returns the inboxes service
func (c *Client) Inboxes() *InboxesService {
	return &InboxesService{client: c}
}

// Agents returns the agents service
func (c *Client) Agents() *AgentsService {
	return &AgentsService{client: c}
}

// Teams returns the teams service
func (c *Client) Teams() *TeamsService {
	return &TeamsService{client: c}
}

// Profile returns the profile service
func (c *Client) Profile() *ProfileService {
	return &ProfileService{client: c}
}
