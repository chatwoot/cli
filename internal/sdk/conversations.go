package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type ConversationsService struct {
	client *Client
}

type Conversation struct {
	ID                   int                    `json:"id"`
	AccountID            int                    `json:"account_id"`
	InboxID              int                    `json:"inbox_id"`
	Status               string                 `json:"status"`
	Priority             *string                `json:"priority"`
	MessagesCount        int                    `json:"messages_count"`
	UnreadCount          int                    `json:"unread_count,omitempty"`
	CreatedAt            int64                  `json:"created_at"`
	Timestamp            int64                  `json:"timestamp"`
	LastActivityAt       int64                  `json:"last_activity_at"`
	ContactLastSeenAt    int64                  `json:"contact_last_seen_at"`
	AgentLastSeenAt      int64                  `json:"agent_last_seen_at"`
	Meta                 ConversationMeta       `json:"meta"`
	Labels               []string               `json:"labels"`
	AdditionalAttributes map[string]interface{} `json:"additional_attributes"`
	Messages             []Message              `json:"messages"`
}

type ConversationMeta struct {
	Sender   *Contact `json:"sender"`
	Assignee *Agent   `json:"assignee"`
	Team     *Team    `json:"team"`
	Channel  string   `json:"channel"`
}

type Contact struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	Phone      string `json:"phone_number"`
	Thumbnail  string `json:"thumbnail"`
	Identifier string `json:"identifier"`
}

type Agent struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Thumbnail          string `json:"thumbnail"`
	AvailabilityStatus string `json:"availability_status"`
}

type Team struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Inbox struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ConversationsListMeta struct {
	AllCount        int `json:"all_count"`
	AssignedCount   int `json:"assigned_count"`
	UnassignedCount int `json:"unassigned_count"`
	MineCount       int `json:"mine_count"`
}

type ConversationsListData struct {
	Meta    ConversationsListMeta `json:"meta"`
	Payload []Conversation        `json:"payload"`
}

type ConversationsListResponse struct {
	Data ConversationsListData `json:"data"`
}

type ListOptions struct {
	Status       string
	InboxID      int
	AssigneeType string
	TeamID       int
	Query        string
	Page         int
	Labels       []string
	SortBy       string
}

type ConversationFilter struct {
	AttributeKey   string   `json:"attribute_key"`
	FilterOperator string   `json:"filter_operator"`
	Values         []string `json:"values,omitempty"`
	QueryOperator  string   `json:"query_operator,omitempty"`
}

type FilterOptions struct {
	Filters []ConversationFilter
	Page    int
}

func (s *ConversationsService) List(opts ListOptions) (*ConversationsListResponse, error) {
	params := url.Values{}

	if opts.Status != "" {
		params.Set("status", opts.Status)
	}
	if opts.InboxID > 0 {
		params.Set("inbox_id", strconv.Itoa(opts.InboxID))
	}
	if opts.AssigneeType != "" {
		params.Set("assignee_type", opts.AssigneeType)
	}
	if opts.TeamID > 0 {
		params.Set("team_id", strconv.Itoa(opts.TeamID))
	}
	if opts.Query != "" {
		params.Set("q", opts.Query)
	}
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	for _, label := range opts.Labels {
		params.Add("labels", label)
	}

	var resp ConversationsListResponse
	if err := s.client.Get("/conversations", params, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

func (s *ConversationsService) Filter(opts FilterOptions) (*ConversationsListResponse, error) {
	body, err := json.Marshal(struct {
		Payload []ConversationFilter `json:"payload"`
	}{Payload: opts.Filters})
	if err != nil {
		return nil, fmt.Errorf("failed to encode conversation filters: %w", err)
	}

	path := "/conversations/filter"
	if opts.Page > 0 {
		path += "?page=" + strconv.Itoa(opts.Page)
	}

	var wireResp struct {
		Meta    ConversationsListMeta `json:"meta"`
		Payload []Conversation        `json:"payload"`
	}
	if err := s.client.Post(path, bytes.NewReader(body), &wireResp); err != nil {
		return nil, err
	}

	return &ConversationsListResponse{
		Data: ConversationsListData{
			Meta:    wireResp.Meta,
			Payload: wireResp.Payload,
		},
	}, nil
}

func (s *ConversationsService) Get(id int) (*Conversation, error) {
	var conv Conversation
	if err := s.client.Get(fmt.Sprintf("/conversations/%d", id), nil, &conv); err != nil {
		return nil, err
	}
	return &conv, nil
}

type ToggleStatusRequest struct {
	Status       string `json:"status"`
	SnoozedUntil *int64 `json:"snoozed_until,omitempty"`
}

type ToggleStatusResponse struct {
	Success        bool   `json:"success"`
	CurrentStatus  string `json:"current_status"`
	ConversationID int    `json:"conversation_id"`
}

func (s *ConversationsService) ToggleStatus(id int, status string, snoozedUntil *int64) (*ToggleStatusResponse, error) {
	body := ToggleStatusRequest{
		Status:       status,
		SnoozedUntil: snoozedUntil,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Payload ToggleStatusResponse `json:"payload"`
	}
	if err := s.client.Post(fmt.Sprintf("/conversations/%d/toggle_status", id), bytes.NewReader(jsonBody), &resp); err != nil {
		return nil, err
	}

	return &resp.Payload, nil
}

type AssignRequest struct {
	AssigneeID *int `json:"assignee_id,omitempty"`
	TeamID     int  `json:"team_id,omitempty"`
}

// Assign updates the conversation's assignee and/or team. Pass a nil
// assigneeID to leave the agent assignment untouched (e.g. team-only
// assignment). Pass a pointer to 0 to explicitly unassign the agent.
func (s *ConversationsService) Assign(id int, assigneeID *int, teamID int) (*User, error) {
	body := AssignRequest{
		AssigneeID: assigneeID,
		TeamID:     teamID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var user User
	if err := s.client.Post(fmt.Sprintf("/conversations/%d/assignments", id), bytes.NewReader(jsonBody), &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *ConversationsService) Unassign(id int) error {
	// Chatwoot treats assignee_id: 0 (not null) as the unassign signal.
	zero := 0
	body := AssignRequest{AssigneeID: &zero}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return s.client.Post(fmt.Sprintf("/conversations/%d/assignments", id), bytes.NewReader(jsonBody), nil)
}

type UpdatePriorityRequest struct {
	Priority *string `json:"priority"`
}

// UpdatePriority sets the conversation priority. Accepted values:
// "urgent", "high", "medium", "low". Pass "" to clear (sends JSON null).
//
// Note: Chatwoot's swagger spec also lists "none" as a valid enum value, but
// the live toggle_priority endpoint returns 500 when sent literally. Sending
// null clears the priority cleanly.
func (s *ConversationsService) UpdatePriority(id int, priority string) error {
	req := UpdatePriorityRequest{}
	if priority != "" {
		req.Priority = &priority
	}
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return s.client.Post(fmt.Sprintf("/conversations/%d/toggle_priority", id), bytes.NewReader(jsonBody), nil)
}
