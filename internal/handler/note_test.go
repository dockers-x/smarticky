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

func TestListNotesFiltersByTag(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestListNotesFiltersByTag?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	taggedNote := client.Note.Create().
		SetTitle("Tagged note").
		SetContent("tagged").
		SetUserID(u.ID).
		SaveX(ctx)
	client.Note.Create().
		SetTitle("Plain note").
		SetContent("plain").
		SetUserID(u.ID).
		SaveX(ctx)

	workTag := client.Tag.Create().
		SetName("work").
		SetUserID(u.ID).
		SaveX(ctx)
	client.Tag.Create().
		SetName("personal").
		SetUserID(u.ID).
		SaveX(ctx)

	taggedNote.Update().AddTags(workTag).SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/notes?tags=work", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)

	h := NewHandler(client, nil)
	if err := h.ListNotes(c); err != nil {
		t.Fatalf("ListNotes returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var got []struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 note, got %d", len(got))
	}
	if got[0].Title != "Tagged note" {
		t.Fatalf("expected Tagged note, got %q", got[0].Title)
	}
}

func TestAddTagToNoteRejectsTagOwnedByAnotherUser(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestAddTagToNoteRejectsTagOwnedByAnotherUser?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	other := client.User.Create().
		SetUsername("other").
		SetPasswordHash("hash").
		SaveX(ctx)

	n := client.Note.Create().
		SetTitle("Owner note").
		SetUserID(owner.ID).
		SaveX(ctx)
	otherTag := client.Tag.Create().
		SetName("other-tag").
		SetUserID(other.ID).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/tags/"+otherTag.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", owner.ID)
	c.SetParamNames("noteId", "tagId")
	c.SetParamValues(n.ID.String(), otherTag.ID.String())

	h := NewHandler(client, nil)
	if err := h.AddTagToNote(c); err != nil {
		t.Fatalf("AddTagToNote returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}

	tags := n.QueryTags().AllX(ctx)
	if len(tags) != 0 {
		t.Fatalf("expected no tags attached, got %d", len(tags))
	}
}
