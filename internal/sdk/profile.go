package sdk

type ProfileService struct {
	client *Client
}

type ProfileResponse struct {
	ID                 int                    `json:"id"`
	Name               string                 `json:"name"`
	Email              string                 `json:"email"`
	AvailabilityStatus string                 `json:"availability_status"`
	Role               string                 `json:"role"`
	Thumbnail          string                 `json:"thumbnail"`
	AvatarURL          string                 `json:"avatar_url"`
	AccountID          int                    `json:"account_id"`
	UISettings         map[string]interface{} `json:"ui_settings"`
	// Accounts lists every account the authenticated user belongs to. Used at
	// login to verify the user has access to the account ID they enter.
	Accounts []ProfileAccount `json:"accounts"`
}

// ProfileAccount is one membership entry from the profile payload's accounts
// array. The endpoint returns more fields (permissions, availability,
// custom_role, ...); only the ones the CLI uses are parsed.
type ProfileAccount struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// Get fetches the current user's profile. Uses a non-account-scoped endpoint.
func (s *ProfileService) Get() (*ProfileResponse, error) {
	var profile ProfileResponse
	if err := s.client.GetRaw("/api/v1/profile", nil, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}
