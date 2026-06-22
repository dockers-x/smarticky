package handler

import (
	"strings"
	"time"

	"smarticky/ent"
)

const (
	defaultShareSignature = "Smarticky"
	maxShareSignatureLen  = 40
	defaultUserTimeZone   = "UTC"
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

func normalizeUserTimeZone(value string) (string, error) {
	timeZone := strings.TrimSpace(value)
	if timeZone == "" {
		return defaultUserTimeZone, nil
	}
	if _, err := time.LoadLocation(timeZone); err != nil {
		return "", err
	}
	return timeZone, nil
}

func userResponse(u *ent.User, includeCreatedAt bool) map[string]interface{} {
	timeZone, err := normalizeUserTimeZone(u.TimeZone)
	if err != nil {
		timeZone = defaultUserTimeZone
	}
	response := map[string]interface{}{
		"id":              u.ID,
		"username":        u.Username,
		"email":           u.Email,
		"nickname":        u.Nickname,
		"role":            u.Role,
		"avatar":          u.Avatar,
		"share_signature": normalizeShareSignature(u.ShareSignature),
		"time_zone":       timeZone,
		"lazycat_uid":     u.LazycatUID,
	}
	if includeCreatedAt {
		response["created_at"] = u.CreatedAt
	}
	return response
}
