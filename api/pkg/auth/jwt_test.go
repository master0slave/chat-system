package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
)

var ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}

func TestIssueThenVerify(t *testing.T) {
	j := NewJWT([]byte("secret"), time.Hour)

	token, err := j.Issue(ann)
	require.NoError(t, err)
	u, err := j.Verify(token)

	require.NoError(t, err)
	assert.Equal(t, ann, u)
}

func TestVerifyRejectsExpiredToken(t *testing.T) {
	j := NewJWT([]byte("secret"), time.Hour)
	issuedAt := time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)
	j.now = func() time.Time { return issuedAt }
	token, err := j.Issue(ann)
	require.NoError(t, err)

	j.now = func() time.Time { return issuedAt.Add(time.Hour + time.Second) }
	_, err = j.Verify(token)

	assert.ErrorIs(t, err, models.ErrUnauthorized)
}

func TestVerifyRejectsOtherSecretAndGarbage(t *testing.T) {
	token, err := NewJWT([]byte("other"), time.Hour).Issue(ann)
	require.NoError(t, err)

	_, err = NewJWT([]byte("secret"), time.Hour).Verify(token)
	assert.ErrorIs(t, err, models.ErrUnauthorized)

	_, err = NewJWT([]byte("secret"), time.Hour).Verify("not-a-jwt")
	assert.ErrorIs(t, err, models.ErrUnauthorized)
}

func TestVerifyRejectsNoneAlgorithm(t *testing.T) {
	unsigned := jwt.NewWithClaims(jwt.SigningMethodNone, claims{
		Role:             models.RoleAgent,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "agent:eve", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	token, err := unsigned.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = NewJWT([]byte("secret"), time.Hour).Verify(token)

	assert.ErrorIs(t, err, models.ErrUnauthorized)
}
