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

func TestExcalidrawLibraryIsScopedToUser(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestExcalidrawLibraryIsScopedToUser?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	alice := client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SaveX(ctx)
	bob := client.User.Create().
		SetUsername("bob").
		SetPasswordHash("hash").
		SaveX(ctx)

	h := NewHandler(client, nil)
	e := echo.New()

	saveLibrary := func(userID int, body string) ExcalidrawLibraryResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodPut, "/api/excalidraw/library", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", userID)

		if err := h.UpdateExcalidrawLibrary(c); err != nil {
			t.Fatalf("UpdateExcalidrawLibrary returned error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var response ExcalidrawLibraryResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode library response: %v", err)
		}
		return response
	}

	readLibrary := func(userID int) ExcalidrawLibraryResponse {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/excalidraw/library", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set("user_id", userID)

		if err := h.GetExcalidrawLibrary(c); err != nil {
			t.Fatalf("GetExcalidrawLibrary returned error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var response ExcalidrawLibraryResponse
		if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
			t.Fatalf("decode library response: %v", err)
		}
		return response
	}

	aliceLibrary := saveLibrary(alice.ID, `{"library_json":"[{\"id\":\"alice-item\",\"status\":\"published\",\"elements\":[],\"created\":1}]"}`)
	bobLibrary := saveLibrary(bob.ID, `{"library_json":"[{\"id\":\"bob-item\",\"status\":\"published\",\"elements\":[],\"created\":2}]"}`)

	if aliceLibrary.ID == bobLibrary.ID {
		t.Fatal("expected different users to have different library rows")
	}
	if got := readLibrary(alice.ID).LibraryJSON; !strings.Contains(got, "alice-item") || strings.Contains(got, "bob-item") {
		t.Fatalf("expected alice library only, got %s", got)
	}
	if got := readLibrary(bob.ID).LibraryJSON; !strings.Contains(got, "bob-item") || strings.Contains(got, "alice-item") {
		t.Fatalf("expected bob library only, got %s", got)
	}
}

func TestUpdateExcalidrawLibraryRejectsNonArrayJSON(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateExcalidrawLibraryRejectsNonArrayJSON?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/api/excalidraw/library", strings.NewReader(`{"library_json":"{\"id\":\"not-array\"}"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", owner.ID)

	h := NewHandler(client, nil)
	if err := h.UpdateExcalidrawLibrary(c); err != nil {
		t.Fatalf("UpdateExcalidrawLibrary returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
