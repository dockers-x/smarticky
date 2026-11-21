package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/ent/note"
	"smarticky/ent/notelink"
	"smarticky/internal/notes"

	"github.com/labstack/echo/v4"
	_ "github.com/lib-x/entsqlite"
)

func TestCreateNoteSyncsOutgoingLinks(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestCreateNoteSyncsOutgoingLinks?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	target := client.Note.Create().SetTitle("Target").SetUserID(owner.ID).SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"title":"Source","content":"[[Target]]"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", owner.ID)

	if err := NewHandler(client, nil).CreateNote(c); err != nil {
		t.Fatalf("CreateNote returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created NoteResponse
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if count := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(created.ID), notelink.TargetNoteIDEQ(target.ID)).CountX(ctx); count != 1 {
		t.Fatalf("expected CreateNote to sync one outgoing link, got %d", count)
	}
}

func TestUpdateNoteToEncryptedClearsOutgoingLinks(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateNoteToEncryptedClearsOutgoingLinks?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	target := client.Note.Create().SetTitle("Target").SetUserID(owner.ID).SaveX(ctx)
	source := client.Note.Create().SetTitle("Source").SetContent("[[Target]]").SetUserID(owner.ID).SaveX(ctx)
	if err := notes.NewService(client).SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync links: %v", err)
	}
	if count := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(source.ID), notelink.TargetNoteIDEQ(target.ID)).CountX(ctx); count != 1 {
		t.Fatalf("expected seeded outgoing link, got %d", count)
	}

	body := `{"protection_mode":"encrypted","encrypted_content":"ciphertext","encryption_alg":"aes-gcm","encryption_kdf":"argon2id","encryption_salt":"salt","encryption_nonce":"nonce"}`
	req := httptest.NewRequest(http.MethodPut, "/api/notes/"+source.ID.String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", owner.ID)
	c.SetParamNames("id")
	c.SetParamValues(source.ID.String())

	if err := NewHandler(client, nil).UpdateNote(c); err != nil {
		t.Fatalf("UpdateNote returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if count := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(source.ID)).CountX(ctx); count != 0 {
		t.Fatalf("expected encrypted update to clear outgoing links, got %d", count)
	}
}

func TestGetNoteLinksReturnsMetadataOnlyForProtectedTargets(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestGetNoteLinksReturnsMetadataOnlyForProtectedTargets?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	hash, err := hashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	target := client.Note.Create().
		SetTitle("Secret Target").
		SetContent("private body").
		SetProtectionMode(note.ProtectionModePassword).
		SetProtectionPasswordHash(hash).
		SetUserID(owner.ID).
		SaveX(ctx)
	source := client.Note.Create().SetTitle("Source").SetContent("[[Secret Target]]").SetUserID(owner.ID).SaveX(ctx)
	if err := notes.NewService(client).SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync links: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/notes/"+source.ID.String()+"/links", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", owner.ID)
	c.SetParamNames("id")
	c.SetParamValues(source.ID.String())

	if err := NewHandler(client, nil).GetNoteLinks(c); err != nil {
		t.Fatalf("GetNoteLinks returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string][]map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	targetMeta := body["outgoing"][0]["target_note"].(map[string]any)
	if targetMeta["id"] != target.ID.String() || targetMeta["title"] != "Secret Target" || targetMeta["protection_mode"] != "password" || targetMeta["content_redacted"] != true {
		t.Fatalf("unexpected target metadata: %+v", targetMeta)
	}
	if _, ok := targetMeta["content"]; ok {
		t.Fatalf("target metadata must not include content: %+v", targetMeta)
	}
}

func TestGetNoteLinksEnforcesUserIsolation(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestGetNoteLinksEnforcesUserIsolation?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	other := client.User.Create().SetUsername("other").SetPasswordHash("hash").SaveX(ctx)
	source := client.Note.Create().SetTitle("Source").SetContent("[[Target]]").SetUserID(owner.ID).SaveX(ctx)
	client.Note.Create().SetTitle("Target").SetUserID(other.ID).SaveX(ctx)
	if err := notes.NewService(client).SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync links: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/notes/"+source.ID.String()+"/links", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", other.ID)
	c.SetParamNames("id")
	c.SetParamValues(source.ID.String())

	if err := NewHandler(client, nil).GetNoteLinks(c); err != nil {
		t.Fatalf("GetNoteLinks returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListNoteLinkGraphFiltersTrashByDefault(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestListNoteLinkGraphFiltersTrashByDefault?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	activeTarget := client.Note.Create().SetTitle("Active Target").SetUserID(owner.ID).SaveX(ctx)
	deletedTarget := client.Note.Create().SetTitle("Deleted Target").SetIsDeleted(true).SetUserID(owner.ID).SaveX(ctx)
	activeSource := client.Note.Create().SetTitle("Active Source").SetContent("[[Active Target]] and [[Deleted Target]]").SetUserID(owner.ID).SaveX(ctx)
	deletedSource := client.Note.Create().SetTitle("Deleted Source").SetContent("[[Active Target]]").SetIsDeleted(true).SetUserID(owner.ID).SaveX(ctx)
	service := notes.NewService(client)
	if err := service.SyncNoteLinks(ctx, owner.ID, activeSource.ID); err != nil {
		t.Fatalf("sync active source links: %v", err)
	}
	if err := service.SyncNoteLinks(ctx, owner.ID, deletedSource.ID); err != nil {
		t.Fatalf("sync deleted source links: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/note-links", nil)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.Set("user_id", owner.ID)

	if err := NewHandler(client, nil).ListNoteLinkGraph(c); err != nil {
		t.Fatalf("ListNoteLinkGraph returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var graph struct {
		Nodes []map[string]any `json:"nodes"`
		Edges []map[string]any `json:"edges"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&graph); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(graph.Edges) != 1 {
		t.Fatalf("expected one non-trash edge, got %d: %+v", len(graph.Edges), graph.Edges)
	}
	if graph.Edges[0]["source"] != activeSource.ID.String() || graph.Edges[0]["target"] != activeTarget.ID.String() {
		t.Fatalf("unexpected edge: %+v", graph.Edges[0])
	}
	for _, node := range graph.Nodes {
		if strings.Contains(node["title"].(string), "Deleted") || node["id"] == deletedTarget.ID.String() {
			t.Fatalf("trash node should be filtered: %+v", node)
		}
	}
}

func TestWikiLinksTrackTargetLifecycleWithoutRewritingOtherSources(t *testing.T) {
	ctx := t.Context()
	client := enttest.Open(t, "sqlite3", "file:TestLinkLifecycle?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	user := client.User.Create().SetUsername("writer").SetPasswordHash("hash").SaveX(ctx)
	h := NewHandler(client, nil)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("user_id", user.ID); return next(c) }
	})
	e.POST("/notes", h.CreateNote)
	e.PUT("/notes/:id", h.UpdateNote)
	e.DELETE("/notes/:id", h.DeleteNote)
	request := func(method, path, body string) NoteResponse {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		var n NoteResponse
		if rec.Code < 200 || rec.Code >= 300 {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		if rec.Code != http.StatusNoContent {
			if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
				t.Fatal(err)
			}
		}
		return n
	}
	source := request("POST", "/notes", `{"title":"Source","content":"[[Target]]"}`)
	unrelated := request("POST", "/notes", `{"title":"Elsewhere","content":"[[Different]]"}`)
	otherLink := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(unrelated.ID)).OnlyX(ctx)
	assertTarget := func(id string) {
		t.Helper()
		row := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(source.ID)).OnlyX(ctx)
		if id == "" && row.TargetNoteID == nil {
			return
		}
		if row.TargetNoteID == nil || row.TargetNoteID.String() != id {
			t.Fatalf("target = %v, want %s", row.TargetNoteID, id)
		}
	}
	assertTarget("")
	first := request("POST", "/notes", `{"title":"Target"}`)
	assertTarget(first.ID.String())
	second := request("POST", "/notes", `{"title":"Target"}`)
	assertTarget("")
	request("PUT", "/notes/"+second.ID.String(), `{"title":"Renamed"}`)
	assertTarget(first.ID.String())
	request("DELETE", "/notes/"+first.ID.String(), "")
	assertTarget("")
	request("PUT", "/notes/"+second.ID.String(), `{"title":"Target"}`)
	assertTarget(second.ID.String())
	request("PUT", "/notes/"+second.ID.String(), `{"content":"just writing"}`)
	if got := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(unrelated.ID)).OnlyX(ctx).ID; got != otherLink.ID {
		t.Fatal("unrelated note links were rewritten")
	}
}
