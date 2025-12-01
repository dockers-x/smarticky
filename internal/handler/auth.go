package handler

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"smarticky/ent/user"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/o1egl/govatar"
	"golang.org/x/crypto/bcrypt"
)

const (
	jwtSecret = "smarticky-secret-key-change-in-production" // TODO: Move to config
	jwtExpiry = 24 * time.Hour
)

type JWTClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// generateAvatar generates a random avatar for a user
func (h *Handler) generateAvatar(username string) (string, error) {
	// Get uploads directory from filesystem
	avatarDir := h.fs.GetUploadsDir("avatars")

	// Generate unique filename
	filename := fmt.Sprintf("%s_%d.png", username, time.Now().UnixNano())
	filePath := filepath.Join(avatarDir, filename)

	// Generate avatar using govatar
	gender := govatar.MALE
	if len(username)%2 == 0 {
		gender = govatar.FEMALE
	}

	err := govatar.GenerateFileForUsername(gender, username, filePath)
	if err != nil {
		return "", fmt.Errorf("failed to generate avatar: %w", err)
	}

	// Verify the file was created and is readable
	if _, err := h.fs.Stat(filePath); err != nil {
		return "", fmt.Errorf("avatar file verification failed: %w", err)
	}

	// Return URL path for web access
	return h.fs.GetUploadsURL("avatars", filename), nil
}

// Setup checks if admin exists, if not creates first admin
func (h *Handler) Setup(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
		Nickname string `json:"nickname"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Check if any user exists
	count, err := h.client.User.Query().Count(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}

	if count > 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Setup already completed"})
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

	// Create admin user
	createUser := h.client.User.
		Create().
		SetUsername(req.Username).
		SetPasswordHash(string(hashedPassword)).
		SetEmail(req.Email).
		SetAvatar(avatarPath).
		SetRole(user.RoleAdmin)

	if req.Nickname != "" {
		createUser.SetNickname(req.Nickname)
	}

	newUser, err := createUser.Save(context.Background())

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create admin user"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Admin user created successfully",
		"user": map[string]interface{}{
			"id":       newUser.ID,
			"username": newUser.Username,
			"email":    newUser.Email,
			"nickname": newUser.Nickname,
			"role":     newUser.Role,
			"avatar":   newUser.Avatar,
		},
	})
}

// CheckSetup checks if setup is needed
func (h *Handler) CheckSetup(c echo.Context) error {
	count, err := h.client.User.Query().Count(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"setup_needed": count == 0,
	})
}

// Login authenticates a user
func (h *Handler) Login(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Find user
	u, err := h.client.User.Query().
		Where(user.UsernameEQ(req.Username)).
		Only(context.Background())

	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
	}

	// Create JWT token
	claims := &JWTClaims{
		UserID:   u.ID,
		Username: u.Username,
		Role:     string(u.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(jwtExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": tokenString,
		"user": map[string]interface{}{
			"id":       u.ID,
			"username": u.Username,
			"email":    u.Email,
			"nickname": u.Nickname,
			"role":     u.Role,
			"avatar":   u.Avatar,
		},
	})
}

// GetCurrentUser returns the currently authenticated user
func (h *Handler) GetCurrentUser(c echo.Context) error {
	userID := c.Get("user_id").(int)

	u, err := h.client.User.Get(context.Background(), userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":       u.ID,
		"username": u.Username,
		"email":    u.Email,
		"nickname": u.Nickname,
		"role":     u.Role,
		"avatar":   u.Avatar,
	})
}

// Logout invalidates the current session
func (h *Handler) Logout(c echo.Context) error {
	// With JWT, logout is handled client-side by removing the token
	return c.JSON(http.StatusOK, map[string]string{"message": "Logged out successfully"})
}
