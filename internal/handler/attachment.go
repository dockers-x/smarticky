package handler

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"smarticky/ent"
	"smarticky/ent/attachment"
	"smarticky/ent/note"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// UploadAttachment uploads an attachment to a note
func (h *Handler) UploadAttachment(c echo.Context) error {
	noteID := c.Param("id")
	userID := c.Get("user_id").(int)

	// Parse note UUID
	noteUUID, err := uuid.Parse(noteID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid note ID"})
	}

	// Check if note exists and belongs to user
	n, err := h.client.Note.Query().
		Where(note.IDEQ(noteUUID)).
		WithUser().
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Note not found"})
	}

	// Check ownership (if note has user, must match current user)
	if n.Edges.User != nil && n.Edges.User.ID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}

	// Get uploads directory
	uploadsDir := h.fs.GetUploadsDir("attachments")

	// Generate unique filename
	ext := filepath.Ext(file.Filename)
	filename := uuid.New().String() + ext
	filePath := filepath.Join(uploadsDir, filename)

	// Open uploaded file
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to open uploaded file"})
	}
	defer src.Close()

	// Save file using filesystem abstraction
	if err := h.fs.SaveUploadedFile(src, filePath); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to save file"})
	}

	// Create attachment record
	att, err := h.client.Attachment.
		Create().
		SetFilename(file.Filename).
		SetFilePath(filePath).
		SetFileSize(file.Size).
		SetMimeType(file.Header.Get("Content-Type")).
		SetNoteID(noteUUID).
		SetUserID(userID).
		Save(context.Background())

	if err != nil {
		// Clean up file if database insert fails
		h.fs.Remove(filePath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create attachment record"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":         att.ID,
		"filename":   att.Filename,
		"file_size":  att.FileSize,
		"mime_type":  att.MimeType,
		"created_at": att.CreatedAt,
	})
}

// ListAttachments lists all attachments for a note
func (h *Handler) ListAttachments(c echo.Context) error {
	noteID := c.Param("id")
	userID := c.Get("user_id").(int)

	// Parse note UUID
	noteUUID, err := uuid.Parse(noteID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid note ID"})
	}

	// Check if note exists and belongs to user
	n, err := h.client.Note.Query().
		Where(note.IDEQ(noteUUID)).
		WithUser().
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Note not found"})
	}

	// Check ownership
	if n.Edges.User != nil && n.Edges.User.ID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	// Get attachments
	attachments, err := h.client.Attachment.Query().
		Where(attachment.HasNoteWith(note.IDEQ(noteUUID))).
		All(context.Background())

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch attachments"})
	}

	var result []map[string]interface{}
	for _, att := range attachments {
		result = append(result, map[string]interface{}{
			"id":         att.ID,
			"filename":   att.Filename,
			"file_size":  att.FileSize,
			"mime_type":  att.MimeType,
			"created_at": att.CreatedAt,
		})
	}

	return c.JSON(http.StatusOK, result)
}

// DownloadAttachment downloads an attachment
func (h *Handler) DownloadAttachment(c echo.Context) error {
	attachmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid attachment ID"})
	}

	userID := c.Get("user_id").(int)

	// Get attachment with note and user
	att, err := h.client.Attachment.Query().
		Where(attachment.IDEQ(attachmentID)).
		WithNote(func(q *ent.NoteQuery) {
			q.WithUser()
		}).
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Attachment not found"})
	}

	// Check ownership
	if att.Edges.Note != nil && att.Edges.Note.Edges.User != nil {
		if att.Edges.Note.Edges.User.ID != userID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
		}
	}

	// Serve file
	return c.File(att.FilePath)
}

// DeleteAttachment deletes an attachment
func (h *Handler) DeleteAttachment(c echo.Context) error {
	attachmentID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid attachment ID"})
	}

	userID := c.Get("user_id").(int)

	// Get attachment with note and user
	att, err := h.client.Attachment.Query().
		Where(attachment.IDEQ(attachmentID)).
		WithNote(func(q *ent.NoteQuery) {
			q.WithUser()
		}).
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Attachment not found"})
	}

	// Check ownership
	if att.Edges.Note != nil && att.Edges.Note.Edges.User != nil {
		if att.Edges.Note.Edges.User.ID != userID {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
		}
	}

	// Delete file from disk
	if err := h.fs.Remove(att.FilePath); err != nil {
		fmt.Printf("Warning: Failed to delete file %s: %v\n", att.FilePath, err)
	}

	// Delete attachment record
	if err := h.client.Attachment.DeleteOneID(attachmentID).Exec(context.Background()); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete attachment"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Attachment deleted successfully"})
}
