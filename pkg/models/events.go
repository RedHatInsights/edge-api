package models

import (
	identity "github.com/redhatinsights/platform-go-middlewares/identity"
)

// AdvisorRecommendation for a system
type AdvisorRecommendation struct {
	PublishDate     string `json:"publish_date"`
	RebootRequired  bool   `json:"reboot_required"`
	RuleDescription string `json:"rule_description"`
	RuleID          string `json:"rule_id"`
	RuleURL         string `json:"rule_url"`
	TotalRisk       string `json:"total_risk"`
}

// ConsoleRedhatComCloudEventsSchema See https://raw.githubusercontent.com/cloudevents/spec/main/cloudevents/formats/cloudevents.json for base CloudEvents schema. NOTE: this schema inlines and further constrains some CloudEvents properties
type ConsoleRedhatComCloudEventsSchema struct {

	// Advisor recommendations for a system
	AdvisorRecommendations []*AdvisorRecommendation `json:"advisor_recommendations,omitempty"`
	Data                   *Data                    `json:"data,omitempty"`

	// Identifies the schema that data adheres to.
	Dataschema string `json:"dataschema"`
	Error      *Error `json:"error,omitempty"`

	// Identifies the event.
	ID string `json:"id"`

	// Describes the console.redhat.com bundle.
	Redhatconsolebundle string `json:"redhatconsolebundle,omitempty"`

	// Red Hat Organization ID
	Redhatorgid string `json:"redhatorgid"`

	// Describes the console.redhat.com app that generated the event.
	Source string `json:"source"`

	// Specifies the version of the CloudEvents spec targeted.
	Specversion string `json:"specversion"`

	// Describes the subject of the event. URN in format urn:redhat:console:$instance_type:$id. The urn may be longer to accommodate hierarchies
	Subject string      `json:"subject"`
	System  *RhelSystem `json:"system,omitempty"`

	// Timestamp of when the occurrence happened. Must adhere to RFC 3339.
	Time string `json:"time"`

	// The type of the event.
	Type string `json:"type"`

	// The users identity
	Identity identity.XRHID `json:"identity"`

	// Timestamp of when a service interacted with this event. Must adhere to RFC 3339.
	Lasthandeltime string `json:"lasthandeltime"`
}

// Data contains optional data for an event
type Data struct {
	AdvisorRecommendations *AdvisorRecommendation `json:"advisor_recommendations,omitempty"`
	Error                  *Error                 `json:"error,omitempty"`
	System                 *RhelSystem            `json:"system,omitempty"`
}

// Error An error reported by an application.
type Error struct {

	// Machine-readable error code that identifies the error.
	Code string `json:"code"`

	// Human readable description of the error.
	Message string `json:"message"`

	// The severity of the error.
	Severity string `json:"severity"`

	// The stack trace/traceback (optional)
	StackTrace string `json:"stack_trace,omitempty"`
}

// RhelSystem A RHEL system managed by console.redhat.com
type RhelSystem struct {
	DisplayName string           `json:"display_name,omitempty"`
	HostURL     string           `json:"host_url,omitempty"`
	Hostname    string           `json:"hostname,omitempty"`
	InventoryID string           `json:"inventory_id"`
	RhelVersion string           `json:"rhel_version,omitempty"`
	Tags        []*RhelSystemTag `json:"tags,omitempty"`
}

// RhelSystemTag tags for a RHEL system
type RhelSystemTag struct {
	Key       string `json:"key"`
	Namespace string `json:"namespace"`
	Value     string `json:"value"`
}

// EdgeInstallerData is data needed for the ISO
type EdgeInstallerData struct {
	SSHKey      string `json:"ssh_key"`
	SSHName     string `json:"ssh_name"`
	DownloadURL string `json:"download_url"`
}

// EdgeCreateCommitEvent wraps the console event with image information
type EdgeCreateCommitEvent struct {
	ConsoleSchema ConsoleRedhatComCloudEventsSchema `json:"consoleschema"`
	NewImage      Image                             `json:"newimage"`
}
