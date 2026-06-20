package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestVerifyNotePasswordRejectsOtherUsersNote(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestVerifyNotePasswordRejectsOtherUsersNote?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	other := client.User.Create().
		SetUsername("other").
		SetPasswordHash("hash").
		SaveX(ctx)

	passwordHash, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	n := client.Note.Create().
		SetTitle("Owner note").
		SetContent("private").
		SetIsLocked(true).
		SetPassword(passwordHash).
		SetUserID(owner.ID).
		SaveX(ctx)

	h := NewHandler(client, nil)

	e := echo.New()
	ownerReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/verify-password", strings.NewReader(`{"password":"secret"}`))
	ownerReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	ownerRec := httptest.NewRecorder()
	ownerCtx := e.NewContext(ownerReq, ownerRec)
	ownerCtx.Set("user_id", owner.ID)
	ownerCtx.SetParamNames("id")
	ownerCtx.SetParamValues(n.ID.String())

	if err := h.VerifyNotePassword(ownerCtx); err != nil {
		t.Fatalf("VerifyNotePassword returned error for owner: %v", err)
	}
	if ownerRec.Code != http.StatusOK {
		t.Fatalf("expected owner status %d, got %d: %s", http.StatusOK, ownerRec.Code, ownerRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/verify-password", strings.NewReader(`{"password":"secret"}`))
	otherReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	otherRec := httptest.NewRecorder()
	otherCtx := e.NewContext(otherReq, otherRec)
	otherCtx.Set("user_id", other.ID)
	otherCtx.SetParamNames("id")
	otherCtx.SetParamValues(n.ID.String())

	if err := h.VerifyNotePassword(otherCtx); err != nil {
		t.Fatalf("VerifyNotePassword returned error for other user: %v", err)
	}
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("expected other user status %d, got %d: %s", http.StatusNotFound, otherRec.Code, otherRec.Body.String())
	}
}

func TestEmptyTrashDeletesOnlyCurrentUsersDeletedNotes(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestEmptyTrashDeletesOnlyCurrentUsersDeletedNotes?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	other := client.User.Create().
		SetUsername("other").
		SetPasswordHash("hash").
		SaveX(ctx)

	client.Note.Create().
		SetTitle("owner active").
		SetUserID(owner.ID).
		SaveX(ctx)
	client.Note.Create().
		SetTitle("owner trash").
		SetIsDeleted(true).
		SetUserID(owner.ID).
		SaveX(ctx)
	client.Note.Create().
		SetTitle("other trash").
		SetIsDeleted(true).
		SetUserID(other.ID).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/notes/trash", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", owner.ID)

	h := NewHandler(client, nil)
	if err := h.EmptyTrash(c); err != nil {
		t.Fatalf("EmptyTrash returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var got struct {
		DeletedCount int `json:"deleted_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.DeletedCount != 1 {
		t.Fatalf("expected one deleted note, got %d", got.DeletedCount)
	}

	remaining := client.Note.Query().CountX(ctx)
	if remaining != 2 {
		t.Fatalf("expected two remaining notes, got %d", remaining)
	}
}

func TestListAttachmentsReturnsEmptyArrayWhenNoAttachments(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestListAttachmentsReturnsEmptyArrayWhenNoAttachments?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	n := client.Note.Create().
		SetTitle("note").
		SetUserID(owner.ID).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/notes/"+n.ID.String()+"/attachments", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", owner.ID)
	c.SetParamNames("id")
	c.SetParamValues(n.ID.String())

	h := NewHandler(client, nil)
	if err := h.ListAttachments(c); err != nil {
		t.Fatalf("ListAttachments returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "[]\n" {
		t.Fatalf("expected empty JSON array, got %q", rec.Body.String())
	}
}
