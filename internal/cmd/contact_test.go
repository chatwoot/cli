package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/chatwoot/cli/internal/config"
	"github.com/chatwoot/cli/internal/sdk"
)

func TestContactConversationCommandsUseNativeFiltersAndPagination(t *testing.T) {
	setupTestEnv(t)

	wantFilters := []sdk.ConversationFilter{
		{AttributeKey: "contact_id", FilterOperator: "equal_to", Values: []string{"123"}, QueryOperator: "AND"},
		{AttributeKey: "status", FilterOperator: "equal_to", Values: []string{"open"}, QueryOperator: "AND"},
		{AttributeKey: "inbox_id", FilterOperator: "equal_to", Values: []string{"4"}, QueryOperator: "AND"},
		{AttributeKey: "team_id", FilterOperator: "equal_to", Values: []string{"7"}, QueryOperator: "AND"},
		{AttributeKey: "labels", FilterOperator: "equal_to", Values: []string{"billing", "vip"}},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/accounts/1/conversations/filter" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page = %q, want 2", got)
		}

		var body struct {
			Payload []sdk.ConversationFilter `json:"payload"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if !reflect.DeepEqual(body.Payload, wantFilters) {
			t.Errorf("filters = %#v, want %#v", body.Payload, wantFilters)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"meta":{"all_count":1},"payload":[{"id":88,"status":"open","meta":{"channel":"Channel::Email"}}]}`))
	}))
	defer server.Close()

	if err := config.Save(&config.Config{BaseURL: server.URL, AccountID: 1}); err != nil {
		t.Fatalf("config.Save: %v", err)
	}
	app, err := NewApp(&CLI{Output: "json"}, false, "test")
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	var out bytes.Buffer
	app.Printer.Writer = &out

	err = (&ContactConversationsCmd{
		ID:       123,
		Status:   "open",
		Inbox:    4,
		Assignee: "all",
		Team:     7,
		Label:    []string{"billing", "vip"},
		Page:     2,
	}).Run(app)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out.String(), `"id": 88`) {
		t.Fatalf("output = %s, want filtered conversation", out.String())
	}

	out.Reset()
	err = (&ConvsCmd{
		Contact:  123,
		Status:   "open",
		Inbox:    4,
		Assignee: "all",
		Team:     7,
		Label:    []string{"billing", "vip"},
		Page:     2,
	}).Run(app)
	if err != nil {
		t.Fatalf("ConvsCmd.Run: %v", err)
	}
	if requestCount != 2 {
		t.Fatalf("filter request count = %d, want 2", requestCount)
	}
}

func TestConvsRejectsQueryWithContactFilter(t *testing.T) {
	err := (&ConvsCmd{Contact: 123, Query: "refund"}).Run(&App{})
	if err == nil || !strings.Contains(err.Error(), "--query cannot be combined with --contact") {
		t.Fatalf("error = %v", err)
	}
}
