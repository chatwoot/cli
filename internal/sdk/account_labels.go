package sdk

type AccountLabelsService struct {
	client *Client
}

type AccountLabel struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Color         string `json:"color"`
	ShowOnSidebar bool   `json:"show_on_sidebar"`
}

type AccountLabelsListResponse struct {
	Payload []AccountLabel `json:"payload"`
}

// List returns all labels defined on the account.
func (s *AccountLabelsService) List() ([]AccountLabel, error) {
	var resp AccountLabelsListResponse
	if err := s.client.Get("/labels", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Payload, nil
}
