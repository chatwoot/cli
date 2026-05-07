package sdk

type User struct {
	ID                 int                    `json:"id"`
	AccessToken        string                 `json:"access_token"`
	AccountID          int                    `json:"account_id"`
	AvailableName      string                 `json:"available_name"`
	AvatarURL          string                 `json:"avatar_url"`
	Confirmed          bool                   `json:"confirmed"`
	DisplayName        *string                `json:"display_name"`
	MessageSignature   *string                `json:"message_signature"`
	Email              string                 `json:"email"`
	HMACIdentifier     string                 `json:"hmac_identifier"`
	InviterID          *int                   `json:"inviter_id"`
	Name               string                 `json:"name"`
	Provider           string                 `json:"provider"`
	PubsubToken        string                 `json:"pubsub_token"`
	Role               string                 `json:"role"`
	UISettings         map[string]interface{} `json:"ui_settings"`
	UID                string                 `json:"uid"`
	Type               *string                `json:"type"`
	CustomAttributes   map[string]interface{} `json:"custom_attributes"`
	AvailabilityStatus string                 `json:"availability_status"`
}
