package models

type EventType string

// Event types. docs/events.md is the contract for their payloads.
const (
	EventMessageCreated    EventType = "message.created"
	EventParticipantJoined EventType = "participant.joined"
	EventCaseClosed        EventType = "case.closed"
	EventCaseCreated       EventType = "case.created"
	EventCaseStatusChanged EventType = "case.status_changed"
)

// Event is the envelope sent to WebSocket clients.
type Event struct {
	Type   EventType `json:"type"`
	CaseID string    `json:"caseId"`
	Data   any       `json:"data"`
}
