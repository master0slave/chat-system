package usecases

import (
	"context"
	"strings"
	"unicode/utf8"

	"supportchat/business/models"
)

const MaxNameLength = 50

type authService struct {
	authenticator Authenticator
}

func NewAuthService(a Authenticator) AuthService {
	return &authService{authenticator: a}
}

func (s *authService) Login(_ context.Context, name string, role models.Role) (string, models.User, error) {
	name = strings.Join(strings.Fields(name), " ")
	if n := utf8.RuneCountInString(name); n == 0 || n > MaxNameLength || !role.Valid() {
		return "", models.User{}, models.ErrInvalidInput
	}
	// The ID comes from role + name, so "Ann" gets the same cases every time she logs in.
	u := models.User{ID: string(role) + ":" + strings.ToLower(name), Name: name, Role: role}
	token, err := s.authenticator.Issue(u)
	if err != nil {
		return "", models.User{}, err
	}
	return token, u, nil
}

func (s *authService) Authenticate(_ context.Context, token string) (models.User, error) {
	if token == "" {
		return models.User{}, models.ErrUnauthorized
	}
	return s.authenticator.Verify(token)
}
