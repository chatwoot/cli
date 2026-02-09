package sdk

type ProfileService struct {
	client *Client
}

type ProfileResponse struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	AvailabilityStatus string `json:"availability_status"`
	Role               string `json:"role"`
	Thumbnail          string `json:"thumbnail"`
	AccountID          int    `json:"account_id"`
	UISettings         map[string]interface{} `json:"ui_settings"`
}

// Get fetches the current user's profile. Uses a non-account-scoped endpoint.
func (s *ProfileService) Get() (*ProfileResponse, error) {
	var profile ProfileResponse
	if err := s.client.GetRaw("/api/v1/profile", nil, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}
