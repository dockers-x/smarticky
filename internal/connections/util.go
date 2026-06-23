package connections

import (
	"strings"
	"unicode"
)

func safeRemoteSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "Untitled"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
		case r == '-' || r == '_' || r == ' ':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
		if builder.Len() >= 80 {
			break
		}
	}
	out := strings.Trim(strings.Join(strings.Fields(builder.String()), "-"), "-")
	if out == "" {
		return "Untitled"
	}
	return out
}

func textChunks(value string, maxRunes int) []string {
	if maxRunes <= 0 {
		return []string{value}
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return []string{value}
	}
	chunks := make([]string, 0, len(runes)/maxRunes+1)
	for len(runes) > 0 {
		end := maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[:end]))
		runes = runes[end:]
	}
	return chunks
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
