package usecases

import (
	"time"

	"supportchat/business/models"
)

type CaseDeps struct {
	Cases       CaseRepository
	Messages    MessageRepository
	Broadcaster MessageBroadcaster
	Clock       Clock
	IDs         IDGenerator
}

type caseService struct {
	CaseDeps
}

func NewCaseService(deps CaseDeps) CaseService {
	return &caseService{CaseDeps: deps}
}

func (s *caseService) newMessage(caseID string, sender models.User, kind models.MessageKind, body string, now time.Time) models.Message {
	return models.Message{
		ID:         s.IDs.NewID(),
		CaseID:     caseID,
		SenderID:   sender.ID,
		SenderName: sender.Name,
		SenderRole: sender.Role,
		Kind:       kind,
		Body:       body,
		CreatedAt:  now,
	}
}
