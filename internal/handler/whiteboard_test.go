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

func TestCreateAndUpdateWhiteboardForOwnedNote(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestCreateAndUpdateWhiteboardForOwnedNote?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	n := client.Note.Create().
		SetTitle("note").
		SetUserID(owner.ID).
		SaveX(ctx)

	h := NewHandler(client, nil)
	e := echo.New()

	createReq := httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/whiteboards", strings.NewReader(`{"title":"Sketch","scene_json":"{\"elements\":[],\"files\":{}}"}`))
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set("user_id", owner.ID)
	createCtx.SetParamNames("id")
	createCtx.SetParamValues(n.ID.String())

	if err := h.CreateWhiteboard(createCtx); err != nil {
		t.Fatalf("CreateWhiteboard returned error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, createRec.Code, createRec.Body.String())
	}

	var created WhiteboardResponse
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode created response: %v", err)
	}
	if created.NoteID != n.ID {
		t.Fatalf("expected note id %s, got %s", n.ID, created.NoteID)
	}
	if created.Title != "Sketch" {
		t.Fatalf("expected title Sketch, got %q", created.Title)
	}

	updateReq := httptest.NewRequest(http.MethodPut, "/api/whiteboards/"+created.ID.String(), strings.NewReader(`{"scene_json":"{\"elements\":[{\"id\":\"a\"}],\"files\":{}}"}`))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.Set("user_id", owner.ID)
	updateCtx.SetParamNames("id")
	updateCtx.SetParamValues(created.ID.String())

	if err := h.UpdateWhiteboard(updateCtx); err != nil {
		t.Fatalf("UpdateWhiteboard returned error: %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, updateRec.Code, updateRec.Body.String())
	}

	stored := client.Whiteboard.GetX(ctx, created.ID)
	if !strings.Contains(stored.SceneJSON, `"id":"a"`) {
		t.Fatalf("expected updated scene json, got %s", stored.SceneJSON)
	}
}

func TestWhiteboardRejectsOtherUsersNote(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestWhiteboardRejectsOtherUsersNote?mode=memory&cache=shared&_pragma=foreign_keys(1)")
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
		SetTitle("private note").
		SetUserID(owner.ID).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/notes/"+n.ID.String()+"/whiteboards", strings.NewReader(`{"scene_json":"{}"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", other.ID)
	c.SetParamNames("id")
	c.SetParamValues(n.ID.String())

	h := NewHandler(client, nil)
	if err := h.CreateWhiteboard(c); err != nil {
		t.Fatalf("CreateWhiteboard returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
	if count := client.Whiteboard.Query().CountX(ctx); count != 0 {
		t.Fatalf("expected no whiteboards, got %d", count)
	}
}

func TestUpdateWhiteboardRejectsInvalidSceneJSON(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateWhiteboardRejectsInvalidSceneJSON?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	n := client.Note.Create().
		SetTitle("note").
		SetUserID(owner.ID).
		SaveX(ctx)
	w := client.Whiteboard.Create().
		SetTitle("Sketch").
		SetSceneJSON("{}").
		SetNoteID(n.ID).
		SetUserID(owner.ID).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/whiteboards/"+w.ID.String(), strings.NewReader(`{"scene_json":"{bad"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", owner.ID)
	c.SetParamNames("id")
	c.SetParamValues(w.ID.String())

	h := NewHandler(client, nil)
	if err := h.UpdateWhiteboard(c); err != nil {
		t.Fatalf("UpdateWhiteboard returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
