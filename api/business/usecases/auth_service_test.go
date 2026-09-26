package usecases_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

// fakeAuthenticator issues "token-for:<id>|<name>|<role>" so tests can read what was issued.
type fakeAuthenticator struct{}

func (fakeAuthenticator) Issue(u models.User) (string, error) {
	return "token-for:" + u.ID + "|" + u.Name + "|" + string(u.Role), nil
}

func (fakeAuthenticator) Verify(token string) (models.User, error) {
	rest, ok := strings.CutPrefix(token, "token-for:")
	parts := strings.Split(rest, "|")
	if !ok || len(parts) != 3 {
		return models.User{}, models.ErrUnauthorized
	}
	return models.User{ID: parts[0], Name: parts[1], Role: models.Role(parts[2])}, nil
}

func TestLoginIssuesTokenForUser(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	token, u, err := svc.Login(context.Background(), "  Ann  ", models.RoleCustomer)

	require.NoError(t, err)
	assert.Equal(t, models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}, u)
	assert.Equal(t, "token-for:customer:ann|Ann|customer", token)

	back, err := svc.Authenticate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, u, back)
}

// Review Focus: the same person types their name slightly differently on the next login.
func TestLoginGivesSameIDRegardlessOfCaseAndSpaces(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	_, first, err := svc.Login(context.Background(), "Ann  Lee", models.RoleCustomer)
	require.NoError(t, err)
	_, again, err := svc.Login(context.Background(), " ann lee", models.RoleCustomer)
	require.NoError(t, err)

	assert.Equal(t, first.ID, again.ID)
	assert.Equal(t, "customer:ann lee", again.ID)
}

func TestLoginValidation(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	for name, tc := range map[string]struct {
		name string
		role models.Role
	}{
		"empty name":   {"   ", models.RoleCustomer},
		"long name":    {strings.Repeat("a", usecases.MaxNameLength+1), models.RoleCustomer},
		"unknown role": {"Ann", "admin"},
	} {
		_, _, err := svc.Login(context.Background(), tc.name, tc.role)
		assert.ErrorIs(t, err, models.ErrInvalidInput, name)
	}
}

func TestAuthenticateRejectsEmptyToken(t *testing.T) {
	svc := usecases.NewAuthService(fakeAuthenticator{})

	_, err := svc.Authenticate(context.Background(), "")

	assert.ErrorIs(t, err, models.ErrUnauthorized)
}
