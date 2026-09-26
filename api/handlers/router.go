// Package handlers is the HTTP and WebSocket adapter. It calls only driving ports (ADR 0001).
package handlers

import (
	"log/slog"
	"net/url"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"supportchat/business/usecases"
)

type Config struct {
	Auth   usecases.AuthService
	Cases  usecases.CaseService
	Events EventSubscriber
	// AllowedOrigins are the web app origins, for example "http://localhost:3000".
	AllowedOrigins []string
	Logger         *slog.Logger
}

// NewRouter registers every route in docs/openapi.yaml and docs/events.md.
func NewRouter(cfg Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = ErrorHandler(cfg.Logger)
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit("64K"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.AllowedOrigins,
		AllowHeaders: []string{"Authorization", "Content-Type"},
	}))

	auth := requireUser(cfg.Auth)
	cases := caseHandlers{cases: cfg.Cases}
	events := eventHandlers{auth: cfg.Auth, cases: cfg.Cases, events: cfg.Events, originPatterns: hosts(cfg.AllowedOrigins)}

	v1 := e.Group("/v1")
	v1.POST("/login", login(cfg.Auth))
	v1.POST("/cases", cases.open, auth)
	v1.GET("/cases", cases.list, auth)
	v1.GET("/cases/:id", cases.get, auth)
	v1.POST("/cases/:id/join", cases.join, auth)
	v1.GET("/cases/:id/messages", cases.listMessages, auth)
	v1.POST("/cases/:id/messages", cases.send, auth)
	v1.POST("/cases/:id/close", cases.close, auth)
	// WebSocket endpoints read the token from ?token= because browsers cannot set headers on a WebSocket.
	v1.GET("/cases/events", events.agentEvents)
	v1.GET("/cases/:id/events", events.caseEvents)
	return e
}

// hosts turns "http://localhost:3000" into "localhost:3000", the form websocket.AcceptOptions expects.
func hosts(origins []string) []string {
	var out []string
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			out = append(out, u.Host)
		}
	}
	return out
}
