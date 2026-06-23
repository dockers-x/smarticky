package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"smarticky/ent/enttest"
	"smarticky/internal/storage"

	_ "github.com/lib-x/entsqlite"
)

type tarTestEntry struct {
	name string
	body string
	dir  bool
}

func makeTestArchive(t *testing.T, entries ...tarTestEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	for _, entry := range entries {
		header := &tar.Header{
			Name: entry.name,
			Mode: 0644,
		}
		if entry.dir {
			header.Typeflag = tar.TypeDir
			header.Mode = 0755
		} else {
			header.Typeflag = tar.TypeReg
			header.Size = int64(len(entry.body))
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if !entry.dir {
			if _, err := tarWriter.Write([]byte(entry.body)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}

	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gzWriter.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}

func TestExtractBackupArchiveRestoresFiles(t *testing.T) {
	dataDir := t.TempDir()
	h := &Handler{fs: storage.NewFileSystem(dataDir)}

	archive := makeTestArchive(t,
		tarTestEntry{name: "smarticky.db", body: "db-data"},
		tarTestEntry{name: "uploads", dir: true},
		tarTestEntry{name: "uploads/attachments/a.txt", body: "attachment"},
	)

	if err := h.extractBackupArchive(archive); err != nil {
		t.Fatalf("extract backup archive: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dataDir, "uploads", "attachments", "a.txt"))
	if err != nil {
		t.Fatalf("read restored attachment: %v", err)
	}
	if string(got) != "attachment" {
		t.Fatalf("expected restored attachment body, got %q", string(got))
	}
}

func TestExtractBackupArchiveRejectsPathTraversal(t *testing.T) {
	parentDir := t.TempDir()
	dataDir := filepath.Join(parentDir, "data")
	h := &Handler{fs: storage.NewFileSystem(dataDir)}

	archive := makeTestArchive(t,
		tarTestEntry{name: "../escape.txt", body: "escaped"},
	)

	err := h.extractBackupArchive(archive)
	if err == nil {
		t.Fatal("expected path traversal archive to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid archive path") {
		t.Fatalf("expected invalid archive path error, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(parentDir, "escape.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("expected escaped file not to exist, stat error: %v", statErr)
	}
}

func TestExtractBackupArchiveRejectsUnexpectedDataFile(t *testing.T) {
	dataDir := t.TempDir()
	h := &Handler{fs: storage.NewFileSystem(dataDir)}

	archive := makeTestArchive(t,
		tarTestEntry{name: "smarticky.db", body: "db-data"},
		tarTestEntry{name: "uploads", dir: true},
		tarTestEntry{name: "search.bleve/index", body: "index-data"},
	)

	err := h.extractBackupArchive(archive)
	if err == nil {
		t.Fatal("expected unexpected data file archive to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid backup archive entry") {
		t.Fatalf("expected invalid backup archive entry error, got %v", err)
	}
}

func TestRemoveDatabaseSidecars(t *testing.T) {
	dataDir := t.TempDir()
	fs := storage.NewFileSystem(dataDir)
	h := &Handler{fs: fs}

	dbPath := filepath.Join(dataDir, "smarticky.db")
	if err := fs.WriteFile(dbPath, []byte("db"), 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := fs.WriteFile(dbPath+suffix, []byte("sidecar"), 0644); err != nil {
			t.Fatalf("write sidecar %s: %v", suffix, err)
		}
	}

	if err := h.removeDatabaseSidecars(); err != nil {
		t.Fatalf("remove database sidecars: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected db file to remain: %v", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(dbPath + suffix); !os.IsNotExist(err) {
			t.Fatalf("expected sidecar %s to be removed, stat error: %v", suffix, err)
		}
	}
}

func TestVerifyBackupDataRejectsPathTraversal(t *testing.T) {
	h := &Handler{fs: storage.NewMemoryFileSystem()}

	result := h.verifyBackupData(makeTestArchive(t,
		tarTestEntry{name: "smarticky.db", body: "db-data"},
		tarTestEntry{name: "../escape.txt", body: "escaped"},
	))

	if result.Valid {
		t.Fatal("expected traversal backup to be invalid")
	}
	if !strings.Contains(result.Error, "invalid archive path") {
		t.Fatalf("expected invalid archive path error, got %q", result.Error)
	}
}

func TestVerifyBackupDataRejectsUnexpectedDataFile(t *testing.T) {
	h := &Handler{fs: storage.NewMemoryFileSystem()}

	result := h.verifyBackupData(makeTestArchive(t,
		tarTestEntry{name: "smarticky.db", body: "db-data"},
		tarTestEntry{name: "uploads", dir: true},
		tarTestEntry{name: "search.bleve/index", body: "index-data"},
	))

	if result.Valid {
		t.Fatal("expected unexpected data file backup to be invalid")
	}
	if !strings.Contains(result.Error, "invalid backup archive entry") {
		t.Fatalf("expected invalid backup archive entry error, got %q", result.Error)
	}
}

func TestCreateBackupArchiveIncludesEmptyUploadsDirectory(t *testing.T) {
	dataDir := t.TempDir()
	fs := storage.NewFileSystem(dataDir)
	h := &Handler{fs: fs}

	if err := fs.WriteFile(filepath.Join(dataDir, "smarticky.db"), []byte("db-data"), 0644); err != nil {
		t.Fatalf("write db: %v", err)
	}

	archive, err := h.createBackupArchive()
	if err != nil {
		t.Fatalf("create backup archive: %v", err)
	}
	result := h.verifyBackupData(archive.Bytes())
	if !result.Valid {
		t.Fatalf("expected generated backup to be valid, got %q", result.Error)
	}
}

func TestIsBackupFilename(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{name: "smarticky_backup_20260620_163358.tar.gz", want: true},
		{name: "smarticky_auto_backup_20260620_163358.tar.gz", want: true},
		{name: "smarticky_pre_restore_backup_20260620_163358.tar.gz", want: false},
		{name: "notes.tar.gz", want: false},
	}

	for _, tt := range tests {
		if got := isBackupFilename(tt.name); got != tt.want {
			t.Fatalf("isBackupFilename(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestNextBackupRun(t *testing.T) {
	loc := time.FixedZone("CST", 8*60*60)
	tests := []struct {
		name     string
		now      time.Time
		data     backupScheduleData
		want     time.Time
		wantNext bool
		wantErr  bool
	}{
		{
			name:     "manual has no next run",
			now:      time.Date(2026, 6, 22, 1, 30, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "manual"},
			wantNext: false,
		},
		{
			name:     "disabled has no next run",
			now:      time.Date(2026, 6, 22, 1, 30, 0, 0, loc),
			data:     backupScheduleData{Enabled: false, Schedule: "daily"},
			wantNext: false,
		},
		{
			name:     "daily before 2am runs today",
			now:      time.Date(2026, 6, 22, 1, 30, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "daily"},
			want:     time.Date(2026, 6, 22, 2, 0, 0, 0, loc),
			wantNext: true,
		},
		{
			name:     "daily at 2am runs tomorrow",
			now:      time.Date(2026, 6, 22, 2, 0, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "daily"},
			want:     time.Date(2026, 6, 23, 2, 0, 0, 0, loc),
			wantNext: true,
		},
		{
			name:     "weekly runs next sunday",
			now:      time.Date(2026, 6, 22, 9, 0, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "weekly"},
			want:     time.Date(2026, 6, 28, 2, 0, 0, 0, loc),
			wantNext: true,
		},
		{
			name:     "monthly before first day 2am runs today",
			now:      time.Date(2026, 6, 1, 1, 0, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "monthly"},
			want:     time.Date(2026, 6, 1, 2, 0, 0, 0, loc),
			wantNext: true,
		},
		{
			name:     "monthly after first day 2am runs next month",
			now:      time.Date(2026, 6, 1, 3, 0, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "monthly"},
			want:     time.Date(2026, 7, 1, 2, 0, 0, 0, loc),
			wantNext: true,
		},
		{
			name:     "invalid schedule returns error",
			now:      time.Date(2026, 6, 22, 1, 30, 0, 0, loc),
			data:     backupScheduleData{Enabled: true, Schedule: "hourly"},
			wantNext: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := nextBackupRun(tt.now, 1, tt.data)
			if (err != nil) != tt.wantErr {
				t.Fatalf("nextBackupRun error = %v, wantErr %v", err, tt.wantErr)
			}
			if ok != tt.wantNext {
				t.Fatalf("nextBackupRun ok = %v, want %v", ok, tt.wantNext)
			}
			if ok && !got.Equal(tt.want) {
				t.Fatalf("nextBackupRun time = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCleanupTargetBackupsKeepsNewestAcrossManualAndAutomatic(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	client := &fakeBackupTargetClient{
		files: []BackupFileInfo{
			{
				Filename:  backupTaskFilename(7, true, now.Add(-3*time.Hour)),
				CreatedAt: now.Add(-3 * time.Hour),
			},
			{
				Filename:  backupTaskFilename(8, false, now.Add(-4*time.Hour)),
				CreatedAt: now.Add(-4 * time.Hour),
			},
			{
				Filename:  backupTaskFilename(7, false, now.Add(-2*time.Hour)),
				CreatedAt: now.Add(-2 * time.Hour),
			},
			{
				Filename:  backupTaskFilename(7, true, now.Add(-time.Hour)),
				CreatedAt: now.Add(-1 * time.Hour),
			},
		},
	}

	if err := cleanupTargetBackups(context.Background(), client, 0, 2, backupTaskFilenamePrefixes(7)...); err != nil {
		t.Fatalf("cleanup target backups: %v", err)
	}

	wantDeleted := backupTaskFilename(7, true, now.Add(-3*time.Hour))
	if len(client.deleted) != 1 || client.deleted[0] != wantDeleted {
		t.Fatalf("deleted = %#v, want [%q]", client.deleted, wantDeleted)
	}
}

type fakeBackupTargetClient struct {
	files   []BackupFileInfo
	deleted []string
}

func (c *fakeBackupTargetClient) List(context.Context) ([]BackupFileInfo, error) {
	return append([]BackupFileInfo(nil), c.files...), nil
}

func (c *fakeBackupTargetClient) Upload(context.Context, string, []byte) error {
	return nil
}

func (c *fakeBackupTargetClient) Download(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (c *fakeBackupTargetClient) Delete(_ context.Context, filename string) error {
	c.deleted = append(c.deleted, filename)
	return nil
}

func (c *fakeBackupTargetClient) Test(context.Context) error {
	return nil
}

func TestEnsureBackupTargetsMigratedCompletesPartialLegacyMigration(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestEnsureBackupTargetsMigratedCompletesPartialLegacyMigration?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()

	config := client.BackupConfig.Create().
		SetWebdavURL("https://dav.example.com/backups").
		SetWebdavUser("dav-user").
		SetWebdavPassword("dav-pass").
		SetS3Endpoint("https://s3.example.com").
		SetS3Region("us-east-1").
		SetS3Bucket("smarticky").
		SetS3AccessKey("access").
		SetS3SecretKey("secret").
		SetAutoBackupEnabled(true).
		SetBackupSchedule("weekly").
		SetBackupRetentionDays(14).
		SetBackupMaxCount(3).
		SaveX(ctx)

	client.BackupTarget.Create().
		SetName("WebDAV").
		SetType("webdav").
		SetEnabled(true).
		SetWebdavURL(config.WebdavURL).
		SetWebdavUser(config.WebdavUser).
		SetWebdavPassword(config.WebdavPassword).
		SaveX(ctx)

	h := NewHandler(client, storage.NewMemoryFileSystem())
	if err := h.ensureBackupTargetsMigrated(ctx); err != nil {
		t.Fatalf("migrate legacy backup config: %v", err)
	}

	if got := client.BackupTarget.Query().CountX(ctx); got != 2 {
		t.Fatalf("target count after migration = %d, want 2", got)
	}
	task := client.BackupTask.Query().WithTargets().OnlyX(ctx)
	if task.Schedule != "weekly" || !task.Enabled {
		t.Fatalf("task schedule/enabled = %q/%v, want weekly/true", task.Schedule, task.Enabled)
	}
	if task.RetentionDays != 14 || task.MaxCount != 3 {
		t.Fatalf("retention/max count = %d/%d, want 14/3", task.RetentionDays, task.MaxCount)
	}
	if len(task.Edges.Targets) != 2 {
		t.Fatalf("task target count = %d, want 2", len(task.Edges.Targets))
	}
	if !client.BackupConfig.GetX(ctx, config.ID).BackupTargetsMigrated {
		t.Fatal("expected backup config migration marker to be set")
	}

	if err := h.ensureBackupTargetsMigrated(ctx); err != nil {
		t.Fatalf("repeat migrate legacy backup config: %v", err)
	}
	if got := client.BackupTarget.Query().CountX(ctx); got != 2 {
		t.Fatalf("target count after repeat migration = %d, want 2", got)
	}
	if got := client.BackupTask.Query().CountX(ctx); got != 1 {
		t.Fatalf("task count after repeat migration = %d, want 1", got)
	}
}
