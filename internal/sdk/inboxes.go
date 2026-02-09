package sdk

import "fmt"

type InboxesService struct {
	client *Client
}

type InboxFull struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channel_type"`
	AvatarURL   string `json:"avatar_url"`
	ChannelID   int    `json:"channel_id,omitempty"`
	GreetingEnabled bool `json:"greeting_enabled"`
	GreetingMessage string `json:"greeting_message"`
}

type InboxesListResponse struct {
	Payload []InboxFull `json:"payload"`
}

func (s *InboxesService) List() (*InboxesListResponse, error) {
	var resp InboxesListResponse
	if err := s.client.Get("/inboxes", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *InboxesService) Get(id int) (*InboxFull, error) {
	var inbox InboxFull
	if err := s.client.Get(fmt.Sprintf("/inboxes/%d", id), nil, &inbox); err != nil {
		return nil, err
	}
	return &inbox, nil
}
