package sdk

type TeamsService struct {
	client *Client
}

type TeamFull struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AccountID   int    `json:"account_id"`
}

// List returns all teams. The API returns a raw array, not wrapped in payload.
func (s *TeamsService) List() ([]TeamFull, error) {
	var teams []TeamFull
	if err := s.client.Get("/teams", nil, &teams); err != nil {
		return nil, err
	}
	return teams, nil
}
