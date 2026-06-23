package handler

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/backuptask"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/lib-x/timewheel/scheduler"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

type backupScheduleData struct {
	TaskID   int
	Enabled  bool
	Schedule string
}

var errBackupSchedulerUnavailable = errors.New("backup scheduler is not running")

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

func (h *Handler) removeDatabaseSidecars() error {
	dbPath := h.getDBPath()
	var failures []string

	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		path := dbPath + suffix
		if err := h.fs.Remove(path); err != nil && !os.IsNotExist(err) {
			failures = append(failures, fmt.Sprintf("%s: %v", path, err))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("failed to remove database sidecar files: %s", strings.Join(failures, "; "))
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
	if !exists {
		if err := tarWriter.WriteHeader(&tar.Header{
			Name:     "uploads",
			Typeflag: tar.TypeDir,
			Mode:     0755,
		}); err != nil {
			return nil, fmt.Errorf("failed to add empty uploads directory: %w", err)
		}
	} else {
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

		if err := validateBackupArchiveName(header.Name); err != nil {
			return err
		}
		target, err := safeArchiveTarget(dataDir, header.Name)
		if err != nil {
			return err
		}

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

func safeArchiveTarget(dataDir, name string) (string, error) {
	cleanName := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if cleanName == "." || cleanName == ".." || path.IsAbs(cleanName) || strings.HasPrefix(cleanName, "../") {
		return "", fmt.Errorf("invalid archive path %q", name)
	}

	return filepath.Join(dataDir, filepath.FromSlash(cleanName)), nil
}

func validateBackupArchiveName(name string) error {
	if _, err := safeArchiveTarget("/", name); err != nil {
		return err
	}
	cleanName := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if cleanName == "smarticky.db" || cleanName == "uploads" || strings.HasPrefix(cleanName, "uploads/") {
		return nil
	}
	return fmt.Errorf("invalid backup archive entry %q", name)
}

func isBackupFilename(name string) bool {
	return strings.HasPrefix(name, "smarticky_backup_") ||
		strings.HasPrefix(name, "smarticky_auto_backup_")
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

	if err := h.checkpointWAL(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to prepare database for pre-restore backup: %v", err),
		})
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
	if err := h.removeDatabaseSidecars(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// IMPORTANT: Database connections need to be reestablished
	// The application should be restarted for the restored data to take full effect
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":          "restore successful",
		"warning":          "Please restart the application for changes to take full effect",
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

	if err := h.checkpointWAL(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": fmt.Sprintf("failed to prepare database for pre-restore backup: %v", err),
		})
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
	if err := h.removeDatabaseSidecars(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
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

// StartAutoBackup initializes and starts the automatic backup scheduler.
func (h *Handler) StartAutoBackup() *scheduler.Scheduler[int, backupScheduleData] {
	ctx := context.Background()
	if err := h.ensureBackupTargetsMigrated(ctx); err != nil {
		fmt.Printf("Failed to prepare auto backup scheduler: %v\n", err)
		return nil
	}

	s, err := scheduler.NewScheduler[int, backupScheduleData](
		scheduler.Options[int, backupScheduleData]{
			Next: nextBackupRun,
			Run:  h.runScheduledBackupTask,
			OnFinish: func(key int, _ backupScheduleData, err error) {
				if err != nil {
					fmt.Printf("Auto backup task %d failed: %v\n", key, err)
				}
			},
			OnInvalid: func(key int, _ backupScheduleData, err error) {
				fmt.Printf("Auto backup task %d has invalid schedule: %v\n", key, err)
			},
		},
		scheduler.WithWheel(time.Minute, 24*60),
		scheduler.WithReschedulePolicy(scheduler.RescheduleAfterFinish),
	)
	if err != nil {
		fmt.Printf("Failed to create auto backup scheduler: %v\n", err)
		return nil
	}

	tasks, err := h.client.BackupTask.Query().All(ctx)
	if err != nil {
		fmt.Printf("Failed to load auto backup tasks: %v\n", err)
		return nil
	}
	items := make([]scheduler.Item[int, backupScheduleData], 0, len(tasks))
	for _, task := range tasks {
		items = append(items, scheduler.Item[int, backupScheduleData]{
			Key:  task.ID,
			Data: backupScheduleDataFromTask(task),
		})
	}
	if err := s.ReplaceAll(items); err != nil {
		fmt.Printf("Failed to register auto backup tasks: %v\n", err)
		return nil
	}
	if err := s.Start(ctx); err != nil {
		fmt.Printf("Failed to start auto backup scheduler: %v\n", err)
		return nil
	}
	h.backupScheduler = s

	fmt.Println("Auto backup scheduler started")
	return s
}

func (h *Handler) runScheduledBackupTask(ctx context.Context, taskID int, _ backupScheduleData) error {
	task, err := h.client.BackupTask.Query().
		Where(backuptask.ID(taskID), backuptask.Enabled(true)).
		WithTargets().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if task.Schedule == "manual" {
		return nil
	}
	_, err = h.runBackupTask(ctx, task, true)
	return err
}

func nextBackupRun(now time.Time, _ int, data backupScheduleData) (time.Time, bool, error) {
	if !data.Enabled || data.Schedule == "manual" {
		return time.Time{}, false, nil
	}
	if !validBackupSchedule(data.Schedule) {
		return time.Time{}, false, fmt.Errorf("unknown backup schedule %q", data.Schedule)
	}
	return nextBackupScheduleTime(data.Schedule, now), true, nil
}

func nextBackupScheduleTime(schedule string, now time.Time) time.Time {
	switch schedule {
	case "weekly":
		return nextWeeklyBackupTime(now)
	case "monthly":
		return nextMonthlyBackupTime(now)
	default:
		return nextDailyBackupTime(now)
	}
}

func nextDailyBackupTime(now time.Time) time.Time {
	next := scheduledBackupTime(now)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func nextWeeklyBackupTime(now time.Time) time.Time {
	daysUntilSunday := (int(time.Sunday) - int(now.Weekday()) + 7) % 7
	next := scheduledBackupTime(now.AddDate(0, 0, daysUntilSunday))
	if !next.After(now) {
		next = next.AddDate(0, 0, 7)
	}
	return next
}

func nextMonthlyBackupTime(now time.Time) time.Time {
	next := time.Date(now.Year(), now.Month(), 1, 2, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 1, 0)
	}
	return next
}

func scheduledBackupTime(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), 2, 0, 0, 0, day.Location())
}

func backupScheduleDataFromTask(task *ent.BackupTask) backupScheduleData {
	return backupScheduleData{
		TaskID:   task.ID,
		Enabled:  task.Enabled,
		Schedule: task.Schedule,
	}
}

func (h *Handler) upsertBackupTaskSchedule(task *ent.BackupTask) error {
	if h.backupScheduler == nil {
		if task.Enabled && task.Schedule != "manual" {
			return errBackupSchedulerUnavailable
		}
		return nil
	}
	return h.backupScheduler.Upsert(scheduler.Item[int, backupScheduleData]{
		Key:  task.ID,
		Data: backupScheduleDataFromTask(task),
	})
}

func (h *Handler) removeBackupTaskSchedule(taskID int) error {
	if h.backupScheduler == nil {
		return nil
	}
	return h.backupScheduler.Remove(taskID)
}

func (h *Handler) backupTaskNextRunAt(task *ent.BackupTask) *time.Time {
	if h.backupScheduler == nil {
		return nil
	}
	if runtime, ok := h.backupScheduler.Snapshot()[task.ID]; ok && runtime.NextRunAt != nil {
		return runtime.NextRunAt
	}
	return nil
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
	backups := make([]BackupFileInfo, 0)
	for _, file := range files {
		// Only include files that match backup naming pattern
		name := file.Name()
		if isBackupFilename(name) {
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
	backups := make([]BackupFileInfo, 0)
	for _, obj := range result.Contents {
		name := *obj.Key
		// Only include files that match backup naming pattern
		if isBackupFilename(name) {
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
	Valid      bool              `json:"valid"`
	Error      string            `json:"error,omitempty"`
	FileChecks []FileCheckResult `json:"file_checks"`
	TotalSize  int64             `json:"total_size"`
	FileCount  int               `json:"file_count"`
	VerifiedAt time.Time         `json:"verified_at"`
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

		if err := validateBackupArchiveName(header.Name); err != nil {
			result.Error = err.Error()
			return result
		}
		target, err := safeArchiveTarget("/", header.Name)
		if err != nil {
			result.Error = err.Error()
			return result
		}

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
		if isBackupFilename(name) {
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
		if isBackupFilename(name) {
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
