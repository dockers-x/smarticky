package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"smarticky/ent/enttest"
	connectsvc "smarticky/internal/connections"
	"smarticky/internal/storage"

	"github.com/labstack/echo/v4"
	_ "github.com/lib-x/entsqlite"
)

func TestListNoteConnectionTargetsRejectsDisabledAccount(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestListNoteConnectionTargetsRejectsDisabledAccount?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	account := client.NoteConnectionAccount.Create().
		SetName("Disabled SiYuan").
		SetProvider(connectsvc.ProviderSiYuan).
		SetEndpoint("http://127.0.0.1:6806").
		SetEnabled(false).
		SetAuthType("token").
		SetUserID(u.ID).
		SaveX(ctx)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/note-connections/accounts/"+strconv.Itoa(account.ID)+"/targets", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.SetParamNames("id")
	c.SetParamValues(strconv.Itoa(account.ID))

	h := NewHandler(client, storage.NewMemoryFileSystem())
	if err := h.ListNoteConnectionTargets(c); err != nil {
		t.Fatalf("ListNoteConnectionTargets returned error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"] != "Note connection account is disabled" {
		t.Fatalf("error = %q, want disabled account message", body["error"])
	}
}
