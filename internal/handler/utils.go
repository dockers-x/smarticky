package handler

import (
	"os"
	"path/filepath"
)

// GetDataDir returns the data directory path from environment or default
func GetDataDir() string {
	dataDir := os.Getenv("SMARTICKY_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	return dataDir
}

// GetUploadsDir returns the uploads directory path
func GetUploadsDir(subdir string) string {
	dataDir := GetDataDir()
	uploadsDir := filepath.Join(dataDir, "uploads", subdir)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return filepath.Join("uploads", subdir) // Fallback to old behavior
	}

	return uploadsDir
}

// GetUploadsURL returns the URL path for uploaded files
func GetUploadsURL(subdir, filename string) string {
	return "/uploads/" + subdir + "/" + filename
}
