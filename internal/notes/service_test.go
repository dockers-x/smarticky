package notes

import (
	"context"
	"testing"

	"smarticky/ent"
	"smarticky/ent/enttest"
	"smarticky/ent/note"
	searchsvc "smarticky/internal/search"

	_ "github.com/lib-x/entsqlite"
)

func TestServiceRedactsLockedNoteContent(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestServiceRedactsLockedNoteContent?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().SetUsername("alice").SetPasswordHash("hash").SaveX(ctx)
	n := client.Note.Create().
		SetTitle("Secret").
		SetContent("hidden").
		SetProtectionMode(note.ProtectionModePassword).
		SetUserID(u.ID).
		SaveX(ctx)

	service := NewService(client)
	view, err := service.Get(ctx, u.ID, n.ID, true)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if view.Content != "" || !view.ContentRedacted {
		t.Fatalf("expected locked content to be redacted, got content=%q redacted=%v", view.Content, view.ContentRedacted)
	}
}

func TestServiceCreateAssignsOwner(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestServiceCreateAssignsOwner?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	other := client.User.Create().SetUsername("other").SetPasswordHash("hash").SaveX(ctx)

	service := NewService(client)
	created, err := service.Create(ctx, owner.ID, CreateInput{Title: "Created", Content: "Body"})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if _, err := service.Get(ctx, other.ID, created.ID, false); err == nil {
		t.Fatal("expected other user to be unable to read created note")
	}
}

func TestServiceSearchDoesNotMatchLockedContentWhenRedacting(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestServiceSearchDoesNotMatchLockedContentWhenRedacting?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().SetUsername("alice").SetPasswordHash("hash").SaveX(ctx)
	client.Note.Create().
		SetTitle("Private").
		SetContent("needle").
		SetProtectionMode(note.ProtectionModeEncrypted).
		SetEncryptedContent("ciphertext").
		SetEncryptionAlg("aes-gcm").
		SetEncryptionKdf("argon2id").
		SetEncryptionSalt("salt").
		SetEncryptionNonce("nonce").
		SetUserID(u.ID).
		SaveX(ctx)
	client.Note.Create().
		SetTitle("Public").
		SetContent("needle").
		SetUserID(u.ID).
		SaveX(ctx)

	service := NewService(client)
	rows, err := service.List(ctx, u.ID, ListOptions{Query: "needle", RedactLocked: true})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Title != "Public" {
		t.Fatalf("expected only public content match, got %+v", rows)
	}
}

func TestServiceIndexedSearchEnforcesOwnership(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestServiceIndexedSearchEnforcesOwnership?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	other := client.User.Create().SetUsername("other").SetPasswordHash("hash").SaveX(ctx)
	owned := client.Note.Create().SetTitle("Owned").SetContent("indexed needle").SetUserID(owner.ID).SaveX(ctx)
	foreign := client.Note.Create().SetTitle("Foreign").SetContent("indexed needle").SetUserID(other.ID).SaveX(ctx)

	index, err := searchsvc.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	for _, row := range []*ent.Note{owned, foreign} {
		if err := index.IndexNote(ctx, row); err != nil {
			t.Fatalf("IndexNote: %v", err)
		}
	}

	service := NewService(client, index)
	rows, err := service.List(ctx, owner.ID, ListOptions{Query: "needle"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != owned.ID {
		t.Fatalf("expected only owner note, got %+v", rows)
	}
}

func TestServiceIndexedSearchDoesNotMatchEncryptedContent(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestServiceIndexedSearchDoesNotMatchEncryptedContent?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	plain := client.Note.Create().
		SetTitle("Plain").
		SetContent("service needle").
		SetUserID(u.ID).
		SaveX(ctx)
	encrypted := client.Note.Create().
		SetTitle("Encrypted Title").
		SetContent("").
		SetProtectionMode(note.ProtectionModeEncrypted).
		SetEncryptedContent("service needle ciphertext").
		SetEncryptionAlg("aes-gcm").
		SetEncryptionKdf("pbkdf2-sha256:310000").
		SetEncryptionSalt("salt").
		SetEncryptionNonce("nonce").
		SetUserID(u.ID).
		SaveX(ctx)

	index, err := searchsvc.NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	for _, row := range []*ent.Note{plain, encrypted} {
		if err := index.IndexNote(ctx, row); err != nil {
			t.Fatalf("IndexNote: %v", err)
		}
	}

	service := NewService(client, index)
	rows, err := service.List(ctx, u.ID, ListOptions{Query: "service"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != plain.ID {
		t.Fatalf("expected only plain indexed content match, got %+v", rows)
	}
}
