package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"smarticky/ent"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

// getDBPath returns the database file path
func (h *Handler) getDBPath() string {
	return filepath.Join(h.fs.GetDataDir(), "smarticky.db")
}

// createBackupArchive creates a tar.gz archive containing database and uploads
func (h *Handler) createBackupArchive() (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	gzWriter := gzip.NewWriter(buf)
	defer gzWriter.Close()

	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	dataDir := h.fs.GetDataDir()
	fs := h.fs.GetFs()

	// Helper function to add a file to tar archive
	addFile := func(path string, name string) error {
		fileInfo, err := h.fs.Stat(path)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}

		header, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			return fmt.Errorf("failed to create tar header: %w", err)
		}
		header.Name = name

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header: %w", err)
		}

		if fileInfo.IsDir() {
			return nil
		}

		file, err := h.fs.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %w", path, err)
		}
		defer file.Close()

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("failed to copy file data: %w", err)
		}

		return nil
	}

	// Add database file
	dbPath := h.getDBPath()
	if err := addFile(dbPath, "smarticky.db"); err != nil {
		return nil, err
	}

	// Add uploads directory recursively
	uploadsDir := filepath.Join(dataDir, "uploads")
	exists, _ := h.fs.Exists(uploadsDir)
	if exists {
		err := afero.Walk(fs, uploadsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Get relative path from data directory
			relPath, err := filepath.Rel(dataDir, path)
			if err != nil {
				return err
			}

			return addFile(path, relPath)
		})

		if err != nil {
			return nil, fmt.Errorf("failed to add uploads directory: %w", err)
		}
	}

	if err := tarWriter.Close(); err != nil {
		return nil, err
	}
	if err := gzWriter.Close(); err != nil {
		return nil, err
	}

	return buf, nil
}

// extractBackupArchive extracts a tar.gz archive to the data directory
func (h *Handler) extractBackupArchive(data []byte) error {
	buf := bytes.NewReader(data)
	gzReader, err := gzip.NewReader(buf)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)
	dataDir := h.fs.GetDataDir()

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(dataDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := h.fs.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			file, err := h.fs.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			file.Close()
		}
	}

	return nil
}

// GetBackupConfig retrieves or creates the backup configuration
func (h *Handler) GetBackupConfig(c echo.Context) error {
	ctx := context.Background()

	// Try to get existing config
	configs, err := h.client.BackupConfig.Query().All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// If no config exists, create default
	if len(configs) == 0 {
		config, err := h.client.BackupConfig.Create().Save(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, config)
	}

	return c.JSON(http.StatusOK, configs[0])
}

// UpdateBackupConfig updates the backup configuration
func (h *Handler) UpdateBackupConfig(c echo.Context) error {
	var req struct {
		WebDAVURL         *string `json:"webdav_url"`
		WebDAVUser        *string `json:"webdav_user"`
		WebDAVPassword    *string `json:"webdav_password"`
		S3Endpoint        *string `json:"s3_endpoint"`
		S3Region          *string `json:"s3_region"`
		S3Bucket          *string `json:"s3_bucket"`
		S3AccessKey       *string `json:"s3_access_key"`
		S3SecretKey       *string `json:"s3_secret_key"`
		AutoBackupEnabled *bool   `json:"auto_backup_enabled"`
		BackupSchedule    *string `json:"backup_schedule"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()

	// Get or create config
	configs, err := h.client.BackupConfig.Query().All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var configID int
	if len(configs) == 0 {
		config, err := h.client.BackupConfig.Create().Save(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		configID = config.ID
	} else {
		configID = configs[0].ID
	}

	// Update fields
	update := h.client.BackupConfig.UpdateOneID(configID)

	if req.WebDAVURL != nil {
		update.SetWebdavURL(*req.WebDAVURL)
	}
	if req.WebDAVUser != nil {
		update.SetWebdavUser(*req.WebDAVUser)
	}
	if req.WebDAVPassword != nil {
		update.SetWebdavPassword(*req.WebDAVPassword)
	}
	if req.S3Endpoint != nil {
		update.SetS3Endpoint(*req.S3Endpoint)
	}
	if req.S3Region != nil {
		update.SetS3Region(*req.S3Region)
	}
	if req.S3Bucket != nil {
		update.SetS3Bucket(*req.S3Bucket)
	}
	if req.S3AccessKey != nil {
		update.SetS3AccessKey(*req.S3AccessKey)
	}
	if req.S3SecretKey != nil {
		update.SetS3SecretKey(*req.S3SecretKey)
	}
	if req.AutoBackupEnabled != nil {
		update.SetAutoBackupEnabled(*req.AutoBackupEnabled)
	}
	if req.BackupSchedule != nil {
		update.SetBackupSchedule(*req.BackupSchedule)
	}

	config, err := update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, config)
}

// BackupWebDAV backs up the database to WebDAV
func (h *Handler) BackupWebDAV(c echo.Context) error {
	ctx := context.Background()

	// Get config from database
	config, err := h.client.BackupConfig.Query().First(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	url := config.WebdavURL
	user := config.WebdavUser
	password := config.WebdavPassword

	if url == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "WebDAV URL not configured"})
	}

	// Create backup archive
	archive, err := h.createBackupArchive()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create backup: %v", err)})
	}

	// Connect to WebDAV
	client := gowebdav.NewClient(url, user, password)

	filename := fmt.Sprintf("smarticky_backup_%s.tar.gz", time.Now().Format("20060102_150405"))

	if err := client.Write(filename, archive.Bytes(), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("webdav upload failed: %v", err)})
	}

	// Update last backup time
	h.client.BackupConfig.UpdateOneID(config.ID).
		SetLastBackupAt(time.Now()).
		SaveX(ctx)

	return c.JSON(http.StatusOK, map[string]string{"message": "backup successful", "file": filename})
}

// BackupS3 backs up the database to S3
func (h *Handler) BackupS3(c echo.Context) error {
	ctx := context.Background()

	// Get config from database
	config, err := h.client.BackupConfig.Query().First(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	endpoint := config.S3Endpoint
	region := config.S3Region
	bucket := config.S3Bucket
	accessKey := config.S3AccessKey
	secretKey := config.S3SecretKey

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "S3 configuration incomplete"})
	}

	// Create backup archive
	archive, err := h.createBackupArchive()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create backup: %v", err)})
	}

	// Connect to S3
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKey, secretKey, ""),
		Endpoint:         aws.String(endpoint),
		Region:           aws.String(region),
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create s3 session"})
	}

	svc := s3.New(sess)

	filename := fmt.Sprintf("smarticky_backup_%s.tar.gz", time.Now().Format("20060102_150405"))

	_, err = svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		Body:   bytes.NewReader(archive.Bytes()),
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("s3 upload failed: %v", err)})
	}

	// Update last backup time
	h.client.BackupConfig.UpdateOneID(config.ID).
		SetLastBackupAt(time.Now()).
		SaveX(ctx)

	return c.JSON(http.StatusOK, map[string]string{"message": "backup successful", "file": filename})
}

// RestoreWebDAV restores database from WebDAV
func (h *Handler) RestoreWebDAV(c echo.Context) error {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	config, err := h.client.BackupConfig.Query().First(ctx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}

	// Download from WebDAV
	client := gowebdav.NewClient(config.WebdavURL, config.WebdavUser, config.WebdavPassword)

	data, err := client.Read(req.Filename)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to download: %v", err)})
	}

	// Create backup of current data before restore
	backupArchive, err := h.createBackupArchive()
	if err == nil {
		// Save current backup
		backupFilename := fmt.Sprintf("smarticky_pre_restore_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
		backupPath := filepath.Join(h.fs.GetDataDir(), backupFilename)
		h.fs.WriteFile(backupPath, backupArchive.Bytes(), 0644)
	}

	// Extract archive to data directory
	if err := h.extractBackupArchive(data); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to extract backup: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "restore successful"})
}

// RestoreS3 restores database from S3
func (h *Handler) RestoreS3(c echo.Context) error {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	config, err := h.client.BackupConfig.Query().First(ctx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}

	// Connect to S3
	s3Config := &aws.Config{
		Credentials:      credentials.NewStaticCredentials(config.S3AccessKey, config.S3SecretKey, ""),
		Endpoint:         aws.String(config.S3Endpoint),
		Region:           aws.String(config.S3Region),
		S3ForcePathStyle: aws.Bool(true),
	}

	sess, err := session.NewSession(s3Config)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create s3 session"})
	}

	svc := s3.New(sess)

	// Download from S3
	result, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(config.S3Bucket),
		Key:    aws.String(req.Filename),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to download: %v", err)})
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read data"})
	}

	// Create backup of current data before restore
	backupArchive, err := h.createBackupArchive()
	if err == nil {
		// Save current backup
		backupFilename := fmt.Sprintf("smarticky_pre_restore_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
		backupPath := filepath.Join(h.fs.GetDataDir(), backupFilename)
		h.fs.WriteFile(backupPath, backupArchive.Bytes(), 0644)
	}

	// Extract archive to data directory
	if err := h.extractBackupArchive(data); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to extract backup: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "restore successful"})
}
