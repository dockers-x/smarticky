package connections

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

func TestSiYuanListTargetsIncludesDocumentTargets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/notebook/lsNotebooks":
			writeSiYuanJSON(t, w, map[string]any{
				"notebooks": []map[string]any{
					{"id": "box-1", "name": "Notebook"},
				},
			})
		case "/api/query/sql":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode SQL request: %v", err)
			}
			if !strings.Contains(payload["stmt"], "SELECT id, content, hpath, box, path FROM blocks") {
				t.Fatalf("unexpected target SQL: %s", payload["stmt"])
			}
			writeSiYuanJSON(t, w, []map[string]any{
				{"id": "doc-a", "content": "A", "hpath": "/A", "box": "box-1", "path": "/a.sy"},
				{"id": "doc-b", "content": "B", "hpath": "/A/B", "box": "box-1", "path": "/a/b.sy"},
			})
		default:
			t.Fatalf("unexpected SiYuan path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider, err := newSiYuanProvider(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("new SiYuan provider: %v", err)
	}

	targets, err := provider.ListTargets(context.Background())
	if err != nil {
		t.Fatalf("ListTargets: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("target count = %d, want notebook plus two documents", len(targets))
	}
	doc := targets[1]
	if doc.Kind != "document" || doc.Name != "Notebook / A" || doc.ParentID != "box-1" {
		t.Fatalf("document target = %#v, want encoded document under notebook", doc)
	}
	decoded := decodeSiYuanImportTargetID(doc.ID)
	if decoded.Box != "box-1" || decoded.HPath != "/A" {
		t.Fatalf("decoded document target = %#v, want box-1 /A", decoded)
	}
}

func TestSiYuanImportNotesFiltersDocumentSubtree(t *testing.T) {
	var importSQL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/query/sql":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode SQL request: %v", err)
			}
			importSQL = payload["stmt"]
			if !strings.Contains(importSQL, "box = 'box-1'") ||
				!strings.Contains(importSQL, "hpath = '/A'") ||
				!strings.Contains(importSQL, "hpath LIKE '/A/%' ESCAPE '\\'") {
				t.Fatalf("import SQL did not filter selected subtree: %s", importSQL)
			}
			writeSiYuanJSON(t, w, []map[string]any{
				{"id": "doc-a", "content": "A", "markdown": "# A", "hpath": "/A", "box": "box-1", "path": "/a.sy", "created": "20260102030405", "updated": "20260102040506"},
				{"id": "doc-b", "content": "B", "markdown": "# B", "hpath": "/A/B", "box": "box-1", "path": "/a/b.sy", "created": "20260103030405", "updated": "20260103040506"},
			})
		case "/api/notebook/lsNotebooks":
			writeSiYuanJSON(t, w, map[string]any{
				"notebooks": []map[string]any{
					{"id": "box-1", "name": "Notebook"},
				},
			})
		case "/api/export/exportMdContent":
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode export request: %v", err)
			}
			writeSiYuanJSON(t, w, map[string]any{
				"hPath":   "/" + strings.TrimPrefix(payload["id"], "doc-"),
				"content": "# " + strings.TrimPrefix(payload["id"], "doc-"),
			})
		case "/api/attr/getBlockAttrs":
			writeSiYuanJSON(t, w, map[string]string{})
		default:
			t.Fatalf("unexpected SiYuan path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	provider, err := newSiYuanProvider(server.URL, "token", server.Client())
	if err != nil {
		t.Fatalf("new SiYuan provider: %v", err)
	}

	notes, err := provider.ImportNotes(context.Background(), encodeSiYuanDocTargetID("box-1", "/A"), 50)
	if err != nil {
		t.Fatalf("ImportNotes: %v", err)
	}
	if importSQL == "" {
		t.Fatal("expected import SQL to be captured")
	}
	if len(notes) != 2 {
		t.Fatalf("note count = %d, want selected document and child", len(notes))
	}
	if notes[0].TargetID != "box-1" || notes[0].TargetName != "Notebook" || notes[0].Path != "/A" {
		t.Fatalf("first note = %#v, want notebook metadata and selected hpath", notes[0])
	}
	if notes[1].Path != "/A/B" {
		t.Fatalf("second note path = %q, want child hpath", notes[1].Path)
	}
}

func writeSiYuanJSON(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{
		"code": 0,
		"msg":  "",
		"data": data,
	}); err != nil {
		t.Fatalf("encode SiYuan response: %v", err)
	}
}
