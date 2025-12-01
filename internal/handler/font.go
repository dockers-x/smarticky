package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"smarticky/ent"
	"smarticky/ent/font"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	maxFontFileSize = 30 * 1024 * 1024 // 30MB
)

var allowedFontFormats = map[string]bool{
	".ttf":   true,
	".otf":   true,
	".woff":  true,
	".woff2": true,
}

// FontResponse represents a font with uploader info
type FontResponse struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	DisplayName  string    `json:"display_name"`
	Format       string    `json:"format"`
	FileSize     int64     `json:"file_size"`
	PreviewText  string    `json:"preview_text"`
	IsShared     bool      `json:"is_shared"`
	UploadedBy   string    `json:"uploaded_by"`
	UploaderID   int       `json:"uploader_id"`
	DownloadURL  string    `json:"download_url"`
	CreatedAt    string    `json:"created_at"`
}

// UploadFont uploads a font file
func (h *Handler) UploadFont(c echo.Context) error {
	userID := c.Get("user_id").(int)

	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}

	// Check file size
	if file.Size > maxFontFileSize {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("File size exceeds maximum limit of %d MB", maxFontFileSize/(1024*1024)),
		})
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedFontFormats[ext] {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid font format. Allowed formats: .ttf, .otf, .woff, .woff2",
		})
	}

	// Get font name and display name from form
	fontName := c.FormValue("name")
	displayName := c.FormValue("display_name")
	previewText := c.FormValue("preview_text")
	isSharedStr := c.FormValue("is_shared")

	if fontName == "" {
		// Use filename without extension as default name
		fontName = strings.TrimSuffix(file.Filename, ext)
	}
	if displayName == "" {
		displayName = fontName
	}
	if previewText == "" {
		previewText = "The quick brown fox jumps over the lazy dog 我能吞下玻璃而不伤身体"
	}

	// Parse is_shared (default to true)
	isShared := true
	if isSharedStr == "false" || isSharedStr == "0" {
		isShared = false
	}

	// Get uploads directory
	uploadsDir := h.fs.GetUploadsDir("fonts")

	// Generate unique filename
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

	// Determine format enum value
	formatValue := strings.TrimPrefix(ext, ".")

	// Create font record
	fontEntity, err := h.client.Font.
		Create().
		SetName(fontName).
		SetDisplayName(displayName).
		SetFilePath(filePath).
		SetFileSize(file.Size).
		SetFormat(font.Format(formatValue)).
		SetPreviewText(previewText).
		SetIsShared(isShared).
		SetUploadedByID(userID).
		Save(context.Background())

	if err != nil {
		// Clean up file if database insert fails
		h.fs.Remove(filePath)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create font record"})
	}

	// Get uploader info
	user, err := h.client.User.Get(context.Background(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user info"})
	}

	return c.JSON(http.StatusOK, FontResponse{
		ID:          fontEntity.ID,
		Name:        fontEntity.Name,
		DisplayName: fontEntity.DisplayName,
		Format:      string(fontEntity.Format),
		FileSize:    fontEntity.FileSize,
		PreviewText: fontEntity.PreviewText,
		IsShared:    fontEntity.IsShared,
		UploadedBy:  user.Username,
		UploaderID:  user.ID,
		DownloadURL: fmt.Sprintf("/api/fonts/%s/download", fontEntity.ID),
		CreatedAt:   fontEntity.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// GetFonts returns all uploaded fonts (shared fonts + user's own fonts)
func (h *Handler) GetFonts(c echo.Context) error {
	userID := c.Get("user_id").(int)

	fonts, err := h.client.Font.
		Query().
		WithUploadedBy().
		Order(ent.Desc(font.FieldCreatedAt)).
		All(context.Background())

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get fonts"})
	}

	response := make([]FontResponse, 0, len(fonts))
	for _, f := range fonts {
		// Only include fonts that are either shared or owned by current user
		if !f.IsShared && f.Edges.UploadedBy.ID != userID {
			continue
		}

		uploaderName := "Unknown"
		uploaderID := 0
		if f.Edges.UploadedBy != nil {
			uploaderName = f.Edges.UploadedBy.Username
			uploaderID = f.Edges.UploadedBy.ID
		}

		response = append(response, FontResponse{
			ID:          f.ID,
			Name:        f.Name,
			DisplayName: f.DisplayName,
			Format:      string(f.Format),
			FileSize:    f.FileSize,
			PreviewText: f.PreviewText,
			IsShared:    f.IsShared,
			UploadedBy:  uploaderName,
			UploaderID:  uploaderID,
			DownloadURL: fmt.Sprintf("/api/fonts/%s/download", f.ID),
			CreatedAt:   f.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	return c.JSON(http.StatusOK, response)
}

// DownloadFont serves a font file
func (h *Handler) DownloadFont(c echo.Context) error {
	fontID := c.Param("id")
	userID := c.Get("user_id").(int)

	// Parse font UUID
	fontUUID, err := uuid.Parse(fontID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid font ID"})
	}

	// Get font record with uploader info
	fontEntity, err := h.client.Font.
		Query().
		Where(font.IDEQ(fontUUID)).
		WithUploadedBy().
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Font not found"})
	}

	// Check permission: only shared fonts or user's own fonts can be downloaded
	if !fontEntity.IsShared && fontEntity.Edges.UploadedBy.ID != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	// Open file
	file, err := h.fs.Open(fontEntity.FilePath)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to open font file"})
	}
	defer file.Close()

	// Set appropriate MIME type
	mimeType := getMimeTypeForFont(fontEntity.Format)
	c.Response().Header().Set("Content-Type", mimeType)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000") // Cache for 1 year

	// Set CORS headers to allow font loading from any origin
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")

	// Stream file
	_, err = io.Copy(c.Response().Writer, file)
	return err
}

// DeleteFont deletes a font (only uploader or admin)
func (h *Handler) DeleteFont(c echo.Context) error {
	fontID := c.Param("id")
	userID := c.Get("user_id").(int)
	userRole := c.Get("user_role").(string)

	// Parse font UUID
	fontUUID, err := uuid.Parse(fontID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid font ID"})
	}

	// Get font record
	fontEntity, err := h.client.Font.
		Query().
		Where(font.IDEQ(fontUUID)).
		WithUploadedBy().
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Font not found"})
	}

	// Check permission: only uploader or admin can delete
	if fontEntity.Edges.UploadedBy.ID != userID && userRole != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	// Delete file
	if err := h.fs.Remove(fontEntity.FilePath); err != nil {
		// Log error but continue with database deletion
		fmt.Printf("Failed to delete font file %s: %v\n", fontEntity.FilePath, err)
	}

	// Delete database record
	if err := h.client.Font.DeleteOne(fontEntity).Exec(context.Background()); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete font"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Font deleted successfully"})
}

// getMimeTypeForFont returns the appropriate MIME type for a font format
func getMimeTypeForFont(format font.Format) string {
	switch format {
	case font.FormatTtf:
		return "font/ttf"
	case font.FormatOtf:
		return "font/otf"
	case font.FormatWoff:
		return "font/woff"
	case font.FormatWoff2:
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}
