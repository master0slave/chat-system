package usecases

import (
	"context"

	"supportchat/business/models"
)

type AuthService interface {
	// Login returns a signed token for the user. Name and role are checked; there is no password (ADR 0005).
	Login(ctx context.Context, name string, role models.Role) (string, models.User, error)
	// Authenticate returns the user in a token, or models.ErrUnauthorized.
	Authenticate(ctx context.Context, token string) (models.User, error)
}
