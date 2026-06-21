package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/ent/note"
	"smarticky/ent/user"
	"smarticky/internal/notes"
	"smarticky/internal/shareimage"

	_ "github.com/lib-x/entsqlite"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestAuthenticatorResolvesBearerTokenToUser(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestAuthenticatorResolvesBearerTokenToUser?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SaveX(ctx)
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	client.MCPToken.Create().
		SetName("test").
		SetTokenHash(HashToken(token)).
		SetUserID(u.ID).
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	principal, err := NewAuthenticator(client, false).Resolve(ctx, req)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if principal.UserID != u.ID {
		t.Fatalf("expected user %d, got %d", u.ID, principal.UserID)
	}
}

func TestAuthenticatorResolvesTrustedLazyCatHeaders(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestAuthenticatorResolvesTrustedLazyCatHeaders?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SetLazycatUID("lazycat").
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("X-HC-User-ID", "lazycat")
	req.Header.Set("X-HC-SOURCE", "app:cloud.lazycat.app.agent")

	principal, err := NewAuthenticator(client, true).Resolve(ctx, req)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if principal.UserID != u.ID {
		t.Fatalf("expected user %d, got %d", u.ID, principal.UserID)
	}
}

func TestAuthenticatorRejectsLazyCatHeadersWhenTrustDisabled(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestAuthenticatorRejectsLazyCatHeadersWhenTrustDisabled?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SetLazycatUID("lazycat").
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("X-HC-User-ID", "lazycat")
	req.Header.Set("X-HC-SOURCE", "app:cloud.lazycat.app.agent")

	if _, err := NewAuthenticator(client, false).Resolve(ctx, req); err == nil {
		t.Fatal("expected Resolve to reject untrusted LazyCat headers")
	}
}

func TestAuthenticatorRejectsClientLazyCatSource(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestAuthenticatorRejectsClientLazyCatSource?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SetLazycatUID("lazycat").
		SaveX(ctx)

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("X-HC-User-ID", "lazycat")
	req.Header.Set("X-HC-SOURCE", "client")

	if _, err := NewAuthenticator(client, true).Resolve(ctx, req); err == nil {
		t.Fatal("expected Resolve to reject client source")
	}
}

func TestNewHTTPHandlerBuildsToolSchemas(t *testing.T) {
	client := enttest.Open(t, "sqlite3", "file:TestNewHTTPHandlerBuildsToolSchemas?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	handler := NewHTTPHandler(client, notes.NewService(client), shareimage.NewService(client, t.TempDir()), false)
	if handler == nil {
		t.Fatal("expected MCP HTTP handler")
	}
}

func TestHTTPHandlerCreateNoteWithBearerToken(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestHTTPHandlerCreateNoteWithBearerToken?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SaveX(ctx)
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	client.MCPToken.Create().
		SetName("test").
		SetTokenHash(HashToken(token)).
		SetUserID(u.ID).
		SaveX(ctx)

	httpServer := httptest.NewServer(NewHTTPHandler(
		client,
		notes.NewService(client),
		shareimage.NewService(client, t.TempDir()),
		false,
	))
	defer httpServer.Close()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	session, err := mcpClient.Connect(ctx, &mcpsdk.StreamableClientTransport{
		Endpoint:             httpServer.URL,
		HTTPClient:           &http.Client{Transport: bearerTransport{token: token}},
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "smarticky_create_note",
		Arguments: map[string]any{
			"title":   "MCP note",
			"content": "created through MCP",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.GetError())
	}

	count := client.Note.Query().
		Where(note.TitleEQ("MCP note"), note.HasUserWith(user.IDEQ(u.ID))).
		CountX(ctx)
	if count != 1 {
		t.Fatalf("expected one created note for user, got %d", count)
	}
}

func TestHTTPHandlerGenerateImageReturnsTokenDownloadURL(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestHTTPHandlerGenerateImageReturnsTokenDownloadURL?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("alice").
		SetPasswordHash("hash").
		SaveX(ctx)
	token, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	client.MCPToken.Create().
		SetName("test").
		SetTokenHash(HashToken(token)).
		SetUserID(u.ID).
		SaveX(ctx)

	mux := http.NewServeMux()
	mux.Handle("/mcp", NewHTTPHandler(
		client,
		notes.NewService(client),
		shareimage.NewService(client, t.TempDir()),
		false,
	))
	mux.Handle("/mcp/images/", NewImageDownloadHandler(
		client,
		shareimage.NewService(client, t.TempDir()),
		false,
	))
	httpServer := httptest.NewServer(mux)
	defer httpServer.Close()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	httpClient := &http.Client{Transport: bearerTransport{token: token}}
	session, err := mcpClient.Connect(ctx, &mcpsdk.StreamableClientTransport{
		Endpoint:             httpServer.URL + "/mcp",
		HTTPClient:           httpClient,
		DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer session.Close()

	result, err := session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "smarticky_generate_note_image",
		Arguments: map[string]any{
			"title":   "MCP image",
			"content": "rendered through MCP",
		},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("tool returned error: %v", result.GetError())
	}

	var output imageToolOutput
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(raw, &output); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
	if !strings.HasPrefix(output.DownloadURL, httpServer.URL+"/mcp/images/") {
		t.Fatalf("expected MCP image download URL, got %q", output.DownloadURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, output.DownloadURL, nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("image download returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected image download status 200, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != shareimage.ContentTypePNG {
		t.Fatalf("expected %s content type, got %q", shareimage.ContentTypePNG, got)
	}
}

type imageToolOutput struct {
	DownloadURL string `json:"download_url"`
}

type bearerTransport struct {
	token string
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	next := req.Clone(req.Context())
	next.Header = req.Header.Clone()
	next.Header.Set("Authorization", "Bearer "+t.token)
	return http.DefaultTransport.RoundTrip(next)
}
