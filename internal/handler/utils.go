package handler

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
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

func bindStrictJSON(c echo.Context, dst any) error {
	decoder := json.NewDecoder(c.Request().Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}
