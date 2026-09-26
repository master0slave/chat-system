package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"supportchat/business/models"
	"supportchat/realtime"
)

func wsURL(server *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(server.URL, "http") + path
}

func readEvent(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var got map[string]any
	require.NoError(t, wsjson.Read(ctx, conn, &got))
	return got
}

func TestCaseEventsStreamOnlyThatCase(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	cases := &stubCases{getCase: func(_ models.User, id string) (models.Case, error) { return models.Case{ID: id}, nil }}
	server := httptest.NewServer(newRouterWithHub(stubAuth{}, cases, hub))
	defer server.Close()

	conn, _, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/c1/events?token=ann-token"), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	hub.PublishCase("c2", models.Event{Type: models.EventMessageCreated, CaseID: "c2", Data: "not for you"})
	hub.PublishCase("c1", models.Event{Type: models.EventMessageCreated, CaseID: "c1", Data: map[string]string{"body": "hi"}})

	assert.Equal(t, map[string]any{
		"type": "message.created", "caseId": "c1", "data": map[string]any{"body": "hi"},
	}, readEvent(t, conn))
}

func TestCaseEventsRejectBeforeUpgrade(t *testing.T) {
	cases := &stubCases{getCase: func(models.User, string) (models.Case, error) { return models.Case{}, models.ErrForbidden }}
	server := httptest.NewServer(newRouter(stubAuth{}, cases))
	defer server.Close()

	_, resp, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/c1/events?token=bad"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	_, resp, err = websocket.Dial(context.Background(), wsURL(server, "/v1/cases/c1/events?token=ann-token"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestAgentEvents(t *testing.T) {
	hub := realtime.NewHub(realtime.DefaultBufferSize)
	server := httptest.NewServer(newRouterWithHub(stubAuth{}, &stubCases{}, hub))
	defer server.Close()

	_, resp, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/events?token=ann-token"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode, "customers cannot watch the agent feed")

	conn, _, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/events?token=bob-token"), nil)
	require.NoError(t, err)
	defer conn.CloseNow()

	hub.PublishAgents(models.Event{Type: models.EventCaseCreated, CaseID: "c1", Data: nil})

	assert.Equal(t, map[string]any{"type": "case.created", "caseId": "c1", "data": nil}, readEvent(t, conn))
}

func TestEventsRejectOtherOrigins(t *testing.T) {
	server := httptest.NewServer(newRouter(stubAuth{}, &stubCases{}))
	defer server.Close()

	_, resp, err := websocket.Dial(context.Background(), wsURL(server, "/v1/cases/events?token=bob-token"), &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": []string{"http://evil.example"}},
	})

	require.Error(t, err)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
