package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"supportchat/business/models"
	"supportchat/business/usecases"
)

type caseHandlers struct {
	cases usecases.CaseService
}

type openCaseRequest struct {
	Question string `json:"question"`
}

type openCaseResponse struct {
	Case    models.Case    `json:"case"`
	Message models.Message `json:"message"`
}

type sendMessageRequest struct {
	Body string `json:"body"`
}

func (h caseHandlers) open(c echo.Context) error {
	var req openCaseRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	cs, m, err := h.cases.OpenCase(c.Request().Context(), currentUser(c), req.Question)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, openCaseResponse{Case: cs, Message: m})
}

func (h caseHandlers) list(c echo.Context) error {
	status := models.CaseStatus(c.QueryParam("status"))
	cs, err := h.cases.ListCases(c.Request().Context(), currentUser(c), status)
	if err != nil {
		return err
	}
	if cs == nil {
		cs = []models.Case{}
	}
	return c.JSON(http.StatusOK, cs)
}

func (h caseHandlers) get(c echo.Context) error {
	cs, err := h.cases.GetCase(c.Request().Context(), currentUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}

func (h caseHandlers) join(c echo.Context) error {
	cs, err := h.cases.JoinCase(c.Request().Context(), currentUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}

func (h caseHandlers) listMessages(c echo.Context) error {
	limit := 0
	if raw := c.QueryParam("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return models.ErrInvalidInput
		}
		limit = n
	}
	ms, err := h.cases.ListMessages(c.Request().Context(), currentUser(c), c.Param("id"), c.QueryParam("before"), limit)
	if err != nil {
		return err
	}
	if ms == nil {
		ms = []models.Message{}
	}
	return c.JSON(http.StatusOK, ms)
}

func (h caseHandlers) send(c echo.Context) error {
	var req sendMessageRequest
	if err := decodeJSON(c, &req); err != nil {
		return err
	}
	m, err := h.cases.SendMessage(c.Request().Context(), currentUser(c), c.Param("id"), req.Body)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, m)
}

func (h caseHandlers) close(c echo.Context) error {
	cs, err := h.cases.CloseCase(c.Request().Context(), currentUser(c), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cs)
}
