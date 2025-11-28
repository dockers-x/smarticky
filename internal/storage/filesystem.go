package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// FileSystem provides an abstraction over file operations using afero
type FileSystem struct {
	fs      afero.Fs
	baseDir string
}

// NewFileSystem creates a new FileSystem instance
// If baseDir is empty, it uses SMARTICKY_DATA_DIR environment variable or defaults to "data"
func NewFileSystem(baseDir string) *FileSystem {
	if baseDir == "" {
		baseDir = os.Getenv("SMARTICKY_DATA_DIR")
		if baseDir == "" {
			baseDir = "data"
		}
	}

	// Use OsFs for production
	fs := afero.NewOsFs()

	// Ensure base directory exists
	if err := fs.MkdirAll(baseDir, 0755); err != nil {
		// If we can't create the directory, fall back to memory fs for safety
		fs = afero.NewMemMapFs()
	}

	return &FileSystem{
		fs:      fs,
		baseDir: baseDir,
	}
}

// NewMemoryFileSystem creates a FileSystem backed by memory (useful for testing)
func NewMemoryFileSystem() *FileSystem {
	return &FileSystem{
		fs:      afero.NewMemMapFs(),
		baseDir: "data",
	}
}

// GetDataDir returns the base data directory path
func (f *FileSystem) GetDataDir() string {
	return f.baseDir
}

// GetUploadsDir returns the uploads directory path for a specific subdirectory
func (f *FileSystem) GetUploadsDir(subdir string) string {
	uploadsDir := filepath.Join(f.baseDir, "uploads", subdir)
	// Ensure directory exists
	if err := f.fs.MkdirAll(uploadsDir, 0755); err != nil {
		return filepath.Join("uploads", subdir)
	}
	return uploadsDir
}

// GetUploadsURL returns the URL path for an uploaded file
func (f *FileSystem) GetUploadsURL(subdir, filename string) string {
	return "/uploads/" + subdir + "/" + filename
}

// WriteFile writes data to a file
func (f *FileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := f.fs.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return afero.WriteFile(f.fs, path, data, perm)
}

// ReadFile reads data from a file
func (f *FileSystem) ReadFile(path string) ([]byte, error) {
	return afero.ReadFile(f.fs, path)
}

// Create creates a new file
func (f *FileSystem) Create(path string) (afero.File, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := f.fs.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	return f.fs.Create(path)
}

// Open opens a file for reading
func (f *FileSystem) Open(path string) (afero.File, error) {
	return f.fs.Open(path)
}

// Remove removes a file
func (f *FileSystem) Remove(path string) error {
	return f.fs.Remove(path)
}

// Rename renames (moves) a file
func (f *FileSystem) Rename(oldpath, newpath string) error {
	return f.fs.Rename(oldpath, newpath)
}

// Exists checks if a file or directory exists
func (f *FileSystem) Exists(path string) (bool, error) {
	return afero.Exists(f.fs, path)
}

// Stat returns file info
func (f *FileSystem) Stat(path string) (os.FileInfo, error) {
	return f.fs.Stat(path)
}

// MkdirAll creates a directory and all parent directories
func (f *FileSystem) MkdirAll(path string, perm os.FileMode) error {
	return f.fs.MkdirAll(path, perm)
}

// SaveUploadedFile saves an uploaded file from io.Reader to the filesystem
func (f *FileSystem) SaveUploadedFile(reader io.Reader, path string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := f.fs.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	dst, err := f.fs.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy data
	if _, err := io.Copy(dst, reader); err != nil {
		return fmt.Errorf("failed to save file: %w", err)
	}

	return nil
}

// GetFs returns the underlying afero.Fs for advanced operations
func (f *FileSystem) GetFs() afero.Fs {
	return f.fs
}
