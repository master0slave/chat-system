package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
)

type errorResponse struct {
	Error string `json:"error"`
}

// statusFor is the one place that maps domain errors to HTTP statuses (spec section 6).
func statusFor(err error) int {
	switch {
	case errors.Is(err, models.ErrInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, models.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, models.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, models.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, models.ErrCaseClosed), errors.Is(err, models.ErrConflict):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// ErrorHandler writes every error returned by a handler as {"error": "..."}.
// Unknown errors become 500 with a generic message; the details go to the log only.
func ErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status, msg := statusFor(err), err.Error()
		var he *echo.HTTPError
		if errors.As(err, &he) {
			status, msg = he.Code, fmt.Sprint(he.Message)
		}
		if status == http.StatusInternalServerError {
			logger.Error("request failed", "method", c.Request().Method, "path", c.Path(), "err", err)
			msg = "internal error"
		}
		_ = c.JSON(status, errorResponse{Error: msg})
	}
}
