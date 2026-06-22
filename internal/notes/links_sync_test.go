package notes

import (
	"context"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/ent/note"
	"smarticky/ent/notelink"

	_ "github.com/lib-x/entsqlite"
)

func TestSyncNoteLinksResolvesAndAggregatesCurrentUserTargets(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestSyncNoteLinksResolvesAndAggregatesCurrentUserTargets?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	other := client.User.Create().SetUsername("other").SetPasswordHash("hash").SaveX(ctx)
	target := client.Note.Create().SetTitle("Target").SetUserID(owner.ID).SaveX(ctx)
	client.Note.Create().SetTitle("Other Only").SetUserID(other.ID).SaveX(ctx)
	source := client.Note.Create().
		SetTitle("Source").
		SetContent("[[Target]] and [[target|again]] and [[Other Only]]").
		SetUserID(owner.ID).
		SaveX(ctx)

	if err := NewService(client).SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync links: %v", err)
	}

	rows := client.NoteLink.Query().
		Where(notelink.SourceNoteIDEQ(source.ID)).
		Order(notelink.ByTargetRef()).
		AllX(ctx)
	if len(rows) != 2 {
		t.Fatalf("expected 2 links, got %d: %+v", len(rows), rows)
	}
	if rows[0].TargetRef != "Other Only" || rows[0].TargetNoteID != nil || rows[0].TargetKey != "title:other only" {
		t.Fatalf("expected foreign user title to remain unresolved, got %+v", rows[0])
	}
	if rows[1].TargetRef != "Target" || rows[1].TargetNoteID == nil || *rows[1].TargetNoteID != target.ID || rows[1].OccurrenceCount != 2 {
		t.Fatalf("expected resolved aggregated target link, got %+v", rows[1])
	}
}

func TestSyncNoteLinksLeavesAmbiguousFoldedTitleUnresolved(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestSyncNoteLinksLeavesAmbiguousFoldedTitleUnresolved?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	client.Note.Create().SetTitle("Project").SetUserID(owner.ID).SaveX(ctx)
	client.Note.Create().SetTitle("project").SetUserID(owner.ID).SaveX(ctx)
	source := client.Note.Create().SetTitle("Source").SetContent("[[PROJECT]]").SetUserID(owner.ID).SaveX(ctx)

	if err := NewService(client).SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync links: %v", err)
	}

	row := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(source.ID)).OnlyX(ctx)
	if row.TargetNoteID != nil || row.TargetKey != "title:project" {
		t.Fatalf("expected ambiguous folded title to remain unresolved, got %+v", row)
	}
}

func TestSyncNoteLinksClearsOutgoingForEncryptedNotes(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestSyncNoteLinksClearsOutgoingForEncryptedNotes?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	owner := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)
	target := client.Note.Create().SetTitle("Target").SetUserID(owner.ID).SaveX(ctx)
	source := client.Note.Create().SetTitle("Source").SetContent("[[Target]]").SetUserID(owner.ID).SaveX(ctx)

	service := NewService(client)
	if err := service.SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync plain links: %v", err)
	}
	if count := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(source.ID), notelink.TargetNoteIDEQ(target.ID)).CountX(ctx); count != 1 {
		t.Fatalf("expected one outgoing link before encryption, got %d", count)
	}

	source.Update().
		SetProtectionMode(note.ProtectionModeEncrypted).
		SetContent("").
		SetEncryptedContent("ciphertext").
		SetEncryptionAlg("aes-gcm").
		SetEncryptionKdf("argon2id").
		SetEncryptionSalt("salt").
		SetEncryptionNonce("nonce").
		SaveX(ctx)
	if err := service.SyncNoteLinks(ctx, owner.ID, source.ID); err != nil {
		t.Fatalf("sync encrypted links: %v", err)
	}
	if count := client.NoteLink.Query().Where(notelink.SourceNoteIDEQ(source.ID)).CountX(ctx); count != 0 {
		t.Fatalf("expected encrypted note outgoing links to be cleared, got %d", count)
	}
}
