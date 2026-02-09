package sdk

type AgentsService struct {
	client *Client
}

type AgentFull struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	AvailabilityStatus string `json:"availability_status"`
	Role               string `json:"role"`
	Thumbnail          string `json:"thumbnail"`
	AccountID          int    `json:"account_id"`
}

// List returns all agents. The API returns a raw array, not wrapped in payload.
func (s *AgentsService) List() ([]AgentFull, error) {
	var agents []AgentFull
	if err := s.client.Get("/agents", nil, &agents); err != nil {
		return nil, err
	}
	return agents, nil
}
