package models

import (
	"strings"
	"time"
)

type CaseStatus string

const (
	StatusWaiting CaseStatus = "waiting"
	StatusOpen    CaseStatus = "open"
	StatusClosed  CaseStatus = "closed"
)

func (s CaseStatus) Valid() bool {
	return s == StatusWaiting || s == StatusOpen || s == StatusClosed
}

const MaxSubjectLength = 80

type Participant struct {
	UserID   string    `json:"userId"`
	Name     string    `json:"name"`
	Role     Role      `json:"role"`
	JoinedAt time.Time `json:"joinedAt"`
}

type Case struct {
	ID           string        `json:"id"`
	Subject      string        `json:"subject"`
	CustomerID   string        `json:"customerId"`
	Participants []Participant `json:"participants"`
	Status       CaseStatus    `json:"status"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	ClosedAt     *time.Time    `json:"closedAt,omitempty"`
	// Version supports optimistic concurrency: CaseRepository.Update only saves
	// when the stored version still equals this one.
	Version int `json:"-"`
}

// CanView reports whether u may read this case: every agent, or the customer who owns it.
func (c Case) CanView(u User) bool {
	return u.Role == RoleAgent || (u.Role == RoleCustomer && c.CustomerID == u.ID)
}

func (c Case) HasParticipant(userID string) bool {
	for _, p := range c.Participants {
		if p.UserID == userID {
			return true
		}
	}
	return false
}

// SubjectFrom turns the first question into a one-line subject of at most MaxSubjectLength characters.
func SubjectFrom(question string) string {
	oneLine := []rune(strings.Join(strings.Fields(question), " "))
	if len(oneLine) > MaxSubjectLength {
		oneLine = oneLine[:MaxSubjectLength]
	}
	return string(oneLine)
}
