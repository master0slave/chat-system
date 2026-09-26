package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/handlers"
	"supportchat/realtime"
)

var (
	ann = models.User{ID: "customer:ann", Name: "Ann", Role: models.RoleCustomer}
	bob = models.User{ID: "agent:bob", Name: "Bob", Role: models.RoleAgent}
)

// stubAuth accepts two fixed tokens.
type stubAuth struct {
	login func(name string, role models.Role) (string, models.User, error)
}

func (s stubAuth) Login(_ context.Context, name string, role models.Role) (string, models.User, error) {
	return s.login(name, role)
}

func (stubAuth) Authenticate(_ context.Context, token string) (models.User, error) {
	switch token {
	case "ann-token":
		return ann, nil
	case "bob-token":
		return bob, nil
	}
	return models.User{}, models.ErrUnauthorized
}

// stubCases lets each test set only the use case it expects to be called.
type stubCases struct {
	openCase     func(actor models.User, question string) (models.Case, models.Message, error)
	joinCase     func(actor models.User, caseID string) (models.Case, error)
	sendMessage  func(actor models.User, caseID, body string) (models.Message, error)
	closeCase    func(actor models.User, caseID string) (models.Case, error)
	listCases    func(actor models.User, status models.CaseStatus) ([]models.Case, error)
	getCase      func(actor models.User, caseID string) (models.Case, error)
	listMessages func(actor models.User, caseID, beforeID string, limit int) ([]models.Message, error)
}

func (s *stubCases) OpenCase(_ context.Context, a models.User, q string) (models.Case, models.Message, error) {
	return s.openCase(a, q)
}
func (s *stubCases) JoinCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.joinCase(a, id)
}
func (s *stubCases) SendMessage(_ context.Context, a models.User, id, body string) (models.Message, error) {
	return s.sendMessage(a, id, body)
}
func (s *stubCases) CloseCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.closeCase(a, id)
}
func (s *stubCases) ListCases(_ context.Context, a models.User, st models.CaseStatus) ([]models.Case, error) {
	return s.listCases(a, st)
}
func (s *stubCases) GetCase(_ context.Context, a models.User, id string) (models.Case, error) {
	return s.getCase(a, id)
}
func (s *stubCases) ListMessages(_ context.Context, a models.User, id, before string, limit int) ([]models.Message, error) {
	return s.listMessages(a, id, before, limit)
}

func newRouter(auth stubAuth, cases *stubCases) *echo.Echo {
	return newRouterWithHub(auth, cases, realtime.NewHub(realtime.DefaultBufferSize))
}

func newRouterWithHub(auth stubAuth, cases *stubCases, hub *realtime.Hub) *echo.Echo {
	return handlers.NewRouter(handlers.Config{
		Auth:           auth,
		Cases:          cases,
		Events:         hub,
		AllowedOrigins: []string{"http://localhost:3000"},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// call sends one request. token "" means no Authorization header.
func call(t *testing.T, e *echo.Echo, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

