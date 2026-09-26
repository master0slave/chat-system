package models

import (
	"strings"
	"time"
	"unicode/utf8"
)

type MessageKind string

const (
	KindText   MessageKind = "text"
	KindSystem MessageKind = "system"
)

const MaxBodyLength = 2000

type Message struct {
	ID         string      `json:"id"`
	CaseID     string      `json:"caseId"`
	SenderID   string      `json:"senderId"`
	SenderName string      `json:"senderName"`
	SenderRole Role        `json:"senderRole"`
	Kind       MessageKind `json:"kind"`
	Body       string      `json:"body"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// NormalizeBody trims the text and checks that it has 1 to MaxBodyLength characters.
// Characters are counted as Unicode code points, so Thai text is not penalised for using 3 bytes each.
func NormalizeBody(raw string) (string, error) {
	body := strings.TrimSpace(raw)
	if n := utf8.RuneCountInString(body); n == 0 || n > MaxBodyLength {
		return "", ErrInvalidInput
	}
	return body, nil
}
