package handler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"smarticky/ent/user"

	"golang.org/x/crypto/bcrypt"
)

const (
	envAdminUsername = "SMARTICKY_ADMIN_USERNAME"
	envAdminPassword = "SMARTICKY_ADMIN_PASSWORD"
	envAdminEmail    = "SMARTICKY_ADMIN_EMAIL"
	envAdminNickname = "SMARTICKY_ADMIN_NICKNAME"
)

// InitializeAdminFromEnv creates the first admin from environment variables.
// It only runs against an empty user table; once any user exists, env values are ignored.
func (h *Handler) InitializeAdminFromEnv(ctx context.Context, getenv func(string) string) (bool, error) {
	count, err := h.client.User.Query().Count(ctx)
	if err != nil {
		return false, fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	username := strings.TrimSpace(getenv(envAdminUsername))
	password := getenv(envAdminPassword)
	email := strings.TrimSpace(getenv(envAdminEmail))
	nickname := strings.TrimSpace(getenv(envAdminNickname))

	if username == "" && password == "" && email == "" && nickname == "" {
		return false, nil
	}
	if username == "" || password == "" {
		return false, fmt.Errorf("%s and %s must both be set to initialize the first admin", envAdminUsername, envAdminPassword)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return false, fmt.Errorf("hash admin password: %w", err)
	}

	avatarPath := ""
	if h.fs != nil {
		avatarPath, err = h.generateAvatar(username)
		if err != nil {
			avatarPath = ""
		}
	}

	createUser := h.client.User.
		Create().
		SetUsername(username).
		SetPasswordHash(string(hashedPassword)).
		SetRole(user.RoleAdmin).
		SetAvatar(avatarPath)

	if email != "" {
		createUser.SetEmail(email)
	}
	if nickname != "" {
		createUser.SetNickname(nickname)
	}

	if _, err := createUser.Save(ctx); err != nil {
		if avatarPath != "" && h.fs != nil {
			filename := filepath.Base(avatarPath)
			filePath := filepath.Join(h.fs.GetUploadsDir("avatars"), filename)
			_ = h.fs.Remove(filePath)
		}
		return false, fmt.Errorf("create env admin: %w", err)
	}

	return true, nil
}
