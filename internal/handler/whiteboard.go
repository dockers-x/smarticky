package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/user"
	"smarticky/ent/whiteboard"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const maxWhiteboardSceneSize = 20 * 1024 * 1024

type whiteboardRequestError struct {
	status  int
	message string
}

func (e whiteboardRequestError) Error() string {
	return e.message
}

type WhiteboardResponse struct {
	ID        uuid.UUID `json:"id"`
	NoteID    uuid.UUID `json:"note_id"`
	Title     string    `json:"title"`
	SceneJSON string    `json:"scene_json"`
	Thumbnail string    `json:"thumbnail"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateWhiteboardRequest struct {
	Title     string `json:"title"`
	SceneJSON string `json:"scene_json"`
	Thumbnail string `json:"thumbnail"`
}

type UpdateWhiteboardRequest struct {
	Title     *string `json:"title"`
	SceneJSON *string `json:"scene_json"`
	Thumbnail *string `json:"thumbnail"`
}

func whiteboardToResponse(w *ent.Whiteboard) WhiteboardResponse {
	response := WhiteboardResponse{
		ID:        w.ID,
		Title:     w.Title,
		SceneJSON: w.SceneJSON,
		Thumbnail: w.Thumbnail,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
	if w.Edges.Note != nil {
		response.NoteID = w.Edges.Note.ID
	}
	return response
}

func parseWhiteboardID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

func validateSceneJSON(sceneJSON string) error {
	trimmed := strings.TrimSpace(sceneJSON)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > maxWhiteboardSceneSize {
		return whiteboardRequestError{status: http.StatusRequestEntityTooLarge, message: "whiteboard scene is too large"}
	}
	if !json.Valid([]byte(trimmed)) {
		return whiteboardRequestError{status: http.StatusBadRequest, message: "invalid whiteboard scene json"}
	}
	return nil
}

func decodeWhiteboardRequest(c echo.Context, target any) error {
	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response().Writer, req.Body, maxWhiteboardSceneSize+1024*1024)
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(target); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return whiteboardRequestError{status: http.StatusRequestEntityTooLarge, message: "whiteboard scene is too large"}
		}
		return whiteboardRequestError{status: http.StatusBadRequest, message: "invalid request"}
	}
	return nil
}

func writeWhiteboardRequestError(c echo.Context, err error) error {
	if reqErr, ok := err.(whiteboardRequestError); ok {
		return c.JSON(reqErr.status, map[string]string{"error": reqErr.message})
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
}

func (h *Handler) ownedNote(ctx context.Context, noteID uuid.UUID, userID int) (*ent.Note, error) {
	return h.client.Note.Query().
		Where(
			note.ID(noteID),
			note.HasUserWith(user.IDEQ(userID)),
		).
		Only(ctx)
}

func (h *Handler) ownedWhiteboard(ctx context.Context, whiteboardID uuid.UUID, userID int) (*ent.Whiteboard, error) {
	return h.client.Whiteboard.Query().
		Where(
			whiteboard.IDEQ(whiteboardID),
			whiteboard.HasUserWith(user.IDEQ(userID)),
		).
		WithNote().
		Only(ctx)
}

func (h *Handler) CreateWhiteboard(c echo.Context) error {
	noteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid note id"})
	}

	var req CreateWhiteboardRequest
	if err := decodeWhiteboardRequest(c, &req); err != nil {
		return writeWhiteboardRequestError(c, err)
	}
	if err := validateSceneJSON(req.SceneJSON); err != nil {
		return writeWhiteboardRequestError(c, err)
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	if _, err := h.ownedNote(ctx, noteID, userID); ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Whiteboard"
	}
	sceneJSON := strings.TrimSpace(req.SceneJSON)
	if sceneJSON == "" {
		sceneJSON = "{}"
	}

	create := h.client.Whiteboard.Create().
		SetTitle(title).
		SetSceneJSON(sceneJSON).
		SetNoteID(noteID).
		SetUserID(userID)
	if strings.TrimSpace(req.Thumbnail) != "" {
		create.SetThumbnail(req.Thumbnail)
	}

	w, err := create.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	w.Edges.Note = &ent.Note{ID: noteID}
	return c.JSON(http.StatusCreated, whiteboardToResponse(w))
}

func (h *Handler) ListWhiteboards(c echo.Context) error {
	noteID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid note id"})
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	if _, err := h.ownedNote(ctx, noteID, userID); ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	whiteboards, err := h.client.Whiteboard.Query().
		Where(
			whiteboard.HasNoteWith(note.IDEQ(noteID)),
			whiteboard.HasUserWith(user.IDEQ(userID)),
		).
		WithNote().
		Order(ent.Desc(whiteboard.FieldUpdatedAt)).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response := make([]WhiteboardResponse, 0, len(whiteboards))
	for _, w := range whiteboards {
		response = append(response, whiteboardToResponse(w))
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) GetWhiteboard(c echo.Context) error {
	whiteboardID, err := parseWhiteboardID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid whiteboard id"})
	}

	w, err := h.ownedWhiteboard(context.Background(), whiteboardID, c.Get("user_id").(int))
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "whiteboard not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, whiteboardToResponse(w))
}

func (h *Handler) UpdateWhiteboard(c echo.Context) error {
	whiteboardID, err := parseWhiteboardID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid whiteboard id"})
	}

	var req UpdateWhiteboardRequest
	if err := decodeWhiteboardRequest(c, &req); err != nil {
		return writeWhiteboardRequestError(c, err)
	}
	if req.SceneJSON != nil {
		if err := validateSceneJSON(*req.SceneJSON); err != nil {
			return writeWhiteboardRequestError(c, err)
		}
	}

	ctx := context.Background()
	w, err := h.ownedWhiteboard(ctx, whiteboardID, c.Get("user_id").(int))
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "whiteboard not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	update := w.Update().SetUpdatedAt(time.Now())
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			title = "Whiteboard"
		}
		update.SetTitle(title)
	}
	if req.SceneJSON != nil {
		sceneJSON := strings.TrimSpace(*req.SceneJSON)
		if sceneJSON == "" {
			sceneJSON = "{}"
		}
		update.SetSceneJSON(sceneJSON)
	}
	if req.Thumbnail != nil {
		update.SetThumbnail(*req.Thumbnail)
	}

	updated, err := update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	updated.Edges.Note = w.Edges.Note

	return c.JSON(http.StatusOK, whiteboardToResponse(updated))
}

func (h *Handler) DeleteWhiteboard(c echo.Context) error {
	whiteboardID, err := parseWhiteboardID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid whiteboard id"})
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	count, err := h.client.Whiteboard.Delete().
		Where(
			whiteboard.IDEQ(whiteboardID),
			whiteboard.HasUserWith(user.IDEQ(userID)),
		).
		Exec(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if count == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "whiteboard not found"})
	}

	return c.NoContent(http.StatusNoContent)
}
