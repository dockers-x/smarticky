package connections

import (
	"context"
	"errors"
	"strings"
	"testing"

	"smarticky/ent/backupconfig"
	"smarticky/ent/enttest"
	"smarticky/ent/folder"
	"smarticky/ent/note"
	"smarticky/ent/noteconnectionaccount"
	"smarticky/ent/noteconnectionitemmap"
	"smarticky/ent/noteconnectionjob"
	"smarticky/ent/tag"
	"smarticky/internal/secrets"
	"smarticky/internal/storage"

	_ "github.com/lib-x/entsqlite"
)

func TestAccountCredentialsAreEncryptedAndRedacted(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestAccountCredentialsAreEncryptedAndRedacted?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	box := testSecretBox(t)
	service := NewService(client, box)

	response, err := service.CreateAccount(ctx, u.ID, AccountInput{
		Name:     "Local SiYuan",
		Provider: ProviderSiYuan,
		Endpoint: "http://127.0.0.1:6806",
		Token:    stringPtr("siyuan-secret-token"),
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if !response.HasCredentials {
		t.Fatal("expected response to report saved credentials")
	}

	row := client.NoteConnectionAccount.Query().
		Where(noteconnectionaccount.ID(response.ID)).
		OnlyX(ctx)
	if strings.Contains(row.EncryptedCredentials, "siyuan-secret-token") {
		t.Fatalf("credential was stored in plaintext: %q", row.EncryptedCredentials)
	}
	credentials, err := service.decryptCredentials(row)
	if err != nil {
		t.Fatalf("decrypt credentials: %v", err)
	}
	if credentials.Token != "siyuan-secret-token" {
		t.Fatalf("unexpected decrypted token %q", credentials.Token)
	}
}

func TestUpdateAccountWithoutTokenKeepsStoredCredential(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestUpdateAccountWithoutTokenKeepsStoredCredential?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	service := NewService(client, testSecretBox(t))

	created, err := service.CreateAccount(ctx, u.ID, AccountInput{
		Name:     "Joplin",
		Provider: ProviderJoplin,
		Endpoint: "http://127.0.0.1:41184",
		Token:    stringPtr("joplin-token"),
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	before := client.NoteConnectionAccount.GetX(ctx, created.ID).EncryptedCredentials

	updated, err := service.UpdateAccount(ctx, u.ID, created.ID, AccountInput{
		Name:     "Joplin Desktop",
		Endpoint: "http://127.0.0.1:41184",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("update account: %v", err)
	}
	after := client.NoteConnectionAccount.GetX(ctx, updated.ID).EncryptedCredentials
	if before != after {
		t.Fatal("expected encrypted credentials to be preserved when token is omitted")
	}
}

func TestDeleteAccountRemovesConnectionRecordsButKeepsLocalNote(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestDeleteAccountRemovesConnectionRecordsButKeepsLocalNote?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	service := NewService(client, testSecretBox(t))

	created, err := service.CreateAccount(ctx, u.ID, AccountInput{
		Name:     "SiYuan",
		Provider: ProviderSiYuan,
		Endpoint: "http://127.0.0.1:6806",
		Token:    stringPtr("siyuan-token"),
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	localNote := client.Note.Create().
		SetTitle("Local").
		SetContent("body").
		SetUserID(u.ID).
		SaveX(ctx)
	client.NoteConnectionItemMap.Create().
		SetProvider(ProviderSiYuan).
		SetAccountID(created.ID).
		SetNoteID(localNote.ID).
		SetExternalID("remote-doc").
		ExecX(ctx)
	client.NoteConnectionJob.Create().
		SetProvider(ProviderSiYuan).
		SetOperation(OperationPush).
		SetStatus(JobCompleted).
		SetUserID(u.ID).
		SetAccountID(created.ID).
		SetNoteID(localNote.ID).
		ExecX(ctx)

	if err := service.DeleteAccount(ctx, u.ID, created.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	if _, err := client.Note.Get(ctx, localNote.ID); err != nil {
		t.Fatalf("expected local note to remain: %v", err)
	}
	if got := client.NoteConnectionItemMap.Query().
		Where(noteconnectionitemmap.AccountIDEQ(created.ID)).
		CountX(ctx); got != 0 {
		t.Fatalf("item map count after account delete = %d, want 0", got)
	}
	if got := client.NoteConnectionJob.Query().
		Where(noteconnectionjob.AccountIDEQ(created.ID)).
		CountX(ctx); got != 0 {
		t.Fatalf("job count after account delete = %d, want 0", got)
	}
	if exists := client.NoteConnectionAccount.Query().
		Where(noteconnectionaccount.ID(created.ID)).
		ExistX(ctx); exists {
		t.Fatal("expected account to be deleted")
	}
}

func TestDisabledAccountCannotSync(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestDisabledAccountCannotSync?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	account := client.NoteConnectionAccount.Create().
		SetName("Disabled SiYuan").
		SetProvider(ProviderSiYuan).
		SetEndpoint("http://127.0.0.1:6806").
		SetEnabled(false).
		SetAuthType("token").
		SetUserID(u.ID).
		SaveX(ctx)
	note := client.Note.Create().
		SetTitle("Local").
		SetContent("body").
		SetUserID(u.ID).
		SaveX(ctx)
	service := NewService(client, testSecretBox(t))

	if _, err := service.ListTargets(ctx, u.ID, account.ID); err == nil || err.Error() != "note connection account is disabled" {
		t.Fatalf("ListTargets error = %v, want disabled account error", err)
	}
	if _, err := service.ImportNotes(ctx, u.ID, account.ID, ImportRequest{}); err == nil || err.Error() != "note connection account is disabled" {
		t.Fatalf("ImportNotes error = %v, want disabled account error", err)
	}
	if _, err := service.PushNote(ctx, u.ID, account.ID, note.ID, "target"); err == nil || err.Error() != "note connection account is disabled" {
		t.Fatalf("PushNote error = %v, want disabled account error", err)
	}
}

func TestImportNotesRejectsConcurrentImportJob(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestImportNotesRejectsConcurrentImportJob?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	account := client.NoteConnectionAccount.Create().
		SetName("SiYuan").
		SetProvider(ProviderSiYuan).
		SetEndpoint("http://127.0.0.1:6806").
		SetEnabled(true).
		SetAuthType("token").
		SetUserID(u.ID).
		SaveX(ctx)
	client.NoteConnectionJob.Create().
		SetProvider(ProviderSiYuan).
		SetOperation(OperationImport).
		SetStatus(JobRunning).
		SetUserID(u.ID).
		SetAccountID(account.ID).
		ExecX(ctx)
	service := NewService(client, testSecretBox(t))

	_, err := service.ImportNotes(ctx, u.ID, account.ID, ImportRequest{})
	if !errors.Is(err, ErrJobRunning) {
		t.Fatalf("ImportNotes error = %v, want ErrJobRunning", err)
	}
}

func TestTargetKindRankKeepsNotebookBeforeDocument(t *testing.T) {
	targets := []Target{
		{ID: "doc-a", Name: "A", Kind: "document"},
		{ID: "box-1", Name: "Notebook", Kind: "notebook"},
	}

	if targetKindRank(targets[1].Kind) >= targetKindRank(targets[0].Kind) {
		t.Fatalf("notebook rank should sort before document: %#v", targets)
	}
}

func TestCreateImportedNotePreservesRemoteHierarchyAndRaisesFolderDepth(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestCreateImportedNotePreservesRemoteHierarchyAndRaisesFolderDepth?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	account := client.NoteConnectionAccount.Create().
		SetName("SiYuan").
		SetProvider(ProviderSiYuan).
		SetEndpoint("http://127.0.0.1:6806").
		SetEnabled(true).
		SetAuthType("token").
		SetUserID(u.ID).
		SaveX(ctx)
	client.BackupConfig.Create().SetFolderMaxDepth(1).SaveX(ctx)
	service := NewService(client, testSecretBox(t))

	created, err := service.createImportedNote(ctx, u.ID, account, RemoteNote{
		ExternalID: "doc-1",
		TargetID:   "box-1",
		TargetName: "Notebook",
		Path:       "/Notebook/Projects/Area/Imported",
		Title:      "Imported",
		Content:    "body",
		Tags:       []string{"idea", "", "idea", "archive"},
	}, true)
	if err != nil {
		t.Fatalf("create imported note: %v", err)
	}

	leaf := created.QueryFolder().OnlyX(ctx)
	if leaf.Name != "Area" {
		t.Fatalf("leaf folder = %q, want Area", leaf.Name)
	}
	parent := leaf.QueryParent().OnlyX(ctx)
	if parent.Name != "Projects" {
		t.Fatalf("parent folder = %q, want Projects", parent.Name)
	}
	root := parent.QueryParent().OnlyX(ctx)
	if root.Name != "Notebook" {
		t.Fatalf("root folder = %q, want Notebook", root.Name)
	}
	if root.QueryParent().ExistX(ctx) {
		t.Fatal("root folder should not have parent")
	}
	config := client.BackupConfig.Query().OnlyX(ctx)
	if config.FolderMaxDepth != 3 {
		t.Fatalf("folder max depth = %d, want 3", config.FolderMaxDepth)
	}
	if got := created.QueryTags().CountX(ctx); got != 2 {
		t.Fatalf("tag count = %d, want 2", got)
	}
}

func TestCreateImportedNoteCanIgnoreRemoteHierarchy(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestCreateImportedNoteCanIgnoreRemoteHierarchy?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	account := client.NoteConnectionAccount.Create().
		SetName("Joplin").
		SetProvider(ProviderJoplin).
		SetEndpoint("http://127.0.0.1:41184").
		SetEnabled(true).
		SetAuthType("token").
		SetDefaultTargetID("folder-1").
		SetDefaultTargetName("Inbox").
		SetUserID(u.ID).
		SaveX(ctx)
	client.BackupConfig.Create().SetFolderMaxDepth(1).SaveX(ctx)
	service := NewService(client, testSecretBox(t))

	created, err := service.createImportedNote(ctx, u.ID, account, RemoteNote{
		ExternalID: "note-1",
		TargetID:   "folder-1",
		Path:       "/Parent/Child/Imported",
		Title:      "Imported",
		Content:    "body",
	}, false)
	if err != nil {
		t.Fatalf("create imported note: %v", err)
	}

	leaf := created.QueryFolder().OnlyX(ctx)
	if leaf.Name != "Inbox" {
		t.Fatalf("folder = %q, want Inbox", leaf.Name)
	}
	if leaf.QueryParent().ExistX(ctx) {
		t.Fatal("flat import folder should not have parent")
	}
	if got := client.Folder.Query().CountX(ctx); got != 1 {
		t.Fatalf("folder count = %d, want 1", got)
	}
	config := client.BackupConfig.Query().OnlyX(ctx)
	if config.FolderMaxDepth != 1 {
		t.Fatalf("folder max depth = %d, want 1", config.FolderMaxDepth)
	}
}

func TestImportRemoteNoteRollsBackLocalCreatesWhenItemMapFails(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestImportRemoteNoteRollsBackLocalCreatesWhenItemMapFails?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	u := client.User.Create().
		SetUsername("owner").
		SetPasswordHash("hash").
		SaveX(ctx)
	account := client.NoteConnectionAccount.Create().
		SetName("SiYuan").
		SetProvider(ProviderSiYuan).
		SetEndpoint("http://127.0.0.1:6806").
		SetEnabled(true).
		SetAuthType("token").
		SetUserID(u.ID).
		SaveX(ctx)
	existingNote := client.Note.Create().
		SetTitle("Existing").
		SetContent("body").
		SetUserID(u.ID).
		SaveX(ctx)
	client.NoteConnectionItemMap.Create().
		SetProvider(ProviderSiYuan).
		SetAccountID(account.ID).
		SetNoteID(existingNote.ID).
		SetExternalID("dup-doc").
		ExecX(ctx)
	client.BackupConfig.Create().SetFolderMaxDepth(1).SaveX(ctx)
	service := NewService(client, testSecretBox(t))

	err := service.importRemoteNote(ctx, u.ID, account, RemoteNote{
		ExternalID: "dup-doc",
		TargetID:   "box-1",
		TargetName: "Notebook",
		Path:       "/Projects/Area/Imported",
		Title:      "Imported",
		Content:    "body",
		Tags:       []string{"rolled-back-tag"},
	}, true)
	if err == nil {
		t.Fatal("expected duplicate external id to fail item map creation")
	}
	if got := client.Note.Query().Where(note.TitleEQ("Imported")).CountX(ctx); got != 0 {
		t.Fatalf("imported note count after rollback = %d, want 0", got)
	}
	if got := client.Folder.Query().Where(folder.NameIn("Notebook", "Projects", "Area")).CountX(ctx); got != 0 {
		t.Fatalf("folder count after rollback = %d, want 0", got)
	}
	if got := client.Tag.Query().Where(tag.NameEQ("rolled-back-tag")).CountX(ctx); got != 0 {
		t.Fatalf("tag count after rollback = %d, want 0", got)
	}
	if got := client.BackupConfig.Query().Where(backupconfig.FolderMaxDepthEQ(1)).CountX(ctx); got != 1 {
		t.Fatalf("backup config depth was changed despite rollback, matching count = %d", got)
	}
	if got := client.NoteConnectionItemMap.Query().
		Where(noteconnectionitemmap.AccountIDEQ(account.ID)).
		CountX(ctx); got != 1 {
		t.Fatalf("item map count after rollback = %d, want existing map only", got)
	}
}

func TestRedactErrorRemovesTokenQueryValues(t *testing.T) {
	err := errors.New(`Get "http://127.0.0.1:41184/folders?fields=id&token=joplin-secret-token": dial tcp`)

	got := redactError(err)

	if strings.Contains(got, "joplin-secret-token") {
		t.Fatalf("redacted error leaked token: %q", got)
	}
	if !strings.Contains(got, "token=REDACTED") {
		t.Fatalf("redacted error = %q, want token placeholder", got)
	}
}

func testSecretBox(t *testing.T) *secrets.Box {
	t.Helper()
	box, err := secrets.OpenBox(storage.NewMemoryFileSystem())
	if err != nil {
		t.Fatalf("open secret box: %v", err)
	}
	return box
}

func stringPtr(value string) *string {
	return &value
}
