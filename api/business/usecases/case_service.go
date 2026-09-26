package usecases

import (
	"context"
	"errors"
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

const maxUpdateAttempts = 3

// updateCase loads a case, applies change and saves it. When another request saved the
// case in between, it reloads and tries again, so a close never overwrites a concurrent join.
// If change returns an error, updateCase returns the loaded case and that error.
func (s *caseService) updateCase(ctx context.Context, id string, change func(*models.Case) error) (models.Case, error) {
	for attempt := 1; ; attempt++ {
		c, err := s.Cases.Get(ctx, id)
		if err != nil {
			return models.Case{}, err
		}
		if err := change(&c); err != nil {
			return c, err
		}
		err = s.Cases.Update(ctx, c)
		if err == nil {
			c.Version++
			return c, nil
		}
		if !errors.Is(err, models.ErrConflict) || attempt == maxUpdateAttempts {
			return models.Case{}, err
		}
	}
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
