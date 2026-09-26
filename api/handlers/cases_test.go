package handlers_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"supportchat/business/models"
)

var t0 = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

func TestOpenCaseReturns201WithCaseAndMessage(t *testing.T) {
	cases := &stubCases{openCase: func(actor models.User, question string) (models.Case, models.Message, error) {
		assert.Equal(t, ann, actor)
		assert.Equal(t, "How do I reset my password?", question)
		return models.Case{ID: "c1", Status: models.StatusWaiting, CreatedAt: t0, UpdatedAt: t0},
			models.Message{ID: "m1", CaseID: "c1", Body: question, CreatedAt: t0}, nil
	}}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodPost, "/v1/cases", "ann-token", `{"question":"How do I reset my password?"}`)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{
		"case": {"id":"c1","subject":"","customerId":"","participants":null,"status":"waiting",
		         "createdAt":"2026-09-26T09:00:00Z","updatedAt":"2026-09-26T09:00:00Z"},
		"message": {"id":"m1","caseId":"c1","senderId":"","senderName":"","senderRole":"","kind":"",
		            "body":"How do I reset my password?","createdAt":"2026-09-26T09:00:00Z"}
	}`, rec.Body.String())
}

func TestDomainErrorsMapToHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		body   string
	}{
		{models.ErrInvalidInput, http.StatusBadRequest, `{"error":"invalid input"}`},
		{models.ErrForbidden, http.StatusForbidden, `{"error":"forbidden"}`},
		{models.ErrNotFound, http.StatusNotFound, `{"error":"not found"}`},
		{models.ErrCaseClosed, http.StatusConflict, `{"error":"case is closed"}`},
		{models.ErrConflict, http.StatusConflict, `{"error":"case was changed by another request, try again"}`},
		{errors.New("mongo: connection refused at 10.0.0.5"), http.StatusInternalServerError, `{"error":"internal error"}`},
	} {
		cases := &stubCases{sendMessage: func(models.User, string, string) (models.Message, error) {
			return models.Message{}, tc.err
		}}
		e := newRouter(stubAuth{}, cases)

		rec := call(t, e, http.MethodPost, "/v1/cases/c1/messages", "ann-token", `{"body":"hi"}`)

		assert.Equal(t, tc.status, rec.Code, tc.err.Error())
		assert.JSONEq(t, tc.body, rec.Body.String(), "internal details must not leak")
	}
}

func TestListCasesPassesStatusAndNeverReturnsNull(t *testing.T) {
	cases := &stubCases{listCases: func(actor models.User, status models.CaseStatus) ([]models.Case, error) {
		assert.Equal(t, bob, actor)
		assert.Equal(t, models.StatusWaiting, status)
		return nil, nil
	}}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodGet, "/v1/cases?status=waiting", "bob-token", "")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())
}

func TestCaseRoutesPassTheCaseID(t *testing.T) {
	open := models.Case{ID: "c1", Status: models.StatusOpen}
	cases := &stubCases{
		getCase:   func(_ models.User, id string) (models.Case, error) { return models.Case{ID: id}, nil },
		joinCase:  func(_ models.User, id string) (models.Case, error) { return open, nil },
		closeCase: func(_ models.User, id string) (models.Case, error) { return models.Case{ID: id, Status: models.StatusClosed}, nil },
		sendMessage: func(_ models.User, id, body string) (models.Message, error) {
			return models.Message{ID: "m9", CaseID: id, Body: body}, nil
		},
	}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodGet, "/v1/cases/c1", "bob-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"id":"c1"`)

	rec = call(t, e, http.MethodPost, "/v1/cases/c1/join", "bob-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"open"`)

	rec = call(t, e, http.MethodPost, "/v1/cases/c1/messages", "bob-token", `{"body":"hello"}`)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), `"caseId":"c1"`)

	rec = call(t, e, http.MethodPost, "/v1/cases/c1/close", "bob-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"closed"`)
}

func TestListMessagesQueryParameters(t *testing.T) {
	cases := &stubCases{listMessages: func(_ models.User, id, before string, limit int) ([]models.Message, error) {
		assert.Equal(t, "c1", id)
		assert.Equal(t, "m5", before)
		assert.Equal(t, 20, limit)
		return nil, nil
	}}
	e := newRouter(stubAuth{}, cases)

	rec := call(t, e, http.MethodGet, "/v1/cases/c1/messages?before=m5&limit=20", "ann-token", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `[]`, rec.Body.String())

	rec = call(t, e, http.MethodGet, "/v1/cases/c1/messages?limit=abc", "ann-token", "")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
