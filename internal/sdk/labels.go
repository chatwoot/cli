package sdk

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type LabelsService struct {
	client         *Client
	conversationID int
}

type LabelsResponse struct {
	Payload []string `json:"payload"`
}

func (s *LabelsService) List() ([]string, error) {
	path := fmt.Sprintf("/conversations/%d/labels", s.conversationID)
	var resp LabelsResponse
	if err := s.client.Get(path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Payload, nil
}

type AddLabelsRequest struct {
	Labels []string `json:"labels"`
}

func (s *LabelsService) Add(labels []string) ([]string, error) {
	body := AddLabelsRequest{Labels: labels}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/conversations/%d/labels", s.conversationID)
	var resp LabelsResponse
	if err := s.client.Post(path, bytes.NewReader(jsonBody), &resp); err != nil {
		return nil, err
	}

	return resp.Payload, nil
}
