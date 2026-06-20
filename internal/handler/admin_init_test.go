package handler

import (
	"context"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/ent/user"
	"smarticky/internal/storage"

	_ "github.com/lib-x/entsqlite"
	"golang.org/x/crypto/bcrypt"
)

func mapGetenv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func TestInitializeAdminFromEnvCreatesAdminOnlyForEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestInitializeAdminFromEnvCreatesAdminOnlyForEmptyDatabase?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	h := NewHandler(client, storage.NewMemoryFileSystem())
	created, err := h.InitializeAdminFromEnv(ctx, mapGetenv(map[string]string{
		envAdminUsername: "admin",
		envAdminPassword: "admin-password",
		envAdminEmail:    "admin@example.com",
		envAdminNickname: "Owner",
	}))
	if err != nil {
		t.Fatalf("InitializeAdminFromEnv returned error: %v", err)
	}
	if !created {
		t.Fatal("expected admin to be created")
	}

	admin := client.User.Query().Where(user.UsernameEQ("admin")).OnlyX(ctx)
	if admin.Role != user.RoleAdmin {
		t.Fatalf("expected admin role, got %s", admin.Role)
	}
	if admin.Email != "admin@example.com" {
		t.Fatalf("expected email to be set, got %q", admin.Email)
	}
	if admin.Nickname != "Owner" {
		t.Fatalf("expected nickname to be set, got %q", admin.Nickname)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte("admin-password")); err != nil {
		t.Fatalf("password hash does not match env password: %v", err)
	}
}

func TestInitializeAdminFromEnvIgnoresEnvAfterAnyUserExists(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestInitializeAdminFromEnvIgnoresEnvAfterAnyUserExists?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	existing := client.User.Create().
		SetUsername("existing").
		SetPasswordHash("old-hash").
		SetRole(user.RoleUser).
		SaveX(ctx)

	h := NewHandler(client, storage.NewMemoryFileSystem())
	created, err := h.InitializeAdminFromEnv(ctx, mapGetenv(map[string]string{
		envAdminUsername: "admin",
		envAdminPassword: "admin-password",
	}))
	if err != nil {
		t.Fatalf("InitializeAdminFromEnv returned error: %v", err)
	}
	if created {
		t.Fatal("expected no admin creation once a user exists")
	}

	if count := client.User.Query().CountX(ctx); count != 1 {
		t.Fatalf("expected one user, got %d", count)
	}
	unchanged := client.User.GetX(ctx, existing.ID)
	if unchanged.PasswordHash != "old-hash" {
		t.Fatalf("expected existing password hash to remain unchanged, got %q", unchanged.PasswordHash)
	}
	if unchanged.Role != user.RoleUser {
		t.Fatalf("expected existing role to remain user, got %s", unchanged.Role)
	}
	if exists := client.User.Query().Where(user.UsernameEQ("admin")).ExistX(ctx); exists {
		t.Fatal("did not expect env admin to be created in non-empty database")
	}
}

func TestInitializeAdminFromEnvRejectsPartialCredentialsForEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestInitializeAdminFromEnvRejectsPartialCredentialsForEmptyDatabase?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	h := NewHandler(client, storage.NewMemoryFileSystem())
	created, err := h.InitializeAdminFromEnv(ctx, mapGetenv(map[string]string{
		envAdminUsername: "admin",
	}))
	if err == nil {
		t.Fatal("expected partial env credentials to fail")
	}
	if created {
		t.Fatal("expected no admin to be created")
	}
	if count := client.User.Query().CountX(ctx); count != 0 {
		t.Fatalf("expected empty database to remain empty, got %d users", count)
	}
}
