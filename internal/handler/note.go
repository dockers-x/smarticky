package handler

import (
	"context"
	"net/http"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type CreateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Format  string `json:"format"`
}

type UpdateNoteRequest struct {
	Title     *string `json:"title"`
	Content   *string `json:"content"`
	Format    *string `json:"format"`
	IsStarred *bool   `json:"is_starred"`
	IsDeleted *bool   `json:"is_deleted"`
}

func (h *Handler) ListNotes(c echo.Context) error {
	ctx := context.Background()
	query := h.client.Note.Query()

	// Filters
	if c.QueryParam("starred") == "true" {
		query.Where(note.IsStarred(true))
	}
	if c.QueryParam("trash") == "true" {
		query.Where(note.IsDeleted(true))
	} else {
		// Default: not deleted
		query.Where(note.IsDeleted(false))
	}

	search := c.QueryParam("q")
	if search != "" {
		query.Where(
			note.Or(
				note.TitleContainsFold(search),
				note.ContentContainsFold(search),
			),
		)
	}

	// Order by updated_at desc
	query.Order(ent.Desc(note.FieldUpdatedAt))

	notes, err := query.All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, notes)
}

func (h *Handler) CreateNote(c echo.Context) error {
	var req CreateNoteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	n, err := h.client.Note.Create().
		SetTitle(req.Title).
		SetContent(req.Content).
		SetFormat(req.Format).
		Save(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, n)
}

func (h *Handler) GetNote(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	ctx := context.Background()
	n, err := h.client.Note.Get(ctx, id)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, n)
}

func (h *Handler) UpdateNote(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	var req UpdateNoteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	update := h.client.Note.UpdateOneID(id).SetUpdatedAt(time.Now())

	if req.Title != nil {
		update.SetTitle(*req.Title)
	}
	if req.Content != nil {
		update.SetContent(*req.Content)
	}
	if req.Format != nil {
		update.SetFormat(*req.Format)
	}
	if req.IsStarred != nil {
		update.SetIsStarred(*req.IsStarred)
	}
	if req.IsDeleted != nil {
		update.SetIsDeleted(*req.IsDeleted)
	}

	n, err := update.Save(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, n)
}

func (h *Handler) DeleteNote(c echo.Context) error {
	// Permanent delete
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	ctx := context.Background()
	err = h.client.Note.DeleteOneID(id).Exec(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}
