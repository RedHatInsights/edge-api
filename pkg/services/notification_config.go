package services

// ImageNotification is the implementation of expected boddy notification
type ImageNotification struct {
	Version     string                 `json:"version"`
	Bundle      string                 `json:"bundle"`
	Application string                 `json:"application"`
	EventType   string                 `json:"event_type"`
	Timestamp   string                 `json:"timestamp"`
	Account     string                 `json:"account_id"`
	Context     string                 `json:"context"`
	Events      []EventNotification    `json:"events"`
	Recipients  *RecipientNotification `json:"recipients"`
}
type EventNotification struct {
	Metadata string `json:"metadata"`
	Payload  string `json:"payload"`
}
type RecipientNotification struct {
	OnlyAdmins            bool   `json:"only_admins"`
	IgnoreUserPreferences bool   `json:"ignore_user_preferences"`
	Users                 string `json:"users"`
}

const (
	NotificationTopic                = "platform.notifications.ingress"
	NotificationConfigVersion        = "v1.1.0"
	NotificationConfigBundle         = "edge"
	NotificationConfigApplication    = "fleet-management"
	NotificationConfigEventTypeImage = "image-creation"
)
