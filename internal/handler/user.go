package handler

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"smarticky/ent"
	"smarticky/ent/user"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

// ListUsers returns all users (admin only)
func (h *Handler) ListUsers(c echo.Context) error {
	users, err := h.client.User.Query().All(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch users"})
	}

	var result []map[string]interface{}
	for _, u := range users {
		result = append(result, userResponse(u, true))
	}

	return c.JSON(http.StatusOK, result)
}

// CreateUser creates a new user (admin only)
func (h *Handler) CreateUser(c echo.Context) error {
	var req struct {
		Username       string `json:"username"`
		Password       string `json:"password"`
		Email          string `json:"email"`
		Nickname       string `json:"nickname"`
		Role           string `json:"role"`
		ShareSignature string `json:"share_signature"`
		TimeZone       string `json:"time_zone"`
		LazycatUID     string `json:"lazycat_uid"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" {
		req.Role = "user"
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	// Generate avatar
	avatarPath, err := h.generateAvatar(req.Username)
	if err != nil {
		// Log error but continue - avatar is optional
		avatarPath = ""
	}

	// Create user
	createUser := h.client.User.
		Create().
		SetUsername(req.Username).
		SetPasswordHash(string(hashedPassword)).
		SetEmail(req.Email).
		SetAvatar(avatarPath).
		SetRole(user.Role(req.Role)).
		SetShareSignature(normalizeShareSignature(req.ShareSignature))

	if req.Nickname != "" {
		createUser.SetNickname(req.Nickname)
	}
	if strings.TrimSpace(req.TimeZone) != "" {
		timeZone, err := normalizeUserTimeZone(req.TimeZone)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid time zone"})
		}
		createUser.SetTimeZone(timeZone)
	}
	if strings.TrimSpace(req.LazycatUID) != "" {
		createUser.SetLazycatUID(strings.TrimSpace(req.LazycatUID))
	}

	newUser, err := createUser.Save(context.Background())

	if err != nil {
		// Clean up avatar file if user creation failed
		if avatarPath != "" {
			// Extract filename from URL path like "/uploads/avatars/file.png"
			filename := filepath.Base(avatarPath)
			filePath := filepath.Join(h.fs.GetUploadsDir("avatars"), filename)
			_ = h.fs.Remove(filePath)
		}
		if ent.IsConstraintError(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Username already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	return c.JSON(http.StatusOK, userResponse(newUser, false))
}

// UpdateUser updates user information
func (h *Handler) UpdateUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	currentUserID := c.Get("user_id").(int)
	currentRole := c.Get("role").(string)

	// Only admin or the user themselves can update
	if currentRole != "admin" && currentUserID != id {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	var req struct {
		Email          *string `json:"email"`
		Nickname       *string `json:"nickname"`
		Avatar         *string `json:"avatar"`
		Role           *string `json:"role"`
		ShareSignature *string `json:"share_signature"`
		TimeZone       *string `json:"time_zone"`
		LazycatUID     *string `json:"lazycat_uid"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Find user
	u, err := h.client.User.Get(context.Background(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	// Update user
	updateQuery := u.Update()

	if req.Email != nil {
		updateQuery = updateQuery.SetEmail(strings.TrimSpace(*req.Email))
	}

	if req.Nickname != nil {
		updateQuery = updateQuery.SetNickname(strings.TrimSpace(*req.Nickname))
	}

	if req.Avatar != nil {
		updateQuery = updateQuery.SetAvatar(strings.TrimSpace(*req.Avatar))
	}

	if req.ShareSignature != nil {
		updateQuery = updateQuery.SetShareSignature(normalizeShareSignature(*req.ShareSignature))
	}

	if req.TimeZone != nil {
		timeZone, err := normalizeUserTimeZone(*req.TimeZone)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid time zone"})
		}
		updateQuery = updateQuery.SetTimeZone(timeZone)
	}

	if req.LazycatUID != nil {
		lazycatUID := strings.TrimSpace(*req.LazycatUID)
		if lazycatUID == "" {
			updateQuery = updateQuery.ClearLazycatUID()
		} else {
			updateQuery = updateQuery.SetLazycatUID(lazycatUID)
		}
	}

	// Only admin can change role
	if req.Role != nil && currentRole == "admin" {
		role := strings.TrimSpace(*req.Role)
		if role == "admin" || role == "user" {
			updateQuery = updateQuery.SetRole(user.Role(role))
		}
	}

	updatedUser, err := updateQuery.Save(context.Background())
	if err != nil {
		if ent.IsConstraintError(err) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "LazyCat user ID already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user"})
	}

	return c.JSON(http.StatusOK, userResponse(updatedUser, false))
}

// UpdatePassword updates user password
func (h *Handler) UpdatePassword(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	currentUserID := c.Get("user_id").(int)

	// Only the user themselves can update their password
	if currentUserID != id {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Find user
	u, err := h.client.User.Get(context.Background(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Incorrect old password"})
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	// Update password
	_, err = u.Update().SetPasswordHash(string(hashedPassword)).Save(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update password"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

// UploadAvatar uploads user avatar
func (h *Handler) UploadAvatar(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	currentUserID := c.Get("user_id").(int)

	// Only the user themselves can upload their avatar
	if currentUserID != id {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Access denied"})
	}

	// Parse multipart form
	file, err := c.FormFile("avatar")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}

	// Validate file type
	if !strings.HasPrefix(file.Header.Get("Content-Type"), "image/") {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "File must be an image"})
	}

	// Get uploads directory
	uploadsDir := h.fs.GetUploadsDir("avatars")

	// Generate filename
	ext := filepath.Ext(file.Filename)
	filename := strconv.Itoa(id) + ext
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

	// Update user avatar path
	avatarURL := h.fs.GetUploadsURL("avatars", filename)
	u, err := h.client.User.Get(context.Background(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	_, err = u.Update().SetAvatar(avatarURL).Save(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user avatar"})
	}

	return c.JSON(http.StatusOK, map[string]string{"avatar": avatarURL})
}

// DeleteUser deletes a user (admin only)
func (h *Handler) DeleteUser(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	currentUserID := c.Get("user_id").(int)

	// Cannot delete yourself
	if currentUserID == id {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cannot delete yourself"})
	}

	// Delete user
	err = h.client.User.DeleteOneID(id).Exec(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User deleted successfully"})
}
