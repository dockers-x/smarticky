package handler

import (
	"strings"

	"smarticky/ent"
)

const (
	defaultShareSignature = "Smarticky"
	maxShareSignatureLen  = 40
)

func normalizeShareSignature(value string) string {
	signature := strings.TrimSpace(value)
	if signature == "" {
		return defaultShareSignature
	}

	runes := []rune(signature)
	if len(runes) > maxShareSignatureLen {
		signature = string(runes[:maxShareSignatureLen])
	}
	return signature
}

func userResponse(u *ent.User, includeCreatedAt bool) map[string]interface{} {
	response := map[string]interface{}{
		"id":              u.ID,
		"username":        u.Username,
		"email":           u.Email,
		"nickname":        u.Nickname,
		"role":            u.Role,
		"avatar":          u.Avatar,
		"share_signature": normalizeShareSignature(u.ShareSignature),
		"lazycat_uid":     u.LazycatUID,
	}
	if includeCreatedAt {
		response["created_at"] = u.CreatedAt
	}
	return response
}
