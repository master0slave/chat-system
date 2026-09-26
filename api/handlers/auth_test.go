package handlers_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"supportchat/business/models"
)

func TestLogin(t *testing.T) {
	auth := stubAuth{login: func(name string, role models.Role) (string, models.User, error) {
		assert.Equal(t, "Ann", name)
		assert.Equal(t, models.RoleCustomer, role)
		return "ann-token", ann, nil
	}}
	e := newRouter(auth, &stubCases{})

	rec := call(t, e, http.MethodPost, "/v1/login", "", `{"name":"Ann","role":"customer"}`)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"token":"ann-token","user":{"id":"customer:ann","name":"Ann","role":"customer"}}`, rec.Body.String())
}

func TestLoginRejectsBadInput(t *testing.T) {
	auth := stubAuth{login: func(string, models.Role) (string, models.User, error) {
		return "", models.User{}, models.ErrInvalidInput
	}}
	e := newRouter(auth, &stubCases{})

	rec := call(t, e, http.MethodPost, "/v1/login", "", `{"name":"","role":"customer"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.JSONEq(t, `{"error":"invalid input"}`, rec.Body.String())

	rec = call(t, e, http.MethodPost, "/v1/login", "", `{not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestProtectedRoutesNeedAValidToken(t *testing.T) {
	e := newRouter(stubAuth{}, &stubCases{})

	assert.Equal(t, http.StatusUnauthorized, call(t, e, http.MethodGet, "/v1/cases", "", "").Code)
	assert.Equal(t, http.StatusUnauthorized, call(t, e, http.MethodGet, "/v1/cases", "forged", "").Code)
	assert.Equal(t, http.StatusUnauthorized, call(t, e, http.MethodPost, "/v1/cases/c1/messages", "", `{"body":"x"}`).Code)
}
