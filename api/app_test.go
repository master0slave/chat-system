package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSupportConversation runs the whole API against real MongoDB: the spec's main story end to end.
func TestSupportConversation(t *testing.T) {
	cfg := configFromEnv()
	cfg.MongoDB = fmt.Sprintf("test_app_%d", time.Now().UnixNano())
	cfg.JWTSecret = "test-secret"
	ctx := context.Background()
	router, closeDB, err := newApp(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err, "run `make up` first")
	defer closeDB()
	server := httptest.NewServer(router)
	defer server.Close()
	api := client{t: t, base: server.URL}

	annToken := api.login("Ann", "customer")
	bobToken := api.login("Bob", "agent")

	agentFeed := api.dial("/v1/cases/events?token=" + bobToken)
	opened := api.do(annToken, http.MethodPost, "/v1/cases", `{"question":"How do I reset my password?"}`, http.StatusCreated)
	caseID := opened["case"].(map[string]any)["id"].(string)
	assert.Equal(t, "case.created", read(t, agentFeed)["type"])

	annRoom := api.dial("/v1/cases/" + caseID + "/events?token=" + annToken)
	api.do(bobToken, http.MethodPost, "/v1/cases/"+caseID+"/join", "", http.StatusOK)
	assert.Equal(t, "participant.joined", read(t, annRoom)["type"])
	assert.Equal(t, "message.created", read(t, annRoom)["type"]) // "Bob joined the case"

	api.do(bobToken, http.MethodPost, "/v1/cases/"+caseID+"/messages", `{"body":"Click 'Forgot password'"}`, http.StatusCreated)
	reply := read(t, annRoom)
	assert.Equal(t, "Click 'Forgot password'", reply["data"].(map[string]any)["body"], "Ann sees Bob's reply without refreshing")

	api.do(bobToken, http.MethodPost, "/v1/cases/"+caseID+"/close", "", http.StatusOK)
	api.do(annToken, http.MethodPost, "/v1/cases/"+caseID+"/messages", `{"body":"one more thing"}`, http.StatusConflict)

	catToken := api.login("Cat", "customer")
	api.do(catToken, http.MethodGet, "/v1/cases/"+caseID, "", http.StatusForbidden)

	history := api.doList(annToken, "/v1/cases/"+caseID+"/messages")
	var bodies []string
	for _, m := range history {
		bodies = append(bodies, m["body"].(string))
	}
	assert.Equal(t, []string{
		"Case closed by Bob", "Click 'Forgot password'", "Bob joined the case", "How do I reset my password?",
	}, bodies)
}

type client struct {
	t    *testing.T
	base string
}

func (c client) login(name, role string) string {
	res := c.do("", http.MethodPost, "/v1/login", fmt.Sprintf(`{"name":%q,"role":%q}`, name, role), http.StatusOK)
	return res["token"].(string)
}

func (c client) send(token, method, path, body string, wantStatus int) []byte {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, bytes.NewBufferString(body))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	require.Equal(c.t, wantStatus, resp.StatusCode, "%s %s: %s", method, path, raw)
	return raw
}

func (c client) do(token, method, path, body string, wantStatus int) map[string]any {
	c.t.Helper()
	var out map[string]any
	require.NoError(c.t, json.Unmarshal(c.send(token, method, path, body, wantStatus), &out))
	return out
}

func (c client) doList(token, path string) []map[string]any {
	c.t.Helper()
	var out []map[string]any
	require.NoError(c.t, json.Unmarshal(c.send(token, http.MethodGet, path, "", http.StatusOK), &out))
	return out
}

func (c client) dial(path string) *websocket.Conn {
	c.t.Helper()
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(c.base, "http")+path, nil)
	require.NoError(c.t, err)
	c.t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func read(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var e map[string]any
	require.NoError(t, wsjson.Read(ctx, conn, &e))
	return e
}

