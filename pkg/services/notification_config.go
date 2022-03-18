package services

// ImageNotification is the implementation of expected boddy notification
type ImageNotification struct {
	Version     string                  `json:"version"`
	Bundle      string                  `json:"bundle"`
	Application string                  `json:"application"`
	EventType   string                  `json:"event_type"`
	Timestamp   string                  `json:"timestamp"`
	Account     string                  `json:"account_id"`
	Context     string                  `json:"context"`
	Events      []EventNotification     `json:"events"`
	Recipients  []RecipientNotification `json:"recipients"`
}

//EventNotification is used to track events to notification
type EventNotification struct {
	Metadata map[string]string `json:"metadata"`
	Payload  string            `json:"payload"`
}

//RecipientNotification is used to track recipients to notification
type RecipientNotification struct {
	OnlyAdmins            bool     `json:"only_admins"`
	IgnoreUserPreferences bool     `json:"ignore_user_preferences"`
	Users                 []string `json:"users"`
}

const (
	//NotificationTopic to be used
	NotificationTopic = "platform.notifications.ingress"
	//NotificationConfigVersion to be used
	NotificationConfigVersion = "v1.1.0"
	//NotificationConfigBundle to be used
	NotificationConfigBundle = "edge"
	//NotificationConfigApplication to be used
	NotificationConfigApplication = "fleet-management"
	//NotificationConfigEventTypeImage to be used
	NotificationConfigEventTypeImage = "image-creation"
	//NotificationConfigUseres to be used
	NotificationConfigUser = "fleet-management"
)
