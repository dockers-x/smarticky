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

func TestCreateFolderRejectsDepthBeyondDefaultLimit(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestCreateFolderRejectsDepthBeyondDefaultLimit?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	root := client.Folder.Create().
		SetName("root").
		SetUserID(u.ID).
		SaveX(ctx)
	child := client.Folder.Create().
		SetName("child").
		SetUserID(u.ID).
		SetParent(root).
		SaveX(ctx)
	grandchild := client.Folder.Create().
		SetName("grandchild").
		SetUserID(u.ID).
		SetParent(child).
		SaveX(ctx)

	e := echo.New()
	body := `{"name":"too deep","parent_id":"` + grandchild.ID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/folders", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)

	h := NewHandler(client, nil)
	if err := h.CreateFolder(c); err != nil {
		t.Fatalf("CreateFolder returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestUpdateFolderRejectsMoveUnderDescendant(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateFolderRejectsMoveUnderDescendant?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	root := client.Folder.Create().
		SetName("root").
		SetUserID(u.ID).
		SaveX(ctx)
	child := client.Folder.Create().
		SetName("child").
		SetUserID(u.ID).
		SetParent(root).
		SaveX(ctx)

	e := echo.New()
	body := `{"parent_id":"` + child.ID.String() + `"}`
	req := httptest.NewRequest(http.MethodPut, "/api/folders/"+root.ID.String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.SetParamNames("id")
	c.SetParamValues(root.ID.String())

	h := NewHandler(client, nil)
	if err := h.UpdateFolder(c); err != nil {
		t.Fatalf("UpdateFolder returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestDeleteFolderRejectsNonEmptyFolder(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestDeleteFolderRejectsNonEmptyFolder?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	f := client.Folder.Create().
		SetName("work").
		SetUserID(u.ID).
		SaveX(ctx)
	client.Note.Create().
		SetTitle("note").
		SetUserID(u.ID).
		SetFolder(f).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/folders/"+f.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.SetParamNames("id")
	c.SetParamValues(f.ID.String())

	h := NewHandler(client, nil)
	if err := h.DeleteFolder(c); err != nil {
		t.Fatalf("DeleteFolder returned error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d: %s", http.StatusConflict, rec.Code, rec.Body.String())
	}
}

func TestDeleteFolderClearsDeletedNotesInFolder(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestDeleteFolderClearsDeletedNotesInFolder?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	f := client.Folder.Create().
		SetName("archive").
		SetUserID(u.ID).
		SaveX(ctx)
	deletedNote := client.Note.Create().
		SetTitle("deleted").
		SetIsDeleted(true).
		SetUserID(u.ID).
		SetFolder(f).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/folders/"+f.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.SetParamNames("id")
	c.SetParamValues(f.ID.String())

	h := NewHandler(client, nil)
	if err := h.DeleteFolder(c); err != nil {
		t.Fatalf("DeleteFolder returned error: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
	if exists := client.Folder.Query().ExistX(ctx); exists {
		t.Fatal("expected folder to be deleted")
	}
	noteRow := client.Note.GetX(ctx, deletedNote.ID)
	if noteRow.QueryFolder().ExistX(ctx) {
		t.Fatal("expected deleted note to be unfiled after folder deletion")
	}
}

func TestMoveNotesAssignsFolderAndListFiltersByFolder(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestMoveNotesAssignsFolderAndListFiltersByFolder?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	f := client.Folder.Create().
		SetName("work").
		SetUserID(u.ID).
		SaveX(ctx)
	moved := client.Note.Create().
		SetTitle("moved").
		SetUserID(u.ID).
		SaveX(ctx)
	client.Note.Create().
		SetTitle("plain").
		SetUserID(u.ID).
		SaveX(ctx)

	e := echo.New()
	body := `{"note_ids":["` + moved.ID.String() + `"],"folder_id":"` + f.ID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/notes/move", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)

	h := NewHandler(client, nil)
	if err := h.MoveNotes(c); err != nil {
		t.Fatalf("MoveNotes returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/notes?folder_id="+f.ID.String(), nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set("user_id", u.ID)
	if err := h.ListNotes(listCtx); err != nil {
		t.Fatalf("ListNotes returned error: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status %d, got %d: %s", http.StatusOK, listRec.Code, listRec.Body.String())
	}

	var got []struct {
		Title    string  `json:"title"`
		FolderID *string `json:"folder_id"`
	}
	if err := json.NewDecoder(listRec.Body).Decode(&got); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(got) != 1 || got[0].Title != "moved" || got[0].FolderID == nil {
		t.Fatalf("unexpected folder list response: %+v", got)
	}
}

func TestMoveNotesRejectsOtherUsersFolder(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestMoveNotesRejectsOtherUsersFolder?mode=memory&cache=shared&_pragma=foreign_keys(1)")
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
		SetTitle("note").
		SetUserID(owner.ID).
		SaveX(ctx)
	otherFolder := client.Folder.Create().
		SetName("private").
		SetUserID(other.ID).
		SaveX(ctx)

	e := echo.New()
	body := `{"note_ids":["` + n.ID.String() + `"],"folder_id":"` + otherFolder.ID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/notes/move", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", owner.ID)

	h := NewHandler(client, nil)
	if err := h.MoveNotes(c); err != nil {
		t.Fatalf("MoveNotes returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNotFound, rec.Code, rec.Body.String())
	}
}
