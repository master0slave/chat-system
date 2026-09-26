package usecases

import "supportchat/business/models"

type Authenticator interface {
	Issue(u models.User) (string, error)
	// Verify returns models.ErrUnauthorized for a malformed, expired or tampered token.
	Verify(token string) (models.User, error)
}
