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
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Phone       string `json:"phone_number"`
	Thumbnail   string `json:"thumbnail"`
	Identifier  string `json:"identifier"`
}

type Agent struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	Thumbnail   string `json:"thumbnail"`
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

type ConversationsListResponse struct {
	Data struct {
		Meta struct {
			AllCount      int `json:"all_count"`
			AssignedCount int `json:"assigned_count"`
			UnassignedCount int `json:"unassigned_count"`
			MineCount     int `json:"mine_count"`
		} `json:"meta"`
		Payload []Conversation `json:"payload"`
	} `json:"data"`
}

type ListOptions struct {
	Status       string
	InboxID      int
	AssigneeType string
	Page         int
	Labels       []string
	SortBy       string
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
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}
	for _, label := range opts.Labels {
		params.Add("labels[]", label)
	}
	if opts.SortBy != "" {
		params.Set("sort_by", opts.SortBy)
	}

	var resp ConversationsListResponse
	if err := s.client.Get("/conversations", params, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
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
	Success           bool   `json:"success"`
	CurrentStatus     string `json:"current_status"`
	ConversationID    int    `json:"conversation_id"`
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

	var resp ToggleStatusResponse
	if err := s.client.Post(fmt.Sprintf("/conversations/%d/toggle_status", id), bytes.NewReader(jsonBody), &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

type AssignRequest struct {
	AssigneeID int `json:"assignee_id"`
	TeamID     int `json:"team_id,omitempty"`
}

func (s *ConversationsService) Assign(id int, assigneeID int, teamID int) (*Conversation, error) {
	body := AssignRequest{
		AssigneeID: assigneeID,
		TeamID:     teamID,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	var conv Conversation
	if err := s.client.Post(fmt.Sprintf("/conversations/%d/assignments", id), bytes.NewReader(jsonBody), &conv); err != nil {
		return nil, err
	}

	return &conv, nil
}
