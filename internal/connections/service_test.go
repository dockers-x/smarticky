package connections

import (
	"context"
	"errors"
	"strings"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/ent/noteconnectionaccount"
	"smarticky/ent/noteconnectionitemmap"
	"smarticky/ent/noteconnectionjob"
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
