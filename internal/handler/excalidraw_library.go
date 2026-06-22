package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/excalidrawlibrary"
	"smarticky/ent/user"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const maxExcalidrawLibrarySize = 20 * 1024 * 1024

type ExcalidrawLibraryResponse struct {
	ID          uuid.UUID `json:"id"`
	LibraryJSON string    `json:"library_json"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UpdateExcalidrawLibraryRequest struct {
	LibraryJSON string `json:"library_json"`
}

func excalidrawLibraryToResponse(library *ent.ExcalidrawLibrary) ExcalidrawLibraryResponse {
	return ExcalidrawLibraryResponse{
		ID:          library.ID,
		LibraryJSON: library.LibraryJSON,
		CreatedAt:   library.CreatedAt,
		UpdatedAt:   library.UpdatedAt,
	}
}

func validateExcalidrawLibraryJSON(libraryJSON string) error {
	trimmed := strings.TrimSpace(libraryJSON)
	if trimmed == "" {
		return whiteboardRequestError{status: http.StatusBadRequest, message: "invalid excalidraw library json"}
	}
	if len(trimmed) > maxExcalidrawLibrarySize {
		return whiteboardRequestError{status: http.StatusRequestEntityTooLarge, message: "excalidraw library is too large"}
	}

	var decoded []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return whiteboardRequestError{status: http.StatusBadRequest, message: "invalid excalidraw library json"}
	}
	return nil
}

func decodeExcalidrawLibraryRequest(c echo.Context, target any) error {
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response().Writer, req.Body, maxExcalidrawLibrarySize+1024*1024)
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return whiteboardRequestError{status: http.StatusRequestEntityTooLarge, message: "excalidraw library is too large"}
		}
		return whiteboardRequestError{status: http.StatusBadRequest, message: "invalid request"}
	}
	return nil
}

func (h *Handler) getOrCreateExcalidrawLibrary(ctx context.Context, userID int) (*ent.ExcalidrawLibrary, error) {
	library, err := h.client.ExcalidrawLibrary.Query().
		Where(excalidrawlibrary.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if err == nil {
		return library, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}

	return h.client.ExcalidrawLibrary.Create().
		SetLibraryJSON("[]").
		SetUserID(userID).
		Save(ctx)
}

func (h *Handler) GetExcalidrawLibrary(c echo.Context) error {
	library, err := h.getOrCreateExcalidrawLibrary(context.Background(), c.Get("user_id").(int))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, excalidrawLibraryToResponse(library))
}

func (h *Handler) UpdateExcalidrawLibrary(c echo.Context) error {
	var req UpdateExcalidrawLibraryRequest
	if err := decodeExcalidrawLibraryRequest(c, &req); err != nil {
		return writeWhiteboardRequestError(c, err)
	}

	libraryJSON := strings.TrimSpace(req.LibraryJSON)
	if err := validateExcalidrawLibraryJSON(libraryJSON); err != nil {
		return writeWhiteboardRequestError(c, err)
	}

	ctx := context.Background()
	library, err := h.getOrCreateExcalidrawLibrary(ctx, c.Get("user_id").(int))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	updated, err := library.Update().
		SetLibraryJSON(libraryJSON).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, excalidrawLibraryToResponse(updated))
}
