package sdk

import (
	"net/url"
	"strconv"
)

type SearchService struct {
	client *Client
}

type SearchOptions struct {
	Query string
	Page  int
}

type SearchResponse struct {
	Payload SearchPayload `json:"payload"`
}

type SearchPayload struct {
	Conversations []ConversationSearchResult `json:"conversations"`
	Contacts      []ContactSearchResult      `json:"contacts"`
	Messages      []MessageSearchResult      `json:"messages"`
	Articles      []ArticleSearchResult      `json:"articles"`
}

// ConversationSearchResult is the trimmed shape returned by /search — it has
// fewer fields than the regular conversation list (no status, labels, priority).
// ID is the conversation's display_id (account-scoped).
type ConversationSearchResult struct {
	ID        int                  `json:"id"`
	AccountID int                  `json:"account_id"`
	CreatedAt int64                `json:"created_at"`
	Message   *MessageSearchResult `json:"message,omitempty"`
	Contact   *ContactSearchResult `json:"contact,omitempty"`
	Inbox     *Inbox               `json:"inbox,omitempty"`
	Agent     *Agent               `json:"agent,omitempty"`
}

type ContactSearchResult struct {
	ID                   int                    `json:"id"`
	Name                 string                 `json:"name"`
	Email                string                 `json:"email"`
	PhoneNumber          string                 `json:"phone_number"`
	Identifier           string                 `json:"identifier"`
	AdditionalAttributes map[string]interface{} `json:"additional_attributes"`
	LastActivityAt       int64                  `json:"last_activity_at"`
}

// MessageSearchResult mirrors api/v1/models/_message.json.jbuilder: conversation
// is flattened to ConversationID (display_id scalar), not a nested object.
type MessageSearchResult struct {
	ID                int                    `json:"id"`
	Content           string                 `json:"content"`
	InboxID           int                    `json:"inbox_id"`
	EchoID            string                 `json:"echo_id,omitempty"`
	ConversationID    int                    `json:"conversation_id"`
	MessageType       int                    `json:"message_type"`
	ContentType       string                 `json:"content_type"`
	Status            string                 `json:"status"`
	ContentAttributes map[string]interface{} `json:"content_attributes"`
	CreatedAt         int64                  `json:"created_at"`
	Private           bool                   `json:"private"`
	SourceID          *string                `json:"source_id"`
	Sender            *MessageSender         `json:"sender,omitempty"`
	Attachments       []Attachment           `json:"attachments,omitempty"`
}

type ArticleSearchResult struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Locale       string `json:"locale"`
	Content      string `json:"content"`
	Slug         string `json:"slug"`
	PortalSlug   string `json:"portal_slug"`
	AccountID    int    `json:"account_id"`
	CategoryName string `json:"category_name"`
	Status       string `json:"status"`
	UpdatedAt    int64  `json:"updated_at"`
}

// Global searches across conversations, contacts, messages, and help-center
// articles in one call (GET /api/v1/accounts/{id}/search).
func (s *SearchService) Global(opts SearchOptions) (*SearchResponse, error) {
	params := url.Values{}
	params.Set("q", opts.Query)
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}

	var resp SearchResponse
	if err := s.client.Get("/search", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
