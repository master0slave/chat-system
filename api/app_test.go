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
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startApp runs the whole API against real MongoDB, in a database of its own.
func startApp(t *testing.T) client {
	t.Helper()
	cfg := configFromEnv()
	cfg.MongoDB = fmt.Sprintf("test_app_%d", time.Now().UnixNano())
	cfg.JWTSecret = "test-secret"
	router, closeDB, err := newApp(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err, "run `make up` first")
	t.Cleanup(closeDB)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return client{t: t, base: server.URL}
}

// TestSupportConversation is the spec's main story end to end.
func TestSupportConversation(t *testing.T) {
	api := startApp(t)

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

// A busy case: several agents join at once, then everyone sends at once, round after round.
// Nobody may get a 409 just because someone else wrote to the case at the same moment.
func TestBusyCaseAcceptsSimultaneousJoinsAndSends(t *testing.T) {
	api := startApp(t)
	annToken := api.login("Ann", "customer")
	var agentTokens []string
	for i := range 6 {
		agentTokens = append(agentTokens, api.login(fmt.Sprintf("Agent %d", i), "agent"))
	}
	opened := api.do(annToken, http.MethodPost, "/v1/cases", `{"question":"help"}`, http.StatusCreated)
	caseID := opened["case"].(map[string]any)["id"].(string)

	joins := api.statusesAtOnce(agentTokens, http.MethodPost, "/v1/cases/"+caseID+"/join", "")
	assert.Equal(t, repeat(http.StatusOK, len(agentTokens)), joins, "simultaneous joins")

	everyone := append([]string{annToken}, agentTokens...)
	for round := range 10 {
		sends := api.statusesAtOnce(everyone, http.MethodPost, "/v1/cases/"+caseID+"/messages", fmt.Sprintf(`{"body":"round %d"}`, round))
		assert.Equal(t, repeat(http.StatusCreated, len(everyone)), sends, "simultaneous sends, round %d", round)
	}

	c := api.do(annToken, http.MethodGet, "/v1/cases/"+caseID, "", http.StatusOK)
	assert.Len(t, c["participants"], 1+len(agentTokens))
}

// statusesAtOnce sends the same request once per token, all at the same moment, and returns the status codes in token order.
func (c client) statusesAtOnce(tokens []string, method, path, body string) []int {
	c.t.Helper()
	statuses := make([]int, len(tokens))
	errs := make([]error, len(tokens))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, token := range tokens {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
			if err != nil {
				errs[i] = err
				return
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			<-start
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				errs[i] = err
				return
			}
			resp.Body.Close()
			statuses[i] = resp.StatusCode
		}()
	}
	close(start)
	wg.Wait()
	for _, err := range errs {
		require.NoError(c.t, err)
	}
	return statuses
}

func repeat(status, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = status
	}
	return out
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
