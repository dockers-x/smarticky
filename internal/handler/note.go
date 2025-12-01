package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/argon2"
)

// Argon2 parameters
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	saltLen       = 16
)

// hashPassword generates an argon2id hash of the password
func hashPassword(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash the password
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Encode to base64 for storage: $argon2id$salt$hash
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$%s$%s", encodedSalt, encodedHash), nil
}

// verifyPassword checks if the provided password matches the stored hash
func verifyPassword(password, storedHash string) (bool, error) {
	// Parse the stored hash: $argon2id$salt$hash
	parts := strings.Split(storedHash, "$")
	if len(parts) != 4 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}

	// Hash the provided password with the same salt
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Compare using constant-time comparison
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}

type NoteResponse struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Color     string    `json:"color"`
	IsLocked  bool      `json:"is_locked"`
	IsStarred bool      `json:"is_starred"`
	IsDeleted bool      `json:"is_deleted"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func noteToResponse(n *ent.Note) NoteResponse {
	return NoteResponse{
		ID:        n.ID,
		Title:     n.Title,
		Content:   n.Content,
		Color:     n.Color,
		IsLocked:  n.IsLocked,
		IsStarred: n.IsStarred,
		IsDeleted: n.IsDeleted,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}

type CreateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	Color   string `json:"color"`
}

type UpdateNoteRequest struct {
	Title     *string `json:"title"`
	Content   *string `json:"content"`
	Color     *string `json:"color"`
	Password  *string `json:"password"`
	IsLocked  *bool   `json:"is_locked"`
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

	// Color filtering
	if color := c.QueryParam("color"); color != "" {
		query.Where(note.ColorEQ(color))
	}

	// Tag filtering
	if tags := c.QueryParam("tags"); tags != "" {
		// For now, we'll implement a basic tag name filtering
		// This will be enhanced once the tag system is fully implemented
		tagNames := strings.Split(tags, ",")
		for _, tagName := range tagNames {
			tagName = strings.TrimSpace(tagName)
			if tagName != "" {
				// Placeholder for tag filtering - will be implemented after tag system is set up
				// query.Where(note.HasTagsWith(tag.NameEQ(tagName)))
			}
		}
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

	// Convert to response format that includes tags
	type NoteWithTagsResponse struct {
		NoteResponse
		Tags []*ent.Tag `json:"tags"`
	}

	response := make([]NoteWithTagsResponse, len(notes))
	for i, n := range notes {
		// Get tags for this note
		tags, err := n.QueryTags().All(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		response[i] = NoteWithTagsResponse{
			NoteResponse: noteToResponse(n),
			Tags:         tags,
		}
	}

	return c.JSON(http.StatusOK, response)
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
		SetColor(req.Color).
		Save(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, noteToResponse(n))
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

	// Get tags for this note
	tags, err := n.QueryTags().All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Convert to response format that includes tags
	type NoteWithTagsResponse struct {
		NoteResponse
		Tags []*ent.Tag `json:"tags"`
	}

	response := NoteWithTagsResponse{
		NoteResponse: noteToResponse(n),
		Tags:         tags,
	}

	return c.JSON(http.StatusOK, response)
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
	if req.Color != nil {
		update.SetColor(*req.Color)
	}
	if req.Password != nil {
		// Hash the password before storing
		if *req.Password == "" {
			// Empty password means remove password protection
			update.SetPassword("")
		} else {
			hashedPassword, err := hashPassword(*req.Password)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
			}
			update.SetPassword(hashedPassword)
		}
	}
	if req.IsLocked != nil {
		update.SetIsLocked(*req.IsLocked)
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

	return c.JSON(http.StatusOK, noteToResponse(n))
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

// VerifyNotePassword verifies if the provided password matches the note's password
func (h *Handler) VerifyNotePassword(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	type VerifyPasswordRequest struct {
		Password string `json:"password"`
	}

	var req VerifyPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	n, err := h.client.Note.Get(ctx, id)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Check if note is locked
	if !n.IsLocked || n.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "note is not password protected"})
	}

	// Verify password
	valid, err := verifyPassword(req.Password, n.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to verify password"})
	}

	if !valid {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "incorrect password"})
	}

	// Return success with note content
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"note":    noteToResponse(n),
	})
}
