package connections

import (
	"strings"
	"testing"
	"time"
)

func TestParseSiYuanFrontMatterRemovesYAMLMetadata(t *testing.T) {
	meta := parseSiYuanFrontMatter("---\ntitle: Imported title\ndate: 2025-08-28T17:28:17+08:00\ntags: [alpha, beta]\n---\n# Heading\nBody")

	if meta.Title != "Imported title" {
		t.Fatalf("title = %q, want Imported title", meta.Title)
	}
	if meta.CreatedAt == nil || meta.CreatedAt.Format(time.RFC3339) != "2025-08-28T09:28:17Z" {
		t.Fatalf("created_at = %v, want 2025-08-28T09:28:17Z", meta.CreatedAt)
	}
	if strings.Contains(meta.Content, "title:") || !strings.HasPrefix(meta.Content, "# Heading") {
		t.Fatalf("content = %q, want front matter removed", meta.Content)
	}
	if got := strings.Join(meta.Tags, ","); got != "alpha,beta" {
		t.Fatalf("tags = %q, want alpha,beta", got)
	}
}

func TestParseSiYuanFrontMatterRemovesLooseSingleLineMetadata(t *testing.T) {
	content := "*** title: Claude Code提示词 by 向阳乔木 date: 2025-08-28T17:28:17+08:00 lastmod: 2025-08-28T17:29:00+08:00 ---\n\n# 正文\n内容"

	meta := parseSiYuanFrontMatter(content)

	if meta.Title != "Claude Code提示词 by 向阳乔木" {
		t.Fatalf("title = %q, want parsed title", meta.Title)
	}
	if meta.CreatedAt == nil || meta.CreatedAt.Format(time.RFC3339) != "2025-08-28T09:28:17Z" {
		t.Fatalf("created_at = %v, want 2025-08-28T09:28:17Z", meta.CreatedAt)
	}
	if meta.UpdatedAt == nil || meta.UpdatedAt.Format(time.RFC3339) != "2025-08-28T09:29:00Z" {
		t.Fatalf("updated_at = %v, want 2025-08-28T09:29:00Z", meta.UpdatedAt)
	}
	if strings.Contains(meta.Content, "title:") || strings.Contains(meta.Content, "lastmod:") {
		t.Fatalf("content = %q, want loose metadata removed", meta.Content)
	}
	if !strings.HasPrefix(meta.Content, "# 正文") {
		t.Fatalf("content = %q, want body to start with heading", meta.Content)
	}
}
