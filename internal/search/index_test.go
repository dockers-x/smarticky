package search

import (
	"context"
	"slices"
	"testing"

	"smarticky/ent"
	"smarticky/ent/enttest"
	"smarticky/ent/note"

	_ "github.com/lib-x/entsqlite"
)

func TestSearchIndexesPlainAndPasswordContentButNotEncryptedContent(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestSearchIndexesPlainAndPasswordContentButNotEncryptedContent?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	plain := client.Note.Create().SetTitle("Plain").SetContent("needle public").SetUserID(owner.ID).SaveX(ctx)
	locked := client.Note.Create().
		SetTitle("Locked").
		SetContent("needle gated").
		SetProtectionMode(note.ProtectionModePassword).
		SetProtectionPasswordHash("hash").
		SetUserID(owner.ID).
		SaveX(ctx)
	encrypted := client.Note.Create().
		SetTitle("Encrypted Title").
		SetContent("").
		SetProtectionMode(note.ProtectionModeEncrypted).
		SetEncryptedContent("needle ciphertext").
		SetEncryptionAlg("aes-gcm").
		SetEncryptionKdf("pbkdf2-sha256:310000").
		SetEncryptionSalt("salt").
		SetEncryptionNonce("nonce").
		SetUserID(owner.ID).
		SaveX(ctx)

	svc, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	for _, row := range []*ent.Note{plain, locked, encrypted} {
		if err := svc.IndexNote(ctx, row); err != nil {
			t.Fatalf("IndexNote: %v", err)
		}
	}

	bodyMatches, err := svc.Search(ctx, SearchOptions{UserID: owner.ID, Query: "needle", Limit: 10})
	if err != nil {
		t.Fatalf("Search body: %v", err)
	}
	if slices.Contains(bodyMatches, encrypted.ID) {
		t.Fatalf("encrypted body/ciphertext must not be searchable, got %v", bodyMatches)
	}
	if !slices.Contains(bodyMatches, plain.ID) || !slices.Contains(bodyMatches, locked.ID) {
		t.Fatalf("expected plain and password notes to match body, got %v", bodyMatches)
	}

	titleMatches, err := svc.Search(ctx, SearchOptions{UserID: owner.ID, Query: "Encrypted", Limit: 10})
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if !slices.Contains(titleMatches, encrypted.ID) {
		t.Fatalf("encrypted title should be searchable, got %v", titleMatches)
	}
}

func TestSearchRebuildUsesDatabaseRows(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestSearchRebuildUsesDatabaseRows?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	plain := client.Note.Create().
		SetTitle("Rebuild Plain").
		SetContent("rebuild public").
		SetUserID(owner.ID).
		SaveX(ctx)
	encrypted := client.Note.Create().
		SetTitle("Encrypted Title").
		SetContent("").
		SetProtectionMode(note.ProtectionModeEncrypted).
		SetEncryptedContent("rebuild ciphertext").
		SetEncryptionAlg("aes-gcm").
		SetEncryptionKdf("pbkdf2-sha256:310000").
		SetEncryptionSalt("salt").
		SetEncryptionNonce("nonce").
		SetUserID(owner.ID).
		SaveX(ctx)

	svc, err := NewMemory()
	if err != nil {
		t.Fatalf("NewMemory: %v", err)
	}
	if err := svc.Rebuild(ctx, client); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}

	bodyMatches, err := svc.Search(ctx, SearchOptions{UserID: owner.ID, Query: "rebuild", Limit: 10})
	if err != nil {
		t.Fatalf("Search body: %v", err)
	}
	if !slices.Contains(bodyMatches, plain.ID) {
		t.Fatalf("expected plain body to match, got %v", bodyMatches)
	}
	if slices.Contains(bodyMatches, encrypted.ID) {
		t.Fatalf("encrypted ciphertext must not match, got %v", bodyMatches)
	}

	titleMatches, err := svc.Search(ctx, SearchOptions{UserID: owner.ID, Query: "Encrypted", Limit: 10})
	if err != nil {
		t.Fatalf("Search title: %v", err)
	}
	if !slices.Contains(titleMatches, encrypted.ID) {
		t.Fatalf("expected encrypted title to match, got %v", titleMatches)
	}
}
