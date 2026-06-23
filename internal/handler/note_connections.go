package handler

import (
	"errors"
	"net/http"
	"strconv"

	"smarticky/ent"
	connectsvc "smarticky/internal/connections"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *Handler) ListNoteConnectionAccounts(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accounts, err := h.connections.ListAccounts(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list note connection accounts"})
	}
	return c.JSON(http.StatusOK, accounts)
}

func (h *Handler) CreateNoteConnectionAccount(c echo.Context) error {
	userID := c.Get("user_id").(int)
	var req connectsvc.AccountInput
	if err := bindStrictJSON(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account request"})
	}
	account, err := h.connections.CreateAccount(c.Request().Context(), userID, req)
	if err != nil {
		return noteConnectionError(c, err)
	}
	return c.JSON(http.StatusCreated, account)
}

func (h *Handler) UpdateNoteConnectionAccount(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accountID, err := noteConnectionAccountID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
	}
	var req connectsvc.AccountInput
	if err := bindStrictJSON(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account request"})
	}
	account, err := h.connections.UpdateAccount(c.Request().Context(), userID, accountID, req)
	if err != nil {
		return noteConnectionError(c, err)
	}
	return c.JSON(http.StatusOK, account)
}

func (h *Handler) DeleteNoteConnectionAccount(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accountID, err := noteConnectionAccountID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
	}
	if err := h.connections.DeleteAccount(c.Request().Context(), userID, accountID); err != nil {
		return noteConnectionError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) TestUnsavedNoteConnectionAccount(c echo.Context) error {
	var req connectsvc.AccountInput
	if err := bindStrictJSON(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account request"})
	}
	if err := h.connections.TestUnsaved(c.Request().Context(), req); err != nil {
		return noteConnectionError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": connectsvc.StatusSuccess})
}

func (h *Handler) TestNoteConnectionAccount(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accountID, err := noteConnectionAccountID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
	}
	var req struct {
		Token *string `json:"token,omitempty"`
	}
	if c.Request().Body != nil {
		_ = c.Bind(&req)
	}
	account, err := h.connections.TestAccount(c.Request().Context(), userID, accountID, req.Token)
	if err != nil {
		return c.JSON(http.StatusOK, map[string]any{
			"status":  connectsvc.StatusFailed,
			"account": account,
			"error":   err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"status":  connectsvc.StatusSuccess,
		"account": account,
	})
}

func (h *Handler) ListNoteConnectionTargets(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accountID, err := noteConnectionAccountID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
	}
	targets, err := h.connections.ListTargets(c.Request().Context(), userID, accountID)
	if err != nil {
		return noteConnectionError(c, err)
	}
	return c.JSON(http.StatusOK, targets)
}

func (h *Handler) ImportNoteConnection(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accountID, err := noteConnectionAccountID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
	}
	var req connectsvc.ImportRequest
	if err := bindStrictJSON(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid import request"})
	}
	result, err := h.connections.ImportNotes(c.Request().Context(), userID, accountID, req)
	if err != nil {
		return noteConnectionError(c, err)
	}
	if err := h.notes.SyncUserLinks(c.Request().Context(), userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to sync note links"})
	}
	h.rebuildSearchIndexBestEffort(c.Request().Context())
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) PushNoteConnection(c echo.Context) error {
	userID := c.Get("user_id").(int)
	accountID, err := noteConnectionAccountID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid account ID"})
	}
	var req struct {
		NoteID   string `json:"note_id"`
		TargetID string `json:"target_id"`
	}
	if err := bindStrictJSON(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid push request"})
	}
	noteID, err := uuid.Parse(req.NoteID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid note ID"})
	}
	result, err := h.connections.PushNote(c.Request().Context(), userID, accountID, noteID, req.TargetID)
	if err != nil {
		return noteConnectionError(c, err)
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) ListNoteConnectionJobs(c echo.Context) error {
	userID := c.Get("user_id").(int)
	jobs, err := h.connections.ListJobs(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list note connection jobs"})
	}
	return c.JSON(http.StatusOK, jobs)
}

func noteConnectionAccountID(c echo.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid account id")
	}
	return id, nil
}

func noteConnectionError(c echo.Context, err error) error {
	switch {
	case ent.IsNotFound(err):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Note connection account not found"})
	case errors.Is(err, connectsvc.ErrUnsupportedProvider):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported note provider"})
	case errors.Is(err, connectsvc.ErrMissingCredential):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Provider credential is required"})
	case errors.Is(err, connectsvc.ErrMissingTarget):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Target is required"})
	case errors.Is(err, connectsvc.ErrAccountDisabled):
		return c.JSON(http.StatusConflict, map[string]string{"error": "Note connection account is disabled"})
	default:
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}
