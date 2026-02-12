package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	AccountID  int
	httpClient *http.Client
}

type ClientOption func(*Client)

func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
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

func (c *Client) do(req *http.Request, v interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
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
	fullURL := fmt.Sprintf("%s%s", c.BaseURL, path)
	if len(params) > 0 {
		fullURL = fmt.Sprintf("%s?%s", fullURL, params.Encode())
	}

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("api_access_token", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	return c.do(req, v)
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
