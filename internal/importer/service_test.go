package importer

import (
	"context"
	"strings"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/internal/storage"

	_ "github.com/lib-x/entsqlite"
)

func TestPreviewEvernoteCreatesJobAndPendingItem(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestPreviewEvernoteCreatesJobAndPendingItem?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	input := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<en-export>
  <note>
    <title>Meeting</title>
    <content><![CDATA[<en-note>Hello</en-note>]]></content>
    <created>20250101T010203Z</created>
    <tag>Work</tag>
  </note>
</en-export>`)

	service := NewService(client, storage.NewMemoryFileSystem())
	result, err := service.PreviewEvernote(ctx, u.ID, "meeting.enex", input)
	if err != nil {
		t.Fatal(err)
	}

	if result.Job.Status != "previewed" {
		t.Fatalf("expected job status previewed, got %q", result.Job.Status)
	}
	if result.Job.NoteCount != 1 {
		t.Fatalf("expected note count 1, got %d", result.Job.NoteCount)
	}
	if result.JobID != result.Job.ID || result.NoteCount != 1 || result.TagCount != 1 {
		t.Fatalf("expected flat preview counts to match job, got job_id=%d note_count=%d tag_count=%d", result.JobID, result.NoteCount, result.TagCount)
	}

	items := result.Job.QueryItems().AllX(ctx)
	if len(items) != 1 {
		t.Fatalf("expected 1 import item, got %d", len(items))
	}
	if items[0].Status != "pending" {
		t.Fatalf("expected item status pending, got %q", items[0].Status)
	}
}

func TestConfirmEvernoteImportsAndSkipsDuplicate(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestConfirmEvernoteImportsAndSkipsDuplicate?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)

	input := `<?xml version="1.0" encoding="UTF-8"?>
<en-export>
  <note>
    <title>Meeting</title>
    <content><![CDATA[<en-note><div>Hello</div></en-note>]]></content>
    <created>20250101T010203Z</created>
  </note>
</en-export>`

	service := NewService(client, storage.NewMemoryFileSystem())
	preview, err := service.PreviewEvernote(ctx, u.ID, "meeting.enex", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.ConfirmEvernote(ctx, u.ID, preview.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 1 || result.Skipped != 0 || result.Failed != 0 {
		t.Fatalf("expected imported=1 skipped=0 failed=0, got imported=%d skipped=%d failed=%d", result.Imported, result.Skipped, result.Failed)
	}
	if result.JobID != preview.Job.ID || result.Status != "completed" || result.ImportedCount != 1 {
		t.Fatalf("expected flat import result fields, got job_id=%d status=%q imported_count=%d", result.JobID, result.Status, result.ImportedCount)
	}

	notes := client.Note.Query().AllX(ctx)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note after first import, got %d", len(notes))
	}
	if notes[0].Content != "Hello" {
		t.Fatalf("expected imported content Hello, got %q", notes[0].Content)
	}

	again, err := service.ConfirmEvernote(ctx, u.ID, preview.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Imported != 1 || again.Skipped != 0 || again.Failed != 0 {
		t.Fatalf("expected repeated confirm to preserve counts, got imported=%d skipped=%d failed=%d", again.Imported, again.Skipped, again.Failed)
	}

	duplicatePreview, err := service.PreviewEvernote(ctx, u.ID, "meeting.enex", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	duplicateResult, err := service.ConfirmEvernote(ctx, u.ID, duplicatePreview.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if duplicateResult.Imported != 0 || duplicateResult.Skipped != 1 || duplicateResult.Failed != 0 {
		t.Fatalf("expected duplicate import to skip, got imported=%d skipped=%d failed=%d", duplicateResult.Imported, duplicateResult.Skipped, duplicateResult.Failed)
	}

	notes = client.Note.Query().AllX(ctx)
	if len(notes) != 1 {
		t.Fatalf("expected duplicate import to keep 1 note, got %d", len(notes))
	}
}
