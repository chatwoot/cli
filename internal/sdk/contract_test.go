package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

const contractAPIKey = "contract-token"

var (
	contractRouterOnce sync.Once
	contractRouter     routers.Router
	contractRouterErr  error
)

type contractResponse struct {
	status int
	body   string
}

func TestConversationsToggleStatusContract(t *testing.T) {
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		assertJSONBody(t, r, map[string]any{
			"status":        "snoozed",
			"snoozed_until": float64(1757506877),
		})

		return contractResponse{body: `{
			"meta": {},
			"payload": {
				"success": true,
				"current_status": "snoozed",
				"conversation_id": 42
			}
		}`}
	})

	snoozedUntil := int64(1757506877)
	got, err := client.Conversations().ToggleStatus(42, "snoozed", &snoozedUntil)
	if err != nil {
		t.Fatalf("ToggleStatus returned error: %v", err)
	}
	if !got.Success || got.CurrentStatus != "snoozed" || got.ConversationID != 42 {
		t.Fatalf("unexpected toggle response: %#v", got)
	}
}

func TestConversationsAssignContract(t *testing.T) {
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		assertJSONBody(t, r, map[string]any{
			"assignee_id": float64(7),
		})

		return contractResponse{body: userJSON(7, "Ada Lovelace", "ada@example.com")}
	})

	got, err := client.Conversations().Assign(42, 7, 0)
	if err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	if got.ID != 7 || got.Email != "ada@example.com" || got.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected assign response: %#v", got)
	}
}

func TestConversationsListContract(t *testing.T) {
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		assertQuery(t, r.URL.Query(), url.Values{
			"assignee_type": {"all"},
			"status":        {"open"},
			"q":             {"refund"},
			"inbox_id":      {"2"},
			"team_id":       {"3"},
			"labels":        {"vip", "billing"},
			"page":          {"4"},
		})
		if got := r.URL.Query()["labels[]"]; len(got) > 0 {
			t.Fatalf("unexpected labels[] query parameter: %v", got)
		}
		if got := r.URL.Query().Get("sort_by"); got != "" {
			t.Fatalf("unexpected sort_by query parameter: %q", got)
		}

		return contractResponse{body: `{
			"data": {
				"meta": {
					"mine_count": 1,
					"unassigned_count": 0,
					"assigned_count": 1,
					"all_count": 1
				},
				"payload": [
					{
						"id": 42,
						"account_id": 1,
						"inbox_id": 2,
						"status": "open",
						"messages_count": 0,
						"created_at": 1710000000,
						"timestamp": 1710000000,
						"last_activity_at": 1710000100,
						"contact_last_seen_at": 0,
						"agent_last_seen_at": 0,
						"labels": ["vip"],
						"additional_attributes": {},
						"messages": [],
						"meta": {
							"sender": {
								"additional_attributes": {},
								"availability_status": "offline",
								"email": "customer@example.com",
								"id": 9,
								"name": "Customer One",
								"phone_number": "+15550100",
								"blocked": false,
								"identifier": "customer-one",
								"thumbnail": "",
								"custom_attributes": {},
								"last_activity_at": 1710000000,
								"created_at": 1710000000
							},
							"channel": "Channel::WebWidget",
							"hmac_verified": false
						}
					}
				]
			}
		}`}
	})

	got, err := client.Conversations().List(ListOptions{
		Status:       "open",
		InboxID:      2,
		AssigneeType: "all",
		TeamID:       3,
		Query:        "refund",
		Page:         4,
		Labels:       []string{"vip", "billing"},
		SortBy:       "latest",
	})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got.Data.Payload) != 1 {
		t.Fatalf("expected 1 conversation, got %d", len(got.Data.Payload))
	}
	conv := got.Data.Payload[0]
	if conv.ID != 42 || conv.Meta.Sender == nil || conv.Meta.Sender.Name != "Customer One" {
		t.Fatalf("unexpected conversation response: %#v", conv)
	}
}

func TestMessagesListContract(t *testing.T) {
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		assertQuery(t, r.URL.Query(), url.Values{
			"before": {"10"},
			"after":  {"20"},
		})

		return contractResponse{body: `{
			"meta": {
				"labels": ["vip"]
			},
			"payload": [
				{
					"id": 100,
					"content": "Hello",
					"account_id": 1,
					"inbox_id": 2,
					"conversation_id": 42,
					"message_type": 1,
					"created_at": 1710000000,
					"updated_at": 1710000000,
					"private": false,
					"status": "sent",
					"source_id": null,
					"content_type": "text",
					"content_attributes": {},
					"sender_type": "User",
					"sender_id": 7,
					"external_source_ids": {},
					"additional_attributes": {},
					"processed_message_content": "Hello",
					"sentiment": null,
					"conversation": null,
					"attachment": null,
					"sender": {
						"id": 7,
						"name": "Agent One",
						"email": "agent@example.com",
						"type": "user",
						"thumbnail": ""
					}
				}
			]
		}`}
	})

	got, err := client.Messages(42).List(10, 20)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(got.Payload) != 1 || got.Payload[0].ID != 100 || got.Payload[0].Sender.Name != "Agent One" {
		t.Fatalf("unexpected messages response: %#v", got)
	}
}

func TestMessagesCreateContract(t *testing.T) {
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		assertJSONBody(t, r, map[string]any{
			"content":      "Template hello",
			"message_type": "outgoing",
			"private":      true,
			"content_type": "text",
			"content_attributes": map[string]any{
				"priority": "high",
			},
			"campaign_id": float64(12),
			"template_params": map[string]any{
				"name":     "purchase_receipt",
				"category": "UTILITY",
				"language": "en_US",
				"processed_params": map[string]any{
					"body": map[string]any{
						"1": "Ada",
					},
				},
			},
		})

		return contractResponse{body: `{
			"id": 101,
			"content": "Template hello",
			"account_id": 1,
			"inbox_id": 2,
			"conversation_id": 42,
			"message_type": 1,
			"created_at": 1710000000,
			"updated_at": 1710000000,
			"private": true,
			"status": "sent",
			"source_id": null,
			"content_type": "text",
			"content_attributes": {"priority": "high"},
			"sender_type": "User",
			"sender_id": 7,
			"external_source_ids": {},
			"additional_attributes": {},
			"processed_message_content": "Template hello",
			"sentiment": null,
			"conversation": null,
			"attachment": null,
			"sender": {
				"id": 7,
				"name": "Agent One"
			}
		}`}
	})

	got, err := client.Messages(42).CreateWithRequest(CreateMessageRequest{
		Content:     "Template hello",
		MessageType: "outgoing",
		Private:     true,
		ContentType: "text",
		ContentAttributes: map[string]any{
			"priority": "high",
		},
		CampaignID: 12,
		TemplateParams: map[string]any{
			"name":     "purchase_receipt",
			"category": "UTILITY",
			"language": "en_US",
			"processed_params": map[string]any{
				"body": map[string]any{
					"1": "Ada",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWithRequest returned error: %v", err)
	}
	if got.ID != 101 || got.ContentAttributes["priority"] != "high" {
		t.Fatalf("unexpected created message: %#v", got)
	}
}

func TestContactsListAndSearchContract(t *testing.T) {
	var calls int
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		calls++
		switch calls {
		case 1:
			if r.URL.Path != "/api/v1/accounts/1/contacts" {
				t.Fatalf("unexpected list path: %s", r.URL.Path)
			}
			assertQuery(t, r.URL.Query(), url.Values{
				"page": {"2"},
				"sort": {"-last_activity_at"},
			})
			return contractResponse{body: contactsListJSON("2", 9, "Ada Lovelace", "ada@example.com")}
		case 2:
			if r.URL.Path != "/api/v1/accounts/1/contacts/search" {
				t.Fatalf("unexpected search path: %s", r.URL.Path)
			}
			assertQuery(t, r.URL.Query(), url.Values{
				"q":    {"grace"},
				"page": {"3"},
				"sort": {"email"},
			})
			return contractResponse{body: contactsListJSON(3, 10, "Grace Hopper", "grace@example.com")}
		default:
			t.Fatalf("unexpected extra request: %s", r.URL.String())
			return contractResponse{}
		}
	})

	list, err := client.Contacts().List(ContactsListOptions{Page: 2, Sort: "-last_activity_at"})
	if err != nil {
		t.Fatalf("Contacts.List returned error: %v", err)
	}
	if len(list.Payload) != 1 || list.Payload[0].Name != "Ada Lovelace" {
		t.Fatalf("unexpected contacts list: %#v", list)
	}

	search, err := client.Contacts().Search(ContactsSearchOptions{Query: "grace", Page: 3, Sort: "email"})
	if err != nil {
		t.Fatalf("Contacts.Search returned error: %v", err)
	}
	if len(search.Payload) != 1 || search.Payload[0].Email != "grace@example.com" {
		t.Fatalf("unexpected contacts search: %#v", search)
	}
}

func TestProfileGetContract(t *testing.T) {
	client := newContractClient(t, func(t *testing.T, r *http.Request, _ *openapi3filter.RequestValidationInput) contractResponse {
		if r.URL.Path != "/api/v1/profile" {
			t.Fatalf("unexpected profile path: %s", r.URL.Path)
		}
		return contractResponse{body: userJSON(7, "Ada Lovelace", "ada@example.com")}
	})

	got, err := client.Profile().Get()
	if err != nil {
		t.Fatalf("Profile.Get returned error: %v", err)
	}
	if got.ID != 7 || got.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("unexpected profile response: %#v", got)
	}
}

func newContractClient(t *testing.T, handler func(*testing.T, *http.Request, *openapi3filter.RequestValidationInput) contractResponse) *Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		input := validateContractRequest(t, r)
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		resp := handler(t, r, input)
		if resp.status == 0 {
			resp.status = http.StatusOK
		}
		validateContractResponse(t, input, resp.status, []byte(resp.body))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.status)
		if _, err := w.Write([]byte(resp.body)); err != nil {
			t.Fatalf("failed to write response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	return NewClient(server.URL, contractAPIKey, 1, WithHTTPClient(server.Client()))
}

func validateContractRequest(t *testing.T, r *http.Request) *openapi3filter.RequestValidationInput {
	t.Helper()

	if got := r.Header.Get("api_access_token"); got != contractAPIKey {
		t.Fatalf("api_access_token header = %q, want %q", got, contractAPIKey)
	}

	route, pathParams, err := getContractRouter(t).FindRoute(r)
	if err != nil {
		t.Fatalf("failed to find Swagger route for %s %s: %v", r.Method, r.URL.String(), err)
	}

	input := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
		},
	}
	if err := openapi3filter.ValidateRequest(context.Background(), input); err != nil {
		t.Fatalf("request does not match Swagger for %s %s: %v", r.Method, r.URL.String(), err)
	}
	return input
}

func validateContractResponse(t *testing.T, input *openapi3filter.RequestValidationInput, status int, body []byte) {
	t.Helper()

	responseInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: input,
		Status:                 status,
		Header:                 http.Header{"Content-Type": []string{"application/json"}},
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
		},
	}
	responseInput.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), responseInput); err != nil {
		t.Fatalf("response does not match Swagger for %s %s: %v\nbody: %s", input.Request.Method, input.Request.URL.String(), err, string(body))
	}
}

func getContractRouter(t *testing.T) routers.Router {
	t.Helper()

	contractRouterOnce.Do(func() {
		loader := openapi3.NewLoader()
		data, err := os.ReadFile("testdata/application_swagger.json")
		if err != nil {
			contractRouterErr = err
			return
		}
		data, err = normalizeOpenAPINullTypes(data)
		if err != nil {
			contractRouterErr = err
			return
		}
		doc, err := loader.LoadFromData(data)
		if err != nil {
			contractRouterErr = err
			return
		}
		if err := doc.Validate(context.Background(), openapi3.DisableExamplesValidation()); err != nil {
			contractRouterErr = err
			return
		}
		doc.Servers = nil
		contractRouter, contractRouterErr = gorillamux.NewRouter(doc)
	})
	if contractRouterErr != nil {
		t.Fatalf("failed to load Swagger contract: %v", contractRouterErr)
	}
	return contractRouter
}

func normalizeOpenAPINullTypes(data []byte) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	normalizeNullTypes(doc)
	return json.Marshal(doc)
}

func normalizeNullTypes(value any) {
	switch typed := value.(type) {
	case map[string]any:
		if schemaType, ok := typed["type"]; ok {
			switch st := schemaType.(type) {
			case string:
				if st == "null" {
					delete(typed, "type")
					typed["nullable"] = true
				}
			case []any:
				types := make([]any, 0, len(st))
				nullable := false
				for _, item := range st {
					if item == "null" {
						nullable = true
						continue
					}
					types = append(types, item)
				}
				if nullable {
					typed["nullable"] = true
					switch len(types) {
					case 0:
						delete(typed, "type")
					case 1:
						typed["type"] = types[0]
					default:
						typed["type"] = types
					}
				}
			}
		}
		for _, child := range typed {
			normalizeNullTypes(child)
		}
	case []any:
		for _, child := range typed {
			normalizeNullTypes(child)
		}
	}
}

func assertJSONBody(t *testing.T, r *http.Request, want map[string]any) {
	t.Helper()

	var got map[string]any
	if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("JSON body mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}
}

func assertQuery(t *testing.T, got, want url.Values) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("query mismatch:\ngot:  %#v\nwant: %#v", got, want)
	}
}

func userJSON(id int, name, email string) string {
	return `{
		"id": ` + jsonNumber(id) + `,
		"access_token": "token",
		"account_id": 1,
		"available_name": "` + name + `",
		"avatar_url": "https://example.com/avatar.png",
		"confirmed": true,
		"display_name": null,
		"message_signature": null,
		"email": "` + email + `",
		"hmac_identifier": "hmac",
		"inviter_id": null,
		"name": "` + name + `",
		"provider": "email",
		"pubsub_token": "pubsub",
		"role": "agent",
		"ui_settings": {},
		"uid": "` + email + `",
		"type": null,
		"custom_attributes": {},
		"accounts": []
	}`
}

func contactsListJSON(currentPage any, id int, name, email string) string {
	pageBytes, _ := json.Marshal(currentPage)
	return `{
		"meta": {
			"count": 1,
			"current_page": ` + string(pageBytes) + `
		},
		"payload": [
			{
				"additional_attributes": {},
				"availability_status": "offline",
				"email": "` + email + `",
				"id": ` + jsonNumber(id) + `,
				"name": "` + name + `",
				"phone_number": "+15550100",
				"blocked": false,
				"identifier": "` + email + `",
				"thumbnail": "",
				"custom_attributes": {},
				"last_activity_at": 1710000000,
				"created_at": 1700000000,
				"contact_inboxes": []
			}
		]
	}`
}

func jsonNumber(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}
