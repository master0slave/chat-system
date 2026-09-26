package handlers

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

// EventSubscriber is what the WebSocket endpoints need from realtime.Hub.
// Each returned channel is closed when cancel is called or the subscriber is dropped for being slow.
type EventSubscriber interface {
	SubscribeCase(caseID string) (events <-chan models.Event, cancel func())
	SubscribeAgents() (events <-chan models.Event, cancel func())
}

const writeTimeout = 5 * time.Second

type eventHandlers struct {
	auth           usecases.AuthService
	cases          usecases.CaseService
	events         EventSubscriber
	originPatterns []string
}

// caseEvents streams one case's events to anyone who may view the case (docs/events.md).
func (h eventHandlers) caseEvents(c echo.Context) error {
	ctx := c.Request().Context()
	u, err := h.auth.Authenticate(ctx, c.QueryParam("token"))
	if err != nil {
		return err
	}
	if _, err := h.cases.GetCase(ctx, u, c.Param("id")); err != nil {
		return err
	}
	events, cancel := h.events.SubscribeCase(c.Param("id"))
	defer cancel()
	return h.stream(c, events)
}

// agentEvents streams case-list events to agents.
func (h eventHandlers) agentEvents(c echo.Context) error {
	u, err := h.auth.Authenticate(c.Request().Context(), c.QueryParam("token"))
	if err != nil {
		return err
	}
	if u.Role != models.RoleAgent {
		return models.ErrForbidden
	}
	events, cancel := h.events.SubscribeAgents()
	defer cancel()
	return h.stream(c, events)
}

// stream upgrades the connection and writes events until the client leaves or the Hub drops it.
// The socket is server-to-client only (ADR 0003): CloseRead ends the stream if the client sends anything.
func (h eventHandlers) stream(c echo.Context, events <-chan models.Event) error {
	conn, err := websocket.Accept(c.Response(), c.Request(), &websocket.AcceptOptions{
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		return nil // Accept has already written the HTTP error response.
	}
	defer conn.CloseNow()

	ctx := conn.CloseRead(c.Request().Context())
	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-events:
			if !ok {
				conn.Close(websocket.StatusTryAgainLater, "client too slow")
				return nil
			}
			wctx, cancel := context.WithTimeout(ctx, writeTimeout)
			err := wsjson.Write(wctx, conn, e)
			cancel()
			if err != nil {
				return nil
			}
		}
	}
}
