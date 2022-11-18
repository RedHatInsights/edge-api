// Package models contains structs, related struct methods, and constants
// This file defines models for events
// FIXME: golangci-lint
// nolint:govet,revive
package models

import (
	identity "github.com/redhatinsights/platform-go-middlewares/identity"
)

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

const (
	/* Event sources (e.g., api, imagemicroservice, devicemicroservice, etc.) */

	// SourceEdgeEventAPI indicates the API service is the source
	SourceEdgeEventAPI string = "urn:redhat:source:edgemanagement:api"

	/* Event types (e.g., image.requested, image.update.requested)
	Doubles as the record key */

	// EventTypeEdgeImageRequested indicates an image has been requested
	EventTypeEdgeImageRequested string = "com.redhat.console.edge.api.image.requested"
	// EventTypeEdgeImageUpdateRequested indicates an image update has been requested
	EventTypeEdgeImageUpdateRequested string = "com.redhat.console.edge.api.image.update.requested"
	// EventTypeEdgeImageISORequested indicates an image update has been requested
	EventTypeEdgeImageISORequested string = "com.redhat.console.edge.api.image.iso.requested"

	// EventTypeEdgeCommitCompleted indicates a commit has completed
	EventTypeEdgeCommitCompleted string = "com.redhat.console.edge.api.image.commit.completed"
	// EventTypeEdgeOstreeRepoCompleted indicates an ostree repo has completed
	EventTypeEdgeOstreeRepoCompleted string = "com.redhat.console.edge.api.image.repo.completed"
	// EventTypeEdgeInstallerCompleted indicates an installer has completed
	EventTypeEdgeInstallerCompleted string = "com.redhat.console.edge.api.image.installer.completed"
)

// CRCCloudEvent is a standard event schema that wraps the Edge-specific "Data" payload
type CRCCloudEvent struct {
	// See https://github.com/cloudevents/spec/blob/v1.0.2/cloudevents/formats/json-format.md for basic schema doc
	// See https://raw.githubusercontent.com/cloudevents/spec/main/cloudevents/formats/cloudevents.json for base CloudEvents schema.
	// See https://github.com/RedHatInsights/event-schemas/blob/main/schemas/events/v1/events.json for CRC event schema

	// the data (or body) unique to the specific event
	Data interface{} `json:"data,omitempty"`

	// Identifies the schema that data adheres to.
	DataSchema string `json:"data_schema"`

	// Identifies the event with a unique ID
	//     id := uuid.New()
	ID string `json:"id"`

	// Red Hat Organization ID
	RedHatOrgID string `json:"redhat_orgid"`

	// Describes the console.redhat.com app that generated the event.
	// e.g., "urn:redhat:source:edgemanagement:api"
	Source string `json:"source"`

	// Specifies the version of the CloudEvents spec targeted.
	// e.g., "v1"
	SpecVersion string `json:"spec_version"`

	// Describes the subject of the event. URN in format urn:redhat:console:$instance_type:$id. The urn may be longer to accommodate hierarchies
	Subject string `json:"subject"`

	// Timestamp of when the occurrence happened. Must adhere to RFC 3339.
	Time string `json:"time"`

	// The type of the event.
	// e.g., "com.redhat.console.edge.api.image.requested"
	//		"com.redhat.console.edge.api.image.update.requested"
	Type string `json:"type"`
}

// EdgeBasePayload describes the edge standard fields for payloads
type EdgeBasePayload struct {
	// The users identity
	Identity identity.XRHID `json:"identity"`

	// Timestamp of when a service interacted with this event. Must adhere to RFC 3339.
	LastHandleTime string `json:"last_handle_time"`

	// Request ID from REST API
	RequestID string `json:"requestid"`
}

// GetIdentity returns the identity from an Edge event
func (epl EdgeBasePayload) GetIdentity() identity.XRHID {
	return epl.Identity
}

// GetRequestID returns the ID of the original REST API request
func (epl EdgeBasePayload) GetRequestID() string {
	return epl.RequestID
}

// EdgeImageRequestedEventPayload provides edge-specific data when an image is requested
type EdgeImageRequestedEventPayload struct {
	EdgeBasePayload
	NewImage Image `json:"new_image"`
}

// EdgeImageUpdateRequestedEventPayload provides edge-specific data when an image update is requested
type EdgeImageUpdateRequestedEventPayload struct {
	EdgeBasePayload
	NewImage Image `json:"new_image"`
}

// EdgeImageISORequestedEventPayload provides edge-specific data when an image iso is requested
type EdgeImageISORequestedEventPayload struct {
	EdgeBasePayload
	NewImage Image `json:"new_image"`
}

// EdgeCommitCompletedEventPayload provides image data when a commit is complete
type EdgeCommitCompletedEventPayload struct {
	EdgeBasePayload
	NewImage Image `json:"new_image"`
}

// EdgeOstreeRepoCompletedEventPayload provides image data when an ostree repo is complete
type EdgeOstreeRepoCompletedEventPayload struct {
	EdgeBasePayload
	NewImage Image `json:"new_image"`
}

// EdgeInstallerCompletedEventPayload provides image data when an installer is complete
type EdgeInstallerCompletedEventPayload struct {
	EdgeBasePayload
	NewImage Image `json:"new_image"`
}
