package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

const userKey = "user"

// requireUser reads "Authorization: Bearer <token>" and stores the user for currentUser.
func requireUser(auth usecases.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token, ok := strings.CutPrefix(c.Request().Header.Get("Authorization"), "Bearer ")
			if !ok {
				return models.ErrUnauthorized
			}
			u, err := auth.Authenticate(c.Request().Context(), token)
			if err != nil {
				return err
			}
			c.Set(userKey, u)
			return next(c)
		}
	}
}

func currentUser(c echo.Context) models.User {
	u, _ := c.Get(userKey).(models.User)
	return u
}

// decodeJSON reads the request body into v. Any malformed body is ErrInvalidInput.
func decodeJSON(c echo.Context, v any) error {
	if err := json.NewDecoder(c.Request().Body).Decode(v); err != nil {
		return models.ErrInvalidInput
	}
	return nil
}

type loginRequest struct {
	Name string      `json:"name"`
	Role models.Role `json:"role"`
}

type loginResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func login(auth usecases.AuthService) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req loginRequest
		if err := decodeJSON(c, &req); err != nil {
			return err
		}
		token, u, err := auth.Login(c.Request().Context(), req.Name, req.Role)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, loginResponse{Token: token, User: u})
	}
}
