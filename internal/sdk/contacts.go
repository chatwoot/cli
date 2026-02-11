package sdk

import (
	"fmt"
	"net/url"
	"strconv"
)

type ContactsService struct {
	client *Client
}

type ContactFull struct {
	ID               int                    `json:"id"`
	Name             string                 `json:"name"`
	Email            string                 `json:"email"`
	PhoneNumber      string                 `json:"phone_number"`
	Thumbnail        string                 `json:"thumbnail"`
	Identifier       string                 `json:"identifier"`
	CompanyName      string                 `json:"company_name,omitempty"`
	LastActivityAt   int64                  `json:"last_activity_at"`
	CreatedAt        int64                  `json:"created_at"`
	ConversationsCount int                  `json:"conversations_count,omitempty"`
	CustomAttributes     map[string]interface{} `json:"custom_attributes"`
	AdditionalAttributes map[string]interface{} `json:"additional_attributes"`
}

type ContactsListResponse struct {
	Meta    map[string]interface{} `json:"meta"`
	Payload []ContactFull          `json:"payload"`
}

type ContactsListOptions struct {
	Page int
}

func (s *ContactsService) List(opts ContactsListOptions) (*ContactsListResponse, error) {
	params := url.Values{}
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}

	var resp ContactsListResponse
	if err := s.client.Get("/contacts", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *ContactsService) Get(id int) (*ContactFull, error) {
	var resp struct {
		Payload ContactFull `json:"payload"`
	}
	if err := s.client.Get(fmt.Sprintf("/contacts/%d", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Payload, nil
}

type ContactsSearchOptions struct {
	Query string
	Page  int
}

func (s *ContactsService) Search(opts ContactsSearchOptions) (*ContactsListResponse, error) {
	params := url.Values{}
	params.Set("q", opts.Query)
	if opts.Page > 0 {
		params.Set("page", strconv.Itoa(opts.Page))
	}

	var resp ContactsListResponse
	if err := s.client.Get("/contacts/search", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
