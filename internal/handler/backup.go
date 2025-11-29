package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"smarticky/ent"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/robfig/cron/v3"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

// getDBPath returns the database file path
func (h *Handler) getDBPath() string {
	return filepath.Join(h.fs.GetDataDir(), "smarticky.db")
}

// checkpointWAL performs a WAL checkpoint to ensure data consistency before backup
func (h *Handler) checkpointWAL() error {
	// Execute WAL checkpoint to flush all changes from WAL to the main database file
	// This ensures the backup contains all committed transactions
	dbPath := h.getDBPath()

	// Open a temporary database connection for executing PRAGMA
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database for checkpoint: %w", err)
	}
	defer db.Close()

	_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return fmt.Errorf("failed to checkpoint WAL: %w", err)
	}
	return nil
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
		WebDAVURL           *string `json:"webdav_url"`
		WebDAVUser          *string `json:"webdav_user"`
		WebDAVPassword      *string `json:"webdav_password"`
		S3Endpoint          *string `json:"s3_endpoint"`
		S3Region            *string `json:"s3_region"`
		S3Bucket            *string `json:"s3_bucket"`
		S3AccessKey         *string `json:"s3_access_key"`
		S3SecretKey         *string `json:"s3_secret_key"`
		AutoBackupEnabled   *bool   `json:"auto_backup_enabled"`
		BackupSchedule      *string `json:"backup_schedule"`
		BackupRetentionDays *int    `json:"backup_retention_days"`
		BackupMaxCount      *int    `json:"backup_max_count"`
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
	if req.BackupRetentionDays != nil {
		update.SetBackupRetentionDays(*req.BackupRetentionDays)
	}
	if req.BackupMaxCount != nil {
		update.SetBackupMaxCount(*req.BackupMaxCount)
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

	// Checkpoint WAL to ensure data consistency
	if err := h.checkpointWAL(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to prepare database for backup: %v", err),
		})
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

	// Cleanup old backups based on retention policy
	if err := h.cleanupWebDAVBackups(config); err != nil {
		// Log error but don't fail the backup
		fmt.Printf("Failed to cleanup old backups: %v\n", err)
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "backup successful", "file": filename})
}

// BackupS3 backs up the database to S3
func (h *Handler) BackupS3(c echo.Context) error {
	ctx := context.Background()

	// Get config from database
	backupConfig, err := h.client.BackupConfig.Query().First(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	endpoint := backupConfig.S3Endpoint
	region := backupConfig.S3Region
	bucket := backupConfig.S3Bucket
	accessKey := backupConfig.S3AccessKey
	secretKey := backupConfig.S3SecretKey

	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "S3 configuration incomplete"})
	}

	// Checkpoint WAL to ensure data consistency
	if err := h.checkpointWAL(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to prepare database for backup: %v", err),
		})
	}

	// Create backup archive
	archive, err := h.createBackupArchive()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create backup: %v", err)})
	}

	// Configure S3 client with custom endpoint
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create s3 config"})
	}

	// Create S3 client with custom endpoint resolver
	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	filename := fmt.Sprintf("smarticky_backup_%s.tar.gz", time.Now().Format("20060102_150405"))

	_, err = svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		Body:   bytes.NewReader(archive.Bytes()),
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("s3 upload failed: %v", err)})
	}

	// Update last backup time
	h.client.BackupConfig.UpdateOneID(backupConfig.ID).
		SetLastBackupAt(time.Now()).
		SaveX(ctx)

	// Cleanup old backups based on retention policy
	if err := h.cleanupS3Backups(ctx, backupConfig); err != nil {
		// Log error but don't fail the backup
		fmt.Printf("Failed to cleanup old backups: %v\n", err)
	}

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
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to create pre-restore backup: %v", err),
		})
	}

	// Save current backup
	backupFilename := fmt.Sprintf("smarticky_pre_restore_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(h.fs.GetDataDir(), backupFilename)
	if err := h.fs.WriteFile(backupPath, backupArchive.Bytes(), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to save pre-restore backup: %v", err),
		})
	}

	// Extract archive to data directory
	if err := h.extractBackupArchive(data); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to extract backup: %v", err)})
	}

	// IMPORTANT: Database connections need to be reestablished
	// The application should be restarted for the restored data to take full effect
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "restore successful",
		"warning": "Please restart the application for changes to take full effect",
		"restart_required": true,
	})
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
	backupConfig, err := h.client.BackupConfig.Query().First(ctx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}

	// Configure S3 client
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(backupConfig.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backupConfig.S3AccessKey, backupConfig.S3SecretKey, "")),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create s3 config"})
	}

	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(backupConfig.S3Endpoint)
		o.UsePathStyle = true
	})

	// Download from S3
	result, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(backupConfig.S3Bucket),
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
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to create pre-restore backup: %v", err),
		})
	}

	// Save current backup
	backupFilename := fmt.Sprintf("smarticky_pre_restore_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(h.fs.GetDataDir(), backupFilename)
	if err := h.fs.WriteFile(backupPath, backupArchive.Bytes(), 0644); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to save pre-restore backup: %v", err),
		})
	}

	// Extract archive to data directory
	if err := h.extractBackupArchive(data); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to extract backup: %v", err)})
	}

	// IMPORTANT: Database connections need to be reestablished
	// The application should be restarted for the restored data to take full effect
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":          "restore successful",
		"warning":          "Please restart the application for changes to take full effect",
		"restart_required": true,
	})
}

// performAutoBackup executes automatic backup based on configured backend
func (h *Handler) performAutoBackup() {
	ctx := context.Background()

	// Get backup configuration
	config, err := h.client.BackupConfig.Query().First(ctx)
	if err != nil || !config.AutoBackupEnabled {
		return // Silently skip if not configured or disabled
	}

	// Checkpoint WAL to ensure data consistency
	if err := h.checkpointWAL(); err != nil {
		fmt.Printf("Auto backup failed: WAL checkpoint error: %v\n", err)
		return
	}

	// Create backup archive
	archive, err := h.createBackupArchive()
	if err != nil {
		fmt.Printf("Auto backup failed: archive creation error: %v\n", err)
		return
	}

	filename := fmt.Sprintf("smarticky_auto_backup_%s.tar.gz", time.Now().Format("20060102_150405"))

	// Try WebDAV backup first if configured
	if config.WebdavURL != "" {
		client := gowebdav.NewClient(config.WebdavURL, config.WebdavUser, config.WebdavPassword)
		if err := client.Write(filename, archive.Bytes(), 0644); err == nil {
			h.client.BackupConfig.UpdateOneID(config.ID).
				SetLastBackupAt(time.Now()).
				SaveX(ctx)

			// Cleanup old backups
			if err := h.cleanupWebDAVBackups(config); err != nil {
				fmt.Printf("Failed to cleanup old WebDAV backups: %v\n", err)
			}

			fmt.Printf("Auto backup successful (WebDAV): %s\n", filename)
			return
		}
	}

	// Try S3 backup if WebDAV failed or not configured
	if config.S3Endpoint != "" && config.S3Bucket != "" {
		cfg, err := awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(config.S3Region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				config.S3AccessKey, config.S3SecretKey, "")),
		)
		if err == nil {
			svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
				o.BaseEndpoint = aws.String(config.S3Endpoint)
				o.UsePathStyle = true
			})

			_, err = svc.PutObject(ctx, &s3.PutObjectInput{
				Bucket: aws.String(config.S3Bucket),
				Key:    aws.String(filename),
				Body:   bytes.NewReader(archive.Bytes()),
			})

			if err == nil {
				h.client.BackupConfig.UpdateOneID(config.ID).
					SetLastBackupAt(time.Now()).
					SaveX(ctx)

				// Cleanup old backups
				if err := h.cleanupS3Backups(ctx, config); err != nil {
					fmt.Printf("Failed to cleanup old S3 backups: %v\n", err)
				}

				fmt.Printf("Auto backup successful (S3): %s\n", filename)
				return
			}
		}
	}

	fmt.Println("Auto backup failed: no valid backup backend configured")
}

// StartAutoBackup initializes and starts the automatic backup scheduler
func (h *Handler) StartAutoBackup() *cron.Cron {
	c := cron.New()

	// Run every day at 2 AM to check if backup is needed
	c.AddFunc("0 2 * * *", func() {
		ctx := context.Background()

		config, err := h.client.BackupConfig.Query().First(ctx)
		if err != nil || !config.AutoBackupEnabled {
			return
		}

		now := time.Now()

		// Determine if backup is needed based on schedule
		shouldBackup := false

		switch config.BackupSchedule {
		case "daily":
			// Always backup on daily schedule
			shouldBackup = true

		case "weekly":
			// Backup only on Sunday
			if now.Weekday() == time.Sunday {
				shouldBackup = true
			}

		case "manual":
			// Manual mode - skip auto backup
			shouldBackup = false
		}

		if shouldBackup {
			h.performAutoBackup()
		}
	})

	c.Start()
	fmt.Println("Auto backup scheduler started")
	return c
}

// BackupFileInfo represents information about a backup file
type BackupFileInfo struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// ListWebDAVBackups lists all backup files on WebDAV
func (h *Handler) ListWebDAVBackups(c echo.Context) error {
	ctx := context.Background()

	// Get config from database
	config, err := h.client.BackupConfig.Query().First(ctx)
	if ent.IsNotFound(err) || config.WebdavURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "WebDAV not configured"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Connect to WebDAV
	client := gowebdav.NewClient(config.WebdavURL, config.WebdavUser, config.WebdavPassword)

	// List files
	files, err := client.ReadDir("/")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to list files: %v", err),
		})
	}

	// Filter and format backup files
	var backups []BackupFileInfo
	for _, file := range files {
		// Only include files that match backup naming pattern
		name := file.Name()
		if (len(name) > 19 && name[:19] == "smarticky_backup_") ||
			(len(name) > 24 && name[:24] == "smarticky_auto_backup_") {
			backups = append(backups, BackupFileInfo{
				Filename:  name,
				Size:      file.Size(),
				CreatedAt: file.ModTime(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"backups": backups,
	})
}

// ListS3Backups lists all backup files on S3
func (h *Handler) ListS3Backups(c echo.Context) error {
	ctx := context.Background()

	// Get config from database
	backupConfig, err := h.client.BackupConfig.Query().First(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if backupConfig.S3Endpoint == "" || backupConfig.S3Bucket == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "S3 not configured"})
	}

	// Configure S3 client
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(backupConfig.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backupConfig.S3AccessKey, backupConfig.S3SecretKey, "")),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create s3 config"})
	}

	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(backupConfig.S3Endpoint)
		o.UsePathStyle = true
	})

	// List objects
	result, err := svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(backupConfig.S3Bucket),
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to list objects: %v", err),
		})
	}

	// Filter and format backup files
	var backups []BackupFileInfo
	for _, obj := range result.Contents {
		name := *obj.Key
		// Only include files that match backup naming pattern
		if (len(name) > 19 && name[:19] == "smarticky_backup_") ||
			(len(name) > 24 && name[:24] == "smarticky_auto_backup_") {
			backups = append(backups, BackupFileInfo{
				Filename:  name,
				Size:      *obj.Size,
				CreatedAt: *obj.LastModified,
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"backups": backups,
	})
}

// BackupVerificationResult represents the result of backup verification
type BackupVerificationResult struct {
	Valid       bool                 `json:"valid"`
	Error       string               `json:"error,omitempty"`
	FileChecks  []FileCheckResult    `json:"file_checks"`
	TotalSize   int64                `json:"total_size"`
	FileCount   int                  `json:"file_count"`
	VerifiedAt  time.Time            `json:"verified_at"`
}

// FileCheckResult represents the check result for a single file
type FileCheckResult struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Size   int64  `json:"size"`
	IsDir  bool   `json:"is_dir"`
	Error  string `json:"error,omitempty"`
}

// VerifyWebDAVBackup verifies a backup file from WebDAV without restoring it
func (h *Handler) VerifyWebDAVBackup(c echo.Context) error {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Filename == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "filename is required"})
	}

	ctx := context.Background()
	config, err := h.client.BackupConfig.Query().First(ctx)
	if err != nil || config.WebdavURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "WebDAV not configured"})
	}

	// Download from WebDAV
	client := gowebdav.NewClient(config.WebdavURL, config.WebdavUser, config.WebdavPassword)
	data, err := client.Read(req.Filename)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to download backup: %v", err),
		})
	}

	// Verify the backup
	result := h.verifyBackupData(data)
	return c.JSON(http.StatusOK, result)
}

// VerifyS3Backup verifies a backup file from S3 without restoring it
func (h *Handler) VerifyS3Backup(c echo.Context) error {
	var req struct {
		Filename string `json:"filename"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Filename == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "filename is required"})
	}

	ctx := context.Background()
	backupConfig, err := h.client.BackupConfig.Query().First(ctx)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "backup not configured"})
	}

	if backupConfig.S3Endpoint == "" || backupConfig.S3Bucket == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "S3 not configured"})
	}

	// Configure S3 client
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(backupConfig.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backupConfig.S3AccessKey, backupConfig.S3SecretKey, "")),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create s3 config"})
	}

	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(backupConfig.S3Endpoint)
		o.UsePathStyle = true
	})

	// Download from S3
	result, err := svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(backupConfig.S3Bucket),
		Key:    aws.String(req.Filename),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to download backup: %v", err),
		})
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read backup data"})
	}

	// Verify the backup
	verifyResult := h.verifyBackupData(data)
	return c.JSON(http.StatusOK, verifyResult)
}

// verifyBackupData verifies backup integrity by extracting to memory
func (h *Handler) verifyBackupData(data []byte) BackupVerificationResult {
	result := BackupVerificationResult{
		Valid:      false,
		VerifiedAt: time.Now(),
		FileChecks: []FileCheckResult{},
	}

	// Create an in-memory filesystem
	memFs := afero.NewMemMapFs()

	// Try to extract the backup to memory
	buf := bytes.NewReader(data)
	gzReader, err := gzip.NewReader(buf)
	if err != nil {
		result.Error = fmt.Sprintf("failed to decompress: %v", err)
		return result
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	totalSize := int64(0)
	fileCount := 0

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Error = fmt.Sprintf("failed to read tar: %v", err)
			return result
		}

		target := "/" + header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			if err := memFs.MkdirAll(target, 0755); err != nil {
				result.Error = fmt.Sprintf("failed to create directory: %v", err)
				return result
			}
			fileCount++

		case tar.TypeReg:
			// Create parent directories if needed
			if err := memFs.MkdirAll(filepath.Dir(target), 0755); err != nil {
				result.Error = fmt.Sprintf("failed to create parent directory: %v", err)
				return result
			}

			file, err := memFs.Create(target)
			if err != nil {
				result.Error = fmt.Sprintf("failed to create file in memory: %v", err)
				return result
			}

			written, err := io.Copy(file, tarReader)
			file.Close()

			if err != nil {
				result.Error = fmt.Sprintf("failed to write file to memory: %v", err)
				return result
			}

			totalSize += written
			fileCount++
		}
	}

	// Verify critical files
	criticalFiles := []string{
		"/smarticky.db",
		"/uploads",
	}

	for _, path := range criticalFiles {
		check := FileCheckResult{Path: path}

		stat, err := memFs.Stat(path)
		if err != nil {
			check.Exists = false
			check.Error = err.Error()
		} else {
			check.Exists = true
			check.Size = stat.Size()
			check.IsDir = stat.IsDir()

			// For database file, verify it's not empty
			if path == "/smarticky.db" && stat.Size() == 0 {
				check.Error = "database file is empty"
			}
		}

		result.FileChecks = append(result.FileChecks, check)
	}

	// Check if all critical files are valid
	allValid := true
	for _, check := range result.FileChecks {
		if !check.Exists || check.Error != "" {
			allValid = false
			break
		}
	}

	result.Valid = allValid
	result.TotalSize = totalSize
	result.FileCount = fileCount

	if !allValid && result.Error == "" {
		result.Error = "one or more critical files are missing or invalid"
	}

	return result
}

// cleanupWebDAVBackups removes old backup files from WebDAV based on retention policy
func (h *Handler) cleanupWebDAVBackups(config *ent.BackupConfig) error {
	if config.WebdavURL == "" {
		return nil
	}

	// Get retention settings
	retentionDays := config.BackupRetentionDays
	maxCount := config.BackupMaxCount

	// If both are 0, no cleanup needed
	if retentionDays == 0 && maxCount == 0 {
		return nil
	}

	// Connect to WebDAV
	client := gowebdav.NewClient(config.WebdavURL, config.WebdavUser, config.WebdavPassword)

	// List all files
	files, err := client.ReadDir("/")
	if err != nil {
		return fmt.Errorf("failed to list webdav files: %w", err)
	}

	// Filter backup files
	var backups []struct {
		Name    string
		ModTime time.Time
	}

	for _, file := range files {
		name := file.Name()
		if (len(name) > 19 && name[:19] == "smarticky_backup_") ||
			(len(name) > 24 && name[:24] == "smarticky_auto_backup_") {
			backups = append(backups, struct {
				Name    string
				ModTime time.Time
			}{
				Name:    name,
				ModTime: file.ModTime(),
			})
		}
	}

	if len(backups) == 0 {
		return nil
	}

	// Sort backups by modification time (newest first)
	// Using simple bubble sort since the list is typically small
	for i := 0; i < len(backups)-1; i++ {
		for j := 0; j < len(backups)-i-1; j++ {
			if backups[j].ModTime.Before(backups[j+1].ModTime) {
				backups[j], backups[j+1] = backups[j+1], backups[j]
			}
		}
	}

	now := time.Now()
	var filesToDelete []string

	for i, backup := range backups {
		shouldDelete := false

		// Check count limit (keep only the newest N files)
		if maxCount > 0 && i >= maxCount {
			shouldDelete = true
		}

		// Check age limit
		if retentionDays > 0 {
			age := now.Sub(backup.ModTime)
			if age > time.Duration(retentionDays)*24*time.Hour {
				shouldDelete = true
			}
		}

		if shouldDelete {
			filesToDelete = append(filesToDelete, backup.Name)
		}
	}

	// Delete old files
	for _, filename := range filesToDelete {
		if err := client.Remove(filename); err != nil {
			fmt.Printf("Failed to delete old backup %s: %v\n", filename, err)
		} else {
			fmt.Printf("Deleted old backup: %s\n", filename)
		}
	}

	return nil
}

// cleanupS3Backups removes old backup files from S3 based on retention policy
func (h *Handler) cleanupS3Backups(ctx context.Context, backupConfig *ent.BackupConfig) error {
	if backupConfig.S3Endpoint == "" || backupConfig.S3Bucket == "" {
		return nil
	}

	// Get retention settings
	retentionDays := backupConfig.BackupRetentionDays
	maxCount := backupConfig.BackupMaxCount

	// If both are 0, no cleanup needed
	if retentionDays == 0 && maxCount == 0 {
		return nil
	}

	// Configure S3 client
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(backupConfig.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backupConfig.S3AccessKey, backupConfig.S3SecretKey, "")),
	)
	if err != nil {
		return fmt.Errorf("failed to create s3 config: %w", err)
	}

	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(backupConfig.S3Endpoint)
		o.UsePathStyle = true
	})

	// List objects
	result, err := svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(backupConfig.S3Bucket),
	})
	if err != nil {
		return fmt.Errorf("failed to list s3 objects: %w", err)
	}

	// Filter backup files
	var backups []struct {
		Key     string
		ModTime time.Time
	}

	for _, obj := range result.Contents {
		name := *obj.Key
		if (len(name) > 19 && name[:19] == "smarticky_backup_") ||
			(len(name) > 24 && name[:24] == "smarticky_auto_backup_") {
			backups = append(backups, struct {
				Key     string
				ModTime time.Time
			}{
				Key:     name,
				ModTime: *obj.LastModified,
			})
		}
	}

	if len(backups) == 0 {
		return nil
	}

	// Sort backups by modification time (newest first)
	for i := 0; i < len(backups)-1; i++ {
		for j := 0; j < len(backups)-i-1; j++ {
			if backups[j].ModTime.Before(backups[j+1].ModTime) {
				backups[j], backups[j+1] = backups[j+1], backups[j]
			}
		}
	}

	now := time.Now()
	var keysToDelete []string

	for i, backup := range backups {
		shouldDelete := false

		// Check count limit (keep only the newest N files)
		if maxCount > 0 && i >= maxCount {
			shouldDelete = true
		}

		// Check age limit
		if retentionDays > 0 {
			age := now.Sub(backup.ModTime)
			if age > time.Duration(retentionDays)*24*time.Hour {
				shouldDelete = true
			}
		}

		if shouldDelete {
			keysToDelete = append(keysToDelete, backup.Key)
		}
	}

	// Delete old files
	for _, key := range keysToDelete {
		_, err := svc.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(backupConfig.S3Bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			fmt.Printf("Failed to delete old backup %s: %v\n", key, err)
		} else {
			fmt.Printf("Deleted old backup: %s\n", key)
		}
	}

	return nil
}
