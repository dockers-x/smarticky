package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"smarticky/internal/storage"
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
