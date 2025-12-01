package handler

import (
	"context"
	"net/http"
	"strings"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/tag"
	"smarticky/ent/user"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// CreateTag creates a new tag
func (h *Handler) CreateTag(c echo.Context) error {
	ctx := context.Background()
	userID := c.Get("user_id").(int)

	var req struct {
		Name  string `json:"name" validate:"required"`
		Color string `json:"color"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Check if tag already exists for this user
	exists, err := h.client.Tag.Query().
		Where(
			tag.NameEQ(req.Name),
			tag.HasUserWith(user.IDEQ(userID)),
		).
		Exist(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if exists {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Tag already exists"})
	}

	// Create tag
	t, err := h.client.Tag.Create().
		SetName(strings.TrimSpace(req.Name)).
		SetColor(req.Color).
		SetUserID(userID).
		Save(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, t)
}

// GetTags returns all tags for the current user
func (h *Handler) GetTags(c echo.Context) error {
	ctx := context.Background()
	userID := c.Get("user_id").(int)

	tags, err := h.client.Tag.Query().
		Where(tag.HasUserWith(user.IDEQ(userID))).
		Order(ent.Asc(tag.FieldName)).
		All(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, tags)
}

// UpdateTag updates an existing tag
func (h *Handler) UpdateTag(c echo.Context) error {
	ctx := context.Background()
	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid tag ID"})
	}

	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// For now, just check if tag exists (we'll add user validation later)
	t, err := h.client.Tag.Get(ctx, tagID)
	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Update tag
	update := h.client.Tag.UpdateOne(t)

	if req.Name != "" {
		update.SetName(strings.TrimSpace(req.Name))
	}
	if req.Color != "" {
		update.SetColor(req.Color)
	}

	t, err = update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, t)
}

// DeleteTag deletes a tag
func (h *Handler) DeleteTag(c echo.Context) error {
	ctx := context.Background()
	userID := c.Get("user_id").(int)
	tagID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid tag ID"})
	}

	// Check if tag exists and belongs to user
	count, err := h.client.Tag.Delete().
		Where(
			tag.ID(tagID),
			tag.HasUserWith(user.ID(userID)),
		).
		Exec(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if count == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Tag deleted successfully"})
}

// AddTagToNote adds a tag to a note
func (h *Handler) AddTagToNote(c echo.Context) error {
	ctx := context.Background()
	userID := c.Get("user_id").(int)

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid note ID"})
	}

	tagID, err := uuid.Parse(c.Param("tagId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid tag ID"})
	}

	// Check if note exists and belongs to user
	n, err := h.client.Note.Query().
		Where(
			note.ID(noteID),
			note.HasUserWith(user.ID(userID)),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// For now, just check if tag exists (we'll add user validation later)
	t, err := h.client.Tag.Get(ctx, tagID)
	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Add tag to note
	err = n.Update().AddTags(t).Exec(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Tag added to note successfully"})
}

// RemoveTagFromNote removes a tag from a note
func (h *Handler) RemoveTagFromNote(c echo.Context) error {
	ctx := context.Background()
	userID := c.Get("user_id").(int)

	noteID, err := uuid.Parse(c.Param("noteId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid note ID"})
	}

	tagID, err := uuid.Parse(c.Param("tagId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid tag ID"})
	}

	// Check if note exists and belongs to user
	n, err := h.client.Note.Query().
		Where(
			note.ID(noteID),
			note.HasUserWith(user.ID(userID)),
		).
		Only(ctx)

	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// For now, just check if tag exists (we'll add user validation later)
	t, err := h.client.Tag.Get(ctx, tagID)
	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Tag not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Remove tag from note
	err = n.Update().RemoveTags(t).Exec(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Tag removed from note successfully"})
}