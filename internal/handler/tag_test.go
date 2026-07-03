package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"smarticky/ent/enttest"

	"github.com/labstack/echo/v4"
	_ "github.com/lib-x/entsqlite"
)

func TestGetTagsIncludesActiveNoteCount(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestGetTagsIncludesActiveNoteCount?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	activeTag := client.Tag.Create().
		SetName("Work").
		SetColor("#E8450A").
		SetUserID(u.ID).
		SaveX(ctx)
	deletedOnlyTag := client.Tag.Create().
		SetName("Archive").
		SetColor("#888888").
		SetUserID(u.ID).
		SaveX(ctx)
	activeNote := client.Note.Create().
		SetTitle("Active").
		SetUserID(u.ID).
		SaveX(ctx)
	deletedNote := client.Note.Create().
		SetTitle("Deleted").
		SetIsDeleted(true).
		SetUserID(u.ID).
		SaveX(ctx)
	activeNote.Update().AddTags(activeTag).SaveX(ctx)
	deletedNote.Update().AddTags(activeTag, deletedOnlyTag).SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)

	if err := NewHandler(client, nil).GetTags(c); err != nil {
		t.Fatalf("GetTags returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var got []struct {
		Name      string `json:"name"`
		NoteCount int    `json:"note_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	counts := map[string]int{}
	for _, item := range got {
		counts[item.Name] = item.NoteCount
	}
	if counts["Work"] != 1 {
		t.Fatalf("expected Work note_count 1, got %d", counts["Work"])
	}
	if counts["Archive"] != 0 {
		t.Fatalf("expected Archive note_count 0, got %d", counts["Archive"])
	}
}
