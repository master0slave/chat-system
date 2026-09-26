// Package auth implements usecases.Authenticator with HS256 JWTs (ADR 0005).
package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"supportchat/business/models"
)

type JWT struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewJWT(secret []byte, ttl time.Duration) *JWT {
	return &JWT{secret: secret, ttl: ttl, now: time.Now}
}

type claims struct {
	Name string      `json:"name"`
	Role models.Role `json:"role"`
	jwt.RegisteredClaims
}

func (j *JWT) Issue(u models.User) (string, error) {
	now := j.now()
	c := claims{
		Name: u.Name,
		Role: u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(j.secret)
}

func (j *JWT) Verify(token string) (models.User, error) {
	var c claims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return j.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil || c.Subject == "" || !c.Role.Valid() {
		return models.User{}, models.ErrUnauthorized
	}
	return models.User{ID: c.Subject, Name: c.Name, Role: c.Role}, nil
}
