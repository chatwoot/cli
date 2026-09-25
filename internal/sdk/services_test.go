package sdk

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// recordedRequest is what a stub server saw for one request.
type recordedRequest struct {
	method string
	path   string
	query  string
	body   map[string]any
	header http.Header
}

// stubServer answers every request with status and body, recording the last
// request it received.
func stubServer(t *testing.T, status int, body string) (*Client, *recordedRequest) {
	t.Helper()
	var got recordedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = recordedRequest{method: r.Method, path: r.URL.Path, query: r.URL.RawQuery, header: r.Header.Clone()}
		if data, _ := io.ReadAll(r.Body); len(data) > 0 {
			if err := json.Unmarshal(data, &got.body); err != nil {
				t.Errorf("request body is not JSON: %q", data)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return NewClient(server.URL, "test-token", 3, WithHTTPClient(server.Client())), &got
}

func TestServiceEndpoints(t *testing.T) {
	cases := []struct {
		name       string
		response   string
		call       func(*Client) (any, error)
		wantMethod string
		wantPath   string
		wantBody   map[string]any
		want       any
	}{
		{
			name:       "account labels list",
			response:   `{"payload":[{"id":1,"title":"billing","color":"#fff","show_on_sidebar":true}]}`,
			call:       func(c *Client) (any, error) { return c.AccountLabels().List() },
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/labels",
			want: []AccountLabel{{ID: 1, Title: "billing", Color: "#fff", ShowOnSidebar: true}},
		},
		{
			name:       "agents list is a raw array",
			response:   `[{"id":7,"name":"Ada","role":"agent"}]`,
			call:       func(c *Client) (any, error) { return c.Agents().List() },
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/agents",
			want: []AgentFull{{ID: 7, Name: "Ada", Role: "agent"}},
		},
		{
			name:       "teams list is a raw array",
			response:   `[{"id":2,"name":"Support","account_id":3}]`,
			call:       func(c *Client) (any, error) { return c.Teams().List() },
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/teams",
			want: []TeamFull{{ID: 2, Name: "Support", AccountID: 3}},
		},
		{
			name:       "inboxes list",
			response:   `{"payload":[{"id":5,"name":"Web","channel_type":"Channel::WebWidget"}]}`,
			call:       func(c *Client) (any, error) { return c.Inboxes().List() },
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/inboxes",
			want: &InboxesListResponse{Payload: []InboxFull{{ID: 5, Name: "Web", ChannelType: "Channel::WebWidget"}}},
		},
		{
			name:       "inbox get",
			response:   `{"id":5,"name":"Web","greeting_enabled":true,"greeting_message":"Hi"}`,
			call:       func(c *Client) (any, error) { return c.Inboxes().Get(5) },
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/inboxes/5",
			want: &InboxFull{ID: 5, Name: "Web", GreetingEnabled: true, GreetingMessage: "Hi"},
		},
		{
			name:       "conversation labels list",
			response:   `{"payload":["billing","vip"]}`,
			call:       func(c *Client) (any, error) { return c.Labels(12).List() },
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/conversations/12/labels",
			want: []string{"billing", "vip"},
		},
		{
			name:       "conversation labels add",
			response:   `{"payload":["billing","vip"]}`,
			call:       func(c *Client) (any, error) { return c.Labels(12).Add([]string{"billing", "vip"}) },
			wantMethod: http.MethodPost, wantPath: "/api/v1/accounts/3/conversations/12/labels",
			wantBody: map[string]any{"labels": []any{"billing", "vip"}},
			want:     []string{"billing", "vip"},
		},
		{
			name:     "contact get unwraps payload",
			response: `{"payload":{"id":88,"name":"Jane","email":"jane@example.com"}}`,
			call: func(c *Client) (any, error) {
				contact, err := c.Contacts().Get(88)
				if err != nil {
					return nil, err
				}
				return []any{contact.ID, contact.Name, contact.Email}, nil
			},
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/contacts/88",
			want: []any{88, "Jane", "jane@example.com"},
		},
		{
			name:     "contact conversations",
			response: `{"payload":[{"id":4521,"status":"open"}]}`,
			call: func(c *Client) (any, error) {
				resp, err := c.Contacts().Conversations(88)
				if err != nil {
					return nil, err
				}
				return []any{len(resp.Payload), resp.Payload[0].ID, resp.Payload[0].Status}, nil
			},
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/contacts/88/conversations",
			want: []any{1, 4521, "open"},
		},
		{
			name:     "conversation get",
			response: `{"id":4521,"status":"pending","inbox_id":5}`,
			call: func(c *Client) (any, error) {
				conv, err := c.Conversations().Get(4521)
				if err != nil {
					return nil, err
				}
				return []any{conv.ID, conv.Status, conv.InboxID}, nil
			},
			wantMethod: http.MethodGet, wantPath: "/api/v1/accounts/3/conversations/4521",
			want: []any{4521, "pending", 5},
		},
		{
			name:       "conversation unassign sends assignee_id 0",
			response:   `{}`,
			call:       func(c *Client) (any, error) { return nil, c.Conversations().Unassign(4521) },
			wantMethod: http.MethodPost, wantPath: "/api/v1/accounts/3/conversations/4521/assignments",
			wantBody: map[string]any{"assignee_id": float64(0)},
		},
		{
			name:       "conversation priority",
			response:   `{}`,
			call:       func(c *Client) (any, error) { return nil, c.Conversations().UpdatePriority(4521, "urgent") },
			wantMethod: http.MethodPost, wantPath: "/api/v1/accounts/3/conversations/4521/toggle_priority",
			wantBody: map[string]any{"priority": "urgent"},
		},
		{
			name:       "conversation priority cleared sends null",
			response:   `{}`,
			call:       func(c *Client) (any, error) { return nil, c.Conversations().UpdatePriority(4521, "") },
			wantMethod: http.MethodPost, wantPath: "/api/v1/accounts/3/conversations/4521/toggle_priority",
			wantBody: map[string]any{"priority": nil},
		},
		{
			name:     "message create is an outgoing message",
			response: `{"id":9,"content":"hi","private":true}`,
			call: func(c *Client) (any, error) {
				msg, err := c.Messages(4521).Create("hi", true)
				if err != nil {
					return nil, err
				}
				return []any{msg.ID, msg.Content, msg.Private}, nil
			},
			wantMethod: http.MethodPost, wantPath: "/api/v1/accounts/3/conversations/4521/messages",
			wantBody: map[string]any{"content": "hi", "message_type": "outgoing", "private": true},
			want:     []any{9, "hi", true},
		},
		{
			name:       "message delete",
			response:   ``,
			call:       func(c *Client) (any, error) { return nil, c.Messages(4521).Delete(9) },
			wantMethod: http.MethodDelete, wantPath: "/api/v1/accounts/3/conversations/4521/messages/9",
		},
		{
			name:     "patch",
			response: `{"ok":true}`,
			call: func(c *Client) (any, error) {
				var out map[string]any
				err := c.Patch("/conversations/1", strings.NewReader(`{"status":"open"}`), &out)
				return out, err
			},
			wantMethod: http.MethodPatch, wantPath: "/api/v1/accounts/3/conversations/1",
			wantBody: map[string]any{"status": "open"},
			want:     map[string]any{"ok": true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client, got := stubServer(t, http.StatusOK, tc.response)
			result, err := tc.call(client)
			if err != nil {
				t.Fatalf("call error: %v", err)
			}
			if got.method != tc.wantMethod || got.path != tc.wantPath {
				t.Fatalf("request = %s %s, want %s %s", got.method, got.path, tc.wantMethod, tc.wantPath)
			}
			if got.header.Get("api-access-token") != "test-token" {
				t.Fatalf("api-access-token = %q", got.header.Get("api-access-token"))
			}
			if tc.wantBody != nil && !reflect.DeepEqual(got.body, tc.wantBody) {
				t.Fatalf("body = %#v, want %#v", got.body, tc.wantBody)
			}
			if tc.want != nil && !reflect.DeepEqual(result, tc.want) {
				t.Fatalf("result = %#v, want %#v", result, tc.want)
			}
		})
	}
}

// Every service surfaces API failures as *APIError rather than a zero value.
func TestServicesReturnAPIErrors(t *testing.T) {
	calls := map[string]func(*Client) error{
		"account labels": func(c *Client) error { _, err := c.AccountLabels().List(); return err },
		"agents":         func(c *Client) error { _, err := c.Agents().List(); return err },
		"teams":          func(c *Client) error { _, err := c.Teams().List(); return err },
		"inboxes":        func(c *Client) error { _, err := c.Inboxes().List(); return err },
		"inbox":          func(c *Client) error { _, err := c.Inboxes().Get(1); return err },
		"labels list":    func(c *Client) error { _, err := c.Labels(1).List(); return err },
		"labels add":     func(c *Client) error { _, err := c.Labels(1).Add([]string{"x"}); return err },
		"contacts":       func(c *Client) error { _, err := c.Contacts().List(ContactsListOptions{}); return err },
		"contact":        func(c *Client) error { _, err := c.Contacts().Get(1); return err },
		"contact convs":  func(c *Client) error { _, err := c.Contacts().Conversations(1); return err },
		"contact search": func(c *Client) error { _, err := c.Contacts().Search(ContactsSearchOptions{Query: "x"}); return err },
		"convs":          func(c *Client) error { _, err := c.Conversations().List(ListOptions{}); return err },
		"conv":           func(c *Client) error { _, err := c.Conversations().Get(1); return err },
		"toggle status":  func(c *Client) error { _, err := c.Conversations().ToggleStatus(1, "open", nil); return err },
		"assign":         func(c *Client) error { _, err := c.Conversations().Assign(1, nil, 2); return err },
		"messages":       func(c *Client) error { _, err := c.Messages(1).List(0); return err },
		"message create": func(c *Client) error { _, err := c.Messages(1).Create("x", false); return err },
		"portals":        func(c *Client) error { _, err := c.HelpCenter().ListPortals(); return err },
		"articles": func(c *Client) error {
			_, err := c.HelpCenter().ListArticles(HelpCenterArticlesOptions{PortalSlug: "p", Locale: "en"})
			return err
		},
		"article": func(c *Client) error { _, err := c.HelpCenter().GetArticle("p", "a"); return err },
	}
	client, _ := stubServer(t, http.StatusNotFound, `{"error":"not found"}`)
	for name, call := range calls {
		var apiErr *APIError
		if err := call(client); !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
			t.Errorf("%s: error = %v, want *APIError 404", name, err)
		}
	}
}

func TestClientTransportAndDecodeErrors(t *testing.T) {
	client, _ := stubServer(t, http.StatusOK, `{not json`)
	if _, err := client.Agents().List(); err == nil || !strings.Contains(err.Error(), "failed to decode response") {
		t.Fatalf("invalid JSON error = %v", err)
	}
	client.Verbose = true
	captureStderr(t, func() {
		if _, err := client.Agents().List(); err == nil || !strings.Contains(err.Error(), "failed to decode response") {
			t.Fatalf("verbose invalid JSON error = %v", err)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	down := NewClient(server.URL, "k", 1)
	if _, err := down.Agents().List(); err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("closed server error = %v", err)
	}
	if _, err := down.RequestRaw(http.MethodGet, "/x", nil, true, nil); err == nil || !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("closed server raw error = %v", err)
	}

	// A base URL that can't form a request fails before any network call.
	bad := NewClient("http://[::1", "k", 1)
	for name, call := range map[string]func() error{
		"get":     func() error { return bad.Get("/x", nil, nil) },
		"get raw": func() error { return bad.GetRaw("/x", nil, nil) },
		"post":    func() error { return bad.Post("/x", nil, nil) },
		"patch":   func() error { return bad.Patch("/x", nil, nil) },
		"delete":  func() error { return bad.Delete("/x", nil) },
		"raw scoped": func() error {
			_, err := bad.RequestRaw(http.MethodGet, "/x", nil, true, nil)
			return err
		},
		"raw exact": func() error {
			_, err := bad.RequestRaw(http.MethodGet, "/x", nil, false, nil)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Errorf("%s with a malformed base URL succeeded", name)
		}
	}
}

func TestRequestRawPathsHeadersAndVerbose(t *testing.T) {
	client, got := stubServer(t, http.StatusCreated, `{"id":1}`)

	resp, err := client.RequestRaw(http.MethodPost, "/conversations", strings.NewReader(`{"a":1}`), true,
		http.Header{"Content-Type": {"application/vnd.test+json"}, "X-Extra": {"one", "two"}})
	if err != nil {
		t.Fatalf("RequestRaw: %v", err)
	}
	if got.path != "/api/v1/accounts/3/conversations" || got.method != http.MethodPost {
		t.Fatalf("scoped request = %s %s", got.method, got.path)
	}
	if got.header.Get("Content-Type") != "application/vnd.test+json" || !reflect.DeepEqual(got.header.Values("X-Extra"), []string{"one", "two"}) {
		t.Fatalf("headers = %v, want overrides applied", got.header)
	}
	if resp.StatusCode != http.StatusCreated || string(resp.Body) != `{"id":1}` || resp.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("raw response = %#v", resp)
	}

	client.Verbose = true
	stderr := captureStderr(t, func() {
		if _, err := client.RequestRaw(http.MethodGet, "/api/v1/profile?x=1", nil, false, nil); err != nil {
			t.Fatalf("exact RequestRaw: %v", err)
		}
	})
	if got.path != "/api/v1/profile" || got.query != "x=1" {
		t.Fatalf("exact request path = %s?%s", got.path, got.query)
	}
	if !strings.Contains(stderr, "> GET ") || !strings.Contains(stderr, "< 201") {
		t.Fatalf("verbose raw output = %q", stderr)
	}
}

func TestGetRawEncodesQuery(t *testing.T) {
	client, got := stubServer(t, http.StatusOK, `{}`)
	if err := client.GetRaw("/hc/p/en/articles.json", map[string][]string{"query": {"a b"}}, nil); err != nil {
		t.Fatalf("GetRaw: %v", err)
	}
	if got.path != "/hc/p/en/articles.json" || got.query != "query=a+b" {
		t.Fatalf("request = %s?%s", got.path, got.query)
	}
}

func TestRedactSensitiveJSONFallsBackToSanitizedText(t *testing.T) {
	if got := redactSensitiveJSON([]byte("plain \x1b[31mtext")); got != "plain text" {
		t.Fatalf("non-JSON body = %q", got)
	}
	got := redactSensitiveJSON([]byte(`[{"webhook_secret":"s","items":[{"api_key":"k","name":"ok"}]}]`))
	if strings.Contains(got, `"s"`) || strings.Contains(got, `"k"`) || !strings.Contains(got, `"ok"`) {
		t.Fatalf("arrays not redacted recursively: %s", got)
	}
}

func TestHelpCenterArgumentValidation(t *testing.T) {
	client, _ := stubServer(t, http.StatusOK, `{}`)
	hc := client.HelpCenter()

	for name, call := range map[string]func() error{
		"articles without portal": func() error {
			_, err := hc.ListArticles(HelpCenterArticlesOptions{Locale: "en"})
			return err
		},
		"articles without locale": func() error {
			_, err := hc.ListArticles(HelpCenterArticlesOptions{PortalSlug: "p"})
			return err
		},
		"article without portal": func() error { _, err := hc.GetArticle(" ", "a"); return err },
		"article without slug":   func() error { _, err := hc.GetArticle("p", ""); return err },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "required") {
			t.Errorf("%s: error = %v, want a required-argument error", name, err)
		}
	}
}
