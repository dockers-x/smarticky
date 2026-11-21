package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/ent/note"
	"smarticky/internal/notetools"

	"github.com/labstack/echo/v4"
	_ "github.com/lib-x/entsqlite"
)

func TestNoteToolsOnlyReadOwnedUnprotectedNotes(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:TestNoteTools?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	ctx := t.Context()
	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	other := client.User.Create().SetUsername("other").SetPasswordHash("hash").SaveX(ctx)
	registry, err := notetools.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	group := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("user_id", owner.ID); return next(c) }
	})
	NewHandler(client, nil).RegisterNoteTools(group, registry)
	for _, tt := range []struct {
		userID int
		mode   note.ProtectionMode
		want   int
	}{{owner.ID, note.ProtectionModeNone, 200}, {owner.ID, note.ProtectionModePassword, 404}, {owner.ID, note.ProtectionModeEncrypted, 404}, {other.ID, note.ProtectionModeNone, 404}} {
		n := client.Note.Create().SetUserID(tt.userID).SetContent("- [ ] write").SetProtectionMode(tt.mode).SaveX(ctx)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/tools/open-tasks", nil))
		if rec.Code != tt.want {
			t.Fatalf("user %d mode %s: %d %s", tt.userID, tt.mode, rec.Code, rec.Body.String())
		}
		if tt.want == 200 {
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["text"] != "- [ ] write" {
				t.Fatalf("unexpected tool result %s", rec.Body.String())
			}
		}
	}
}
