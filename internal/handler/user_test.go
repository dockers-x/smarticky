package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"smarticky/ent/enttest"

	"github.com/labstack/echo/v4"
	_ "github.com/lib-x/entsqlite"
)

func TestUpdateUserPersistsShareSignature(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateUserPersistsShareSignature?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	h := NewHandler(client, nil)
	e := echo.New()

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/users/1",
		strings.NewReader(`{"share_signature":"  Alice Notes  "}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.Set("role", "user")
	c.SetParamNames("id")
	c.SetParamValues(strconv.Itoa(u.ID))

	if err := h.UpdateUser(c); err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var updated struct {
		ShareSignature string `json:"share_signature"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.ShareSignature != "Alice Notes" {
		t.Fatalf("expected trimmed share signature, got %q", updated.ShareSignature)
	}

	fromDB := client.User.GetX(ctx, u.ID)
	if fromDB.ShareSignature != "Alice Notes" {
		t.Fatalf("expected DB share signature to be persisted, got %q", fromDB.ShareSignature)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meRec := httptest.NewRecorder()
	meCtx := e.NewContext(meReq, meRec)
	meCtx.Set("user_id", u.ID)

	if err := h.GetCurrentUser(meCtx); err != nil {
		t.Fatalf("GetCurrentUser returned error: %v", err)
	}
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, meRec.Code, meRec.Body.String())
	}

	var current struct {
		ShareSignature string `json:"share_signature"`
	}
	if err := json.NewDecoder(meRec.Body).Decode(&current); err != nil {
		t.Fatalf("decode current user response: %v", err)
	}
	if current.ShareSignature != "Alice Notes" {
		t.Fatalf("expected current user share signature, got %q", current.ShareSignature)
	}
}

func TestUpdateUserPersistsTimeZone(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateUserPersistsTimeZone?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	h := NewHandler(client, nil)
	e := echo.New()

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/users/1",
		strings.NewReader(`{"time_zone":"Asia/Shanghai"}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.Set("role", "user")
	c.SetParamNames("id")
	c.SetParamValues(strconv.Itoa(u.ID))

	if err := h.UpdateUser(c); err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var updated struct {
		TimeZone string `json:"time_zone"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.TimeZone != "Asia/Shanghai" {
		t.Fatalf("expected response time zone Asia/Shanghai, got %q", updated.TimeZone)
	}

	fromDB := client.User.GetX(ctx, u.ID)
	if fromDB.TimeZone != "Asia/Shanghai" {
		t.Fatalf("expected DB time zone to be persisted, got %q", fromDB.TimeZone)
	}
}

func TestUpdateUserRejectsInvalidTimeZone(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateUserRejectsInvalidTimeZone?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	h := NewHandler(client, nil)
	e := echo.New()

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/users/1",
		strings.NewReader(`{"time_zone":"Mars/Base"}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", u.ID)
	c.Set("role", "user")
	c.SetParamNames("id")
	c.SetParamValues(strconv.Itoa(u.ID))

	if err := h.UpdateUser(c); err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	fromDB := client.User.GetX(ctx, u.ID)
	if fromDB.TimeZone != "UTC" {
		t.Fatalf("expected invalid update to leave default UTC, got %q", fromDB.TimeZone)
	}
}
