package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

type MessagesService struct {
	client         *Client
	conversationID int
}

type Message struct {
	ID              int                    `json:"id"`
	Content         string                 `json:"content"`
	ContentType     string                 `json:"content_type"`
	ContentAttributes map[string]interface{} `json:"content_attributes"`
	MessageType     int                    `json:"message_type"`
	CreatedAt       int64                  `json:"created_at"`
	Private         bool                   `json:"private"`
	SourceID        *string                `json:"source_id"`
	Sender          *MessageSender         `json:"sender"`
	Attachments     []Attachment           `json:"attachments"`
	Conversation    *MessageConversation   `json:"conversation"`
}

type MessageSender struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Type         string `json:"type"`
	Thumbnail    string `json:"thumbnail"`
	AvailableStatus string `json:"available_status,omitempty"`
}

type Attachment struct {
	ID          int    `json:"id"`
	FileType    string `json:"file_type"`
	DataURL     string `json:"data_url"`
	ThumbURL    string `json:"thumb_url"`
	FileSize    int    `json:"file_size"`
}

type MessageConversation struct {
	ID                 int `json:"id"`
	UnreadCount        int `json:"unread_count"`
	LastActivityAt     int64 `json:"last_activity_at"`
	ContactLastSeenAt  int64 `json:"contact_last_seen_at"`
	AgentLastSeenAt    int64 `json:"agent_last_seen_at"`
}

type MessagesListResponse struct {
	Meta    map[string]interface{} `json:"meta"`
	Payload []Message              `json:"payload"`
}

func (s *MessagesService) List(beforeID int) (*MessagesListResponse, error) {
	params := url.Values{}
	if beforeID > 0 {
		params.Set("before", strconv.Itoa(beforeID))
	}

	path := fmt.Sprintf("/conversations/%d/messages", s.conversationID)
	var resp MessagesListResponse
	if err := s.client.Get(path, params, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

type CreateMessageRequest struct {
	Content     string `json:"content"`
	MessageType string `json:"message_type,omitempty"`
	Private     bool   `json:"private,omitempty"`
	ContentType string `json:"content_type,omitempty"`
}

func (s *MessagesService) Create(content string, private bool) (*Message, error) {
	body := CreateMessageRequest{
		Content:     content,
		MessageType: "outgoing",
		Private:     private,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/conversations/%d/messages", s.conversationID)
	var msg Message
	if err := s.client.Post(path, bytes.NewReader(jsonBody), &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

func (s *MessagesService) Delete(messageID int) error {
	path := fmt.Sprintf("/conversations/%d/messages/%d", s.conversationID, messageID)
	return s.client.Delete(path, nil)
}
