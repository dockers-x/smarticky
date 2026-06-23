package handler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/backuptarget"
	"smarticky/ent/backuptask"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/labstack/echo/v4"
	"github.com/studio-b12/gowebdav"
)

const restoreConfirmation = "RESTORE"

type backupTargetPayload struct {
	Name           *string `json:"name"`
	Type           *string `json:"type"`
	Enabled        *bool   `json:"enabled"`
	WebDAVURL      *string `json:"webdav_url"`
	WebDAVUser     *string `json:"webdav_user"`
	WebDAVPassword *string `json:"webdav_password"`
	S3Endpoint     *string `json:"s3_endpoint"`
	S3Region       *string `json:"s3_region"`
	S3Bucket       *string `json:"s3_bucket"`
	S3AccessKey    *string `json:"s3_access_key"`
	S3SecretKey    *string `json:"s3_secret_key"`
}

type backupTargetInput struct {
	Name           string
	Type           string
	Enabled        bool
	WebDAVURL      string
	WebDAVUser     string
	WebDAVPassword string
	S3Endpoint     string
	S3Region       string
	S3Bucket       string
	S3AccessKey    string
	S3SecretKey    string
}

type BackupTargetResponse struct {
	ID                int        `json:"id"`
	Name              string     `json:"name"`
	Type              string     `json:"type"`
	Enabled           bool       `json:"enabled"`
	WebDAVURL         string     `json:"webdav_url,omitempty"`
	WebDAVUser        string     `json:"webdav_user,omitempty"`
	HasWebDAVPassword bool       `json:"has_webdav_password"`
	S3Endpoint        string     `json:"s3_endpoint,omitempty"`
	S3Region          string     `json:"s3_region,omitempty"`
	S3Bucket          string     `json:"s3_bucket,omitempty"`
	HasS3AccessKey    bool       `json:"has_s3_access_key"`
	HasS3SecretKey    bool       `json:"has_s3_secret_key"`
	LastBackupStatus  string     `json:"last_backup_status"`
	LastBackupError   string     `json:"last_backup_error,omitempty"`
	LastBackupAt      *time.Time `json:"last_backup_at,omitempty"`
	LastTestStatus    string     `json:"last_test_status"`
	LastTestError     string     `json:"last_test_error,omitempty"`
	LastTestAt        *time.Time `json:"last_test_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type backupTaskPayload struct {
	Name          *string `json:"name"`
	Enabled       *bool   `json:"enabled"`
	Schedule      *string `json:"schedule"`
	RetentionDays *int    `json:"retention_days"`
	MaxCount      *int    `json:"max_count"`
	TargetIDs     []int   `json:"target_ids"`
}

type backupTaskInput struct {
	Name          string
	Enabled       bool
	Schedule      string
	RetentionDays int
	MaxCount      int
	TargetIDs     []int
}

type BackupTaskResponse struct {
	ID               int                    `json:"id"`
	Name             string                 `json:"name"`
	Enabled          bool                   `json:"enabled"`
	Schedule         string                 `json:"schedule"`
	RetentionDays    int                    `json:"retention_days"`
	MaxCount         int                    `json:"max_count"`
	TargetIDs        []int                  `json:"target_ids"`
	Targets          []BackupTargetResponse `json:"targets"`
	LastBackupStatus string                 `json:"last_backup_status"`
	LastBackupError  string                 `json:"last_backup_error,omitempty"`
	LastBackupAt     *time.Time             `json:"last_backup_at,omitempty"`
	NextRunAt        *time.Time             `json:"next_run_at,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type BackupConnectionTestResponse struct {
	OK        bool      `json:"ok"`
	Message   string    `json:"message"`
	CheckedAt time.Time `json:"checked_at"`
}

type BackupTargetRunResult struct {
	TargetID int    `json:"target_id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

type BackupRunResponse struct {
	Message string                  `json:"message"`
	File    string                  `json:"file"`
	Results []BackupTargetRunResult `json:"results"`
}

type backupTargetClient interface {
	List(ctx context.Context) ([]BackupFileInfo, error)
	Upload(ctx context.Context, filename string, data []byte) error
	Download(ctx context.Context, filename string) ([]byte, error)
	Delete(ctx context.Context, filename string) error
	Test(ctx context.Context) error
}

func (h *Handler) ListBackupTargets(c echo.Context) error {
	ctx := c.Request().Context()
	if err := h.ensureBackupTargetsMigrated(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to prepare backup targets"})
	}

	targets, err := h.client.BackupTarget.Query().
		Order(ent.Asc(backuptarget.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list backup targets"})
	}

	rows := make([]BackupTargetResponse, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, backupTargetResponse(target))
	}
	return c.JSON(http.StatusOK, rows)
}

func (h *Handler) CreateBackupTarget(c echo.Context) error {
	var req backupTargetPayload
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
	}

	input := targetInputFromPayload(backupTargetInput{Enabled: true}, req)
	if err := validateBackupTargetInput(input, true); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	target, err := createBackupTargetBuilder(h.client.BackupTarget.Create(), input).Save(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create backup target"})
	}
	return c.JSON(http.StatusCreated, backupTargetResponse(target))
}

func (h *Handler) UpdateBackupTarget(c echo.Context) error {
	id, err := intParam(c, "id")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
	}

	ctx := c.Request().Context()
	target, err := h.client.BackupTarget.Get(ctx, id)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Backup target not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load backup target"})
	}

	var req backupTargetPayload
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
	}

	input := targetInputFromPayload(targetInputFromEnt(target), req)
	if err := validateBackupTargetInput(input, true); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	updated, err := updateBackupTargetBuilder(target.Update(), input).Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update backup target"})
	}
	return c.JSON(http.StatusOK, backupTargetResponse(updated))
}

func (h *Handler) DeleteBackupTarget(c echo.Context) error {
	id, err := intParam(c, "id")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
	}
	ctx := c.Request().Context()
	taskCount, err := h.client.BackupTask.Query().
		Where(backuptask.HasTargetsWith(backuptarget.ID(id))).
		Count(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to check backup target usage"})
	}
	if taskCount > 0 {
		return c.JSON(http.StatusConflict, map[string]string{"error": "Backup target is used by one or more backup tasks"})
	}
	if err := h.client.BackupTarget.DeleteOneID(id).Exec(ctx); ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Backup target not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete backup target"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) TestUnsavedBackupTarget(c echo.Context) error {
	var req backupTargetPayload
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
	}
	input := targetInputFromPayload(backupTargetInput{Enabled: true}, req)
	if err := validateBackupTargetInput(input, false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	client, err := newBackupTargetClient(input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, connectionTestResponse(client.Test(c.Request().Context())))
}

func (h *Handler) TestBackupTarget(c echo.Context) error {
	target, err := h.backupTargetFromParam(c)
	if err != nil {
		return backupTargetParamError(c, err)
	}

	input := targetInputFromEnt(target)
	if c.Request().ContentLength > 0 {
		var req backupTargetPayload
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
		}
		input = targetInputFromPayload(input, req)
	}
	if err := validateBackupTargetInput(input, false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	client, err := newBackupTargetClient(input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	testErr := client.Test(c.Request().Context())
	if updateErr := h.updateTargetTestStatus(c.Request().Context(), target.ID, testErr); updateErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update test status"})
	}
	return c.JSON(http.StatusOK, connectionTestResponse(testErr))
}

func (h *Handler) ListBackupTasks(c echo.Context) error {
	ctx := c.Request().Context()
	if err := h.ensureBackupTargetsMigrated(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to prepare backup tasks"})
	}
	tasks, err := h.client.BackupTask.Query().
		WithTargets().
		Order(ent.Asc(backuptask.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list backup tasks"})
	}
	rows := make([]BackupTaskResponse, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, h.backupTaskResponse(task))
	}
	return c.JSON(http.StatusOK, rows)
}

func (h *Handler) CreateBackupTask(c echo.Context) error {
	var req backupTaskPayload
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid backup task"})
	}
	input := taskInputFromPayload(backupTaskInput{
		Enabled:       true,
		Schedule:      "manual",
		RetentionDays: 30,
		MaxCount:      10,
	}, req)
	if err := h.validateBackupTaskInput(c.Request().Context(), input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.validateBackupTaskScheduler(input); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}

	task, err := h.client.BackupTask.Create().
		SetName(input.Name).
		SetEnabled(input.Enabled).
		SetSchedule(input.Schedule).
		SetRetentionDays(input.RetentionDays).
		SetMaxCount(input.MaxCount).
		AddTargetIDs(input.TargetIDs...).
		Save(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create backup task"})
	}
	task, err = h.client.BackupTask.Query().Where(backuptask.ID(task.ID)).WithTargets().Only(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load backup task"})
	}
	if err := h.upsertBackupTaskSchedule(task); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to schedule backup task"})
	}
	return c.JSON(http.StatusCreated, h.backupTaskResponse(task))
}

func (h *Handler) UpdateBackupTask(c echo.Context) error {
	id, err := intParam(c, "id")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid backup task"})
	}
	ctx := c.Request().Context()
	task, err := h.client.BackupTask.Query().Where(backuptask.ID(id)).WithTargets().Only(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Backup task not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load backup task"})
	}

	var req backupTaskPayload
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid backup task"})
	}

	input := taskInputFromPayload(taskInputFromEnt(task), req)
	if err := h.validateBackupTaskInput(ctx, input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.validateBackupTaskScheduler(input); err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}

	if err := task.Update().
		SetName(input.Name).
		SetEnabled(input.Enabled).
		SetSchedule(input.Schedule).
		SetRetentionDays(input.RetentionDays).
		SetMaxCount(input.MaxCount).
		ClearTargets().
		AddTargetIDs(input.TargetIDs...).
		Exec(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update backup task"})
	}

	updated, err := h.client.BackupTask.Query().Where(backuptask.ID(id)).WithTargets().Only(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load backup task"})
	}
	if err := h.upsertBackupTaskSchedule(updated); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to schedule backup task"})
	}
	return c.JSON(http.StatusOK, h.backupTaskResponse(updated))
}

func (h *Handler) DeleteBackupTask(c echo.Context) error {
	id, err := intParam(c, "id")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid backup task"})
	}
	if err := h.client.BackupTask.DeleteOneID(id).Exec(c.Request().Context()); ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Backup task not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete backup task"})
	}
	if err := h.removeBackupTaskSchedule(id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to unschedule backup task"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) RunBackupTask(c echo.Context) error {
	id, err := intParam(c, "id")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid backup task"})
	}

	task, err := h.client.BackupTask.Query().
		Where(backuptask.ID(id)).
		WithTargets().
		Only(c.Request().Context())
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Backup task not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to load backup task"})
	}

	result, runErr := h.runBackupTask(c.Request().Context(), task, false)
	if runErr != nil && len(result.Results) == 0 {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": runErr.Error()})
	}
	return c.JSON(http.StatusOK, result)
}

func (h *Handler) RunBackupTarget(c echo.Context) error {
	target, err := h.backupTargetFromParam(c)
	if err != nil {
		return backupTargetParamError(c, err)
	}
	input := targetInputFromEnt(target)
	if err := validateBackupTargetInput(input, false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if err := h.checkpointWAL(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to prepare database for backup"})
	}
	archive, err := h.createBackupArchive()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create backup"})
	}

	filename := fmt.Sprintf("smarticky_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	client, err := newBackupTargetClient(input)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	uploadErr := client.Upload(c.Request().Context(), filename, archive.Bytes())
	_ = h.updateTargetBackupStatus(c.Request().Context(), target.ID, uploadErr)
	if uploadErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Backup upload failed"})
	}
	return c.JSON(http.StatusOK, BackupRunResponse{
		Message: "backup successful",
		File:    filename,
		Results: []BackupTargetRunResult{{
			TargetID: target.ID,
			Name:     target.Name,
			Type:     target.Type,
			OK:       true,
		}},
	})
}

func (h *Handler) ListBackupTargetFiles(c echo.Context) error {
	target, err := h.backupTargetFromParam(c)
	if err != nil {
		return backupTargetParamError(c, err)
	}
	client, err := newBackupTargetClient(targetInputFromEnt(target))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	files, err := client.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list backup files"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"backups": files})
}

func (h *Handler) VerifyBackupTargetFile(c echo.Context) error {
	target, err := h.backupTargetFromParam(c)
	if err != nil {
		return backupTargetParamError(c, err)
	}

	var req struct {
		Filename string `json:"filename"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}
	if err := validateBackupFilename(req.Filename); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	client, err := newBackupTargetClient(targetInputFromEnt(target))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	data, err := client.Download(c.Request().Context(), req.Filename)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to download backup"})
	}
	return c.JSON(http.StatusOK, h.verifyBackupData(data))
}

func (h *Handler) RestoreBackupTargetFile(c echo.Context) error {
	target, err := h.backupTargetFromParam(c)
	if err != nil {
		return backupTargetParamError(c, err)
	}

	var req struct {
		Filename     string `json:"filename"`
		Confirmation string `json:"confirmation"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}
	if req.Confirmation != restoreConfirmation {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Restore confirmation is required"})
	}
	if err := validateBackupFilename(req.Filename); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	client, err := newBackupTargetClient(targetInputFromEnt(target))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	data, err := client.Download(c.Request().Context(), req.Filename)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to download backup"})
	}
	if verification := h.verifyBackupData(data); !verification.Valid {
		return c.JSON(http.StatusBadRequest, verification)
	}
	if err := h.restoreVerifiedBackupData(data); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":          "restore successful",
		"warning":          "Please restart the application for changes to take full effect",
		"restart_required": true,
	})
}

func (h *Handler) performScheduledBackups() {
	ctx := context.Background()
	if err := h.ensureBackupTargetsMigrated(ctx); err != nil {
		fmt.Printf("Auto backup failed: target migration error: %v\n", err)
		return
	}
	tasks, err := h.client.BackupTask.Query().
		Where(backuptask.Enabled(true)).
		WithTargets().
		All(ctx)
	if err != nil {
		fmt.Printf("Auto backup failed: task query error: %v\n", err)
		return
	}

	now := time.Now()
	for _, task := range tasks {
		if !backupScheduleDue(task.Schedule, now) {
			continue
		}
		if _, err := h.runBackupTask(ctx, task, true); err != nil {
			fmt.Printf("Auto backup task %d failed: %v\n", task.ID, err)
		}
	}
}

func (h *Handler) runBackupTask(ctx context.Context, task *ent.BackupTask, automatic bool) (BackupRunResponse, error) {
	targets := task.Edges.Targets
	if len(targets) == 0 {
		err := errors.New("backup task has no targets")
		_ = h.updateTaskBackupStatus(ctx, task.ID, err)
		return BackupRunResponse{Message: err.Error()}, err
	}
	if err := h.checkpointWAL(); err != nil {
		err = fmt.Errorf("failed to prepare database for backup")
		_ = h.updateTaskBackupStatus(ctx, task.ID, err)
		return BackupRunResponse{Message: err.Error()}, err
	}
	archive, err := h.createBackupArchive()
	if err != nil {
		err = fmt.Errorf("failed to create backup archive")
		_ = h.updateTaskBackupStatus(ctx, task.ID, err)
		return BackupRunResponse{Message: err.Error()}, err
	}

	now := time.Now()
	filename := backupTaskFilename(task.ID, automatic, now)
	cleanupPrefixes := backupTaskFilenamePrefixes(task.ID)
	response := BackupRunResponse{Message: "backup completed", File: filename}
	successes := 0
	for _, target := range targets {
		item := BackupTargetRunResult{
			TargetID: target.ID,
			Name:     target.Name,
			Type:     target.Type,
		}
		if !target.Enabled {
			item.Error = "target disabled"
			response.Results = append(response.Results, item)
			_ = h.updateTargetBackupStatus(ctx, target.ID, errors.New(item.Error))
			continue
		}
		client, err := newBackupTargetClient(targetInputFromEnt(target))
		if err == nil {
			err = client.Upload(ctx, filename, archive.Bytes())
		}
		if err != nil {
			item.Error = "backup upload failed"
			_ = h.updateTargetBackupStatus(ctx, target.ID, err)
			response.Results = append(response.Results, item)
			continue
		}

		item.OK = true
		successes++
		_ = h.updateTargetBackupStatus(ctx, target.ID, nil)
		if cleanupErr := cleanupTargetBackups(ctx, client, task.RetentionDays, task.MaxCount, cleanupPrefixes...); cleanupErr != nil {
			fmt.Printf("Failed to cleanup old backups for target %d: %v\n", target.ID, cleanupErr)
		}
		response.Results = append(response.Results, item)
	}

	var statusErr error
	if successes == 0 {
		statusErr = errors.New("backup failed for all targets")
		response.Message = statusErr.Error()
	} else if successes < len(targets) {
		response.Message = "backup completed with target failures"
	}
	_ = h.updateTaskBackupStatus(ctx, task.ID, statusErr)
	return response, statusErr
}

func (h *Handler) restoreVerifiedBackupData(data []byte) error {
	if err := h.checkpointWAL(); err != nil {
		return fmt.Errorf("failed to prepare database for pre-restore backup: %w", err)
	}
	backupArchive, err := h.createBackupArchive()
	if err != nil {
		return fmt.Errorf("failed to create pre-restore backup: %w", err)
	}
	backupFilename := fmt.Sprintf("smarticky_pre_restore_backup_%s.tar.gz", time.Now().Format("20060102_150405"))
	if err := h.fs.WriteFile(filepath.Join(h.fs.GetDataDir(), backupFilename), backupArchive.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to save pre-restore backup: %w", err)
	}
	if err := h.extractBackupArchive(data); err != nil {
		return fmt.Errorf("failed to extract backup: %w", err)
	}
	if err := h.removeDatabaseSidecars(); err != nil {
		return err
	}
	return nil
}

func (h *Handler) ensureBackupTargetsMigrated(ctx context.Context) error {
	configs, err := h.client.BackupConfig.Query().All(ctx)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}
	config := configs[0]
	if config.BackupTargetsMigrated {
		return nil
	}
	desiredTargets := legacyBackupTargetInputs(config)
	if len(desiredTargets) == 0 {
		return h.client.BackupConfig.UpdateOneID(config.ID).
			SetBackupTargetsMigrated(true).
			Exec(ctx)
	}

	tx, err := h.client.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	txClient := tx.Client()
	existingTargets, err := txClient.BackupTarget.Query().All(ctx)
	if err != nil {
		return err
	}

	targetIDs := make([]int, 0, len(desiredTargets))
	for _, input := range desiredTargets {
		target := findLegacyBackupTarget(existingTargets, input)
		if target == nil {
			created, err := createBackupTargetBuilder(txClient.BackupTarget.Create(), input).Save(ctx)
			if err != nil {
				return err
			}
			target = created
			existingTargets = append(existingTargets, created)
		}
		targetIDs = append(targetIDs, target.ID)
	}

	taskCount, err := txClient.BackupTask.Query().Count(ctx)
	if err != nil {
		return err
	}
	if taskCount == 0 {
		schedule := normalizeBackupSchedule(config.BackupSchedule)
		enabled := config.AutoBackupEnabled
		if !enabled {
			schedule = "manual"
		}
		if _, err := txClient.BackupTask.Create().
			SetName("Default backup").
			SetEnabled(enabled).
			SetSchedule(schedule).
			SetRetentionDays(config.BackupRetentionDays).
			SetMaxCount(config.BackupMaxCount).
			AddTargetIDs(targetIDs...).
			Save(ctx); err != nil {
			return err
		}
	}
	if err := txClient.BackupConfig.UpdateOneID(config.ID).
		SetBackupTargetsMigrated(true).
		Exec(ctx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func legacyBackupTargetInputs(config *ent.BackupConfig) []backupTargetInput {
	inputs := make([]backupTargetInput, 0, 2)
	if strings.TrimSpace(config.WebdavURL) != "" {
		inputs = append(inputs, backupTargetInput{
			Name:           "WebDAV",
			Type:           "webdav",
			Enabled:        true,
			WebDAVURL:      config.WebdavURL,
			WebDAVUser:     config.WebdavUser,
			WebDAVPassword: config.WebdavPassword,
		})
	}
	if strings.TrimSpace(config.S3Endpoint) != "" && strings.TrimSpace(config.S3Bucket) != "" {
		inputs = append(inputs, backupTargetInput{
			Name:        "S3",
			Type:        "s3",
			Enabled:     true,
			S3Endpoint:  config.S3Endpoint,
			S3Region:    config.S3Region,
			S3Bucket:    config.S3Bucket,
			S3AccessKey: config.S3AccessKey,
			S3SecretKey: config.S3SecretKey,
		})
	}
	return inputs
}

func findLegacyBackupTarget(targets []*ent.BackupTarget, input backupTargetInput) *ent.BackupTarget {
	for _, target := range targets {
		if target.Type != input.Type {
			continue
		}
		switch input.Type {
		case "webdav":
			if strings.TrimSpace(target.WebdavURL) == strings.TrimSpace(input.WebDAVURL) {
				return target
			}
		case "s3":
			if strings.TrimSpace(target.S3Endpoint) == strings.TrimSpace(input.S3Endpoint) &&
				strings.TrimSpace(target.S3Bucket) == strings.TrimSpace(input.S3Bucket) {
				return target
			}
		}
	}
	return nil
}

func (h *Handler) backupTargetFromParam(c echo.Context) (*ent.BackupTarget, error) {
	id, err := intParam(c, "id")
	if err != nil {
		return nil, err
	}
	target, err := h.client.BackupTarget.Get(c.Request().Context(), id)
	if err != nil {
		return nil, err
	}
	return target, nil
}

func backupTargetParamError(c echo.Context, err error) error {
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Backup target not found"})
	}
	return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid target"})
}

func targetInputFromPayload(base backupTargetInput, req backupTargetPayload) backupTargetInput {
	if req.Name != nil {
		base.Name = strings.TrimSpace(*req.Name)
	}
	if req.Type != nil {
		base.Type = strings.ToLower(strings.TrimSpace(*req.Type))
	}
	if req.Enabled != nil {
		base.Enabled = *req.Enabled
	}
	if req.WebDAVURL != nil {
		base.WebDAVURL = strings.TrimSpace(*req.WebDAVURL)
	}
	if req.WebDAVUser != nil {
		base.WebDAVUser = strings.TrimSpace(*req.WebDAVUser)
	}
	if req.WebDAVPassword != nil {
		base.WebDAVPassword = *req.WebDAVPassword
	}
	if req.S3Endpoint != nil {
		base.S3Endpoint = strings.TrimSpace(*req.S3Endpoint)
	}
	if req.S3Region != nil {
		base.S3Region = strings.TrimSpace(*req.S3Region)
	}
	if req.S3Bucket != nil {
		base.S3Bucket = strings.TrimSpace(*req.S3Bucket)
	}
	if req.S3AccessKey != nil {
		base.S3AccessKey = strings.TrimSpace(*req.S3AccessKey)
	}
	if req.S3SecretKey != nil {
		base.S3SecretKey = *req.S3SecretKey
	}
	return base
}

func targetInputFromEnt(target *ent.BackupTarget) backupTargetInput {
	return backupTargetInput{
		Name:           target.Name,
		Type:           target.Type,
		Enabled:        target.Enabled,
		WebDAVURL:      target.WebdavURL,
		WebDAVUser:     target.WebdavUser,
		WebDAVPassword: target.WebdavPassword,
		S3Endpoint:     target.S3Endpoint,
		S3Region:       target.S3Region,
		S3Bucket:       target.S3Bucket,
		S3AccessKey:    target.S3AccessKey,
		S3SecretKey:    target.S3SecretKey,
	}
}

func validateBackupTargetInput(input backupTargetInput, requireName bool) error {
	if requireName && strings.TrimSpace(input.Name) == "" {
		return errors.New("target name is required")
	}
	switch input.Type {
	case "webdav":
		if strings.TrimSpace(input.WebDAVURL) == "" {
			return errors.New("WebDAV URL is required")
		}
	case "s3":
		if strings.TrimSpace(input.S3Endpoint) == "" ||
			strings.TrimSpace(input.S3Bucket) == "" ||
			strings.TrimSpace(input.S3AccessKey) == "" ||
			strings.TrimSpace(input.S3SecretKey) == "" {
			return errors.New("S3 configuration is incomplete")
		}
	default:
		return errors.New("target type must be webdav or s3")
	}
	return nil
}

func createBackupTargetBuilder(builder *ent.BackupTargetCreate, input backupTargetInput) *ent.BackupTargetCreate {
	return builder.
		SetName(input.Name).
		SetType(input.Type).
		SetEnabled(input.Enabled).
		SetWebdavURL(input.WebDAVURL).
		SetWebdavUser(input.WebDAVUser).
		SetWebdavPassword(input.WebDAVPassword).
		SetS3Endpoint(input.S3Endpoint).
		SetS3Region(input.S3Region).
		SetS3Bucket(input.S3Bucket).
		SetS3AccessKey(input.S3AccessKey).
		SetS3SecretKey(input.S3SecretKey)
}

func updateBackupTargetBuilder(builder *ent.BackupTargetUpdateOne, input backupTargetInput) *ent.BackupTargetUpdateOne {
	return builder.
		SetName(input.Name).
		SetType(input.Type).
		SetEnabled(input.Enabled).
		SetWebdavURL(input.WebDAVURL).
		SetWebdavUser(input.WebDAVUser).
		SetWebdavPassword(input.WebDAVPassword).
		SetS3Endpoint(input.S3Endpoint).
		SetS3Region(input.S3Region).
		SetS3Bucket(input.S3Bucket).
		SetS3AccessKey(input.S3AccessKey).
		SetS3SecretKey(input.S3SecretKey)
}

func backupTargetResponse(target *ent.BackupTarget) BackupTargetResponse {
	return BackupTargetResponse{
		ID:                target.ID,
		Name:              target.Name,
		Type:              target.Type,
		Enabled:           target.Enabled,
		WebDAVURL:         target.WebdavURL,
		WebDAVUser:        target.WebdavUser,
		HasWebDAVPassword: target.WebdavPassword != "",
		S3Endpoint:        target.S3Endpoint,
		S3Region:          target.S3Region,
		S3Bucket:          target.S3Bucket,
		HasS3AccessKey:    target.S3AccessKey != "",
		HasS3SecretKey:    target.S3SecretKey != "",
		LastBackupStatus:  target.LastBackupStatus,
		LastBackupError:   target.LastBackupError,
		LastBackupAt:      optionalTime(target.LastBackupAt),
		LastTestStatus:    target.LastTestStatus,
		LastTestError:     target.LastTestError,
		LastTestAt:        optionalTime(target.LastTestAt),
		CreatedAt:         target.CreatedAt,
		UpdatedAt:         target.UpdatedAt,
	}
}

func taskInputFromPayload(base backupTaskInput, req backupTaskPayload) backupTaskInput {
	if req.Name != nil {
		base.Name = strings.TrimSpace(*req.Name)
	}
	if req.Enabled != nil {
		base.Enabled = *req.Enabled
	}
	if req.Schedule != nil {
		base.Schedule = normalizeBackupSchedule(*req.Schedule)
	}
	if req.RetentionDays != nil {
		base.RetentionDays = *req.RetentionDays
	}
	if req.MaxCount != nil {
		base.MaxCount = *req.MaxCount
	}
	if req.TargetIDs != nil {
		base.TargetIDs = uniquePositiveInts(req.TargetIDs)
	}
	return base
}

func taskInputFromEnt(task *ent.BackupTask) backupTaskInput {
	ids := make([]int, 0, len(task.Edges.Targets))
	for _, target := range task.Edges.Targets {
		ids = append(ids, target.ID)
	}
	return backupTaskInput{
		Name:          task.Name,
		Enabled:       task.Enabled,
		Schedule:      task.Schedule,
		RetentionDays: task.RetentionDays,
		MaxCount:      task.MaxCount,
		TargetIDs:     ids,
	}
}

func (h *Handler) validateBackupTaskInput(ctx context.Context, input backupTaskInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return errors.New("task name is required")
	}
	if !validBackupSchedule(input.Schedule) {
		return errors.New("schedule must be manual, daily, weekly, or monthly")
	}
	if input.RetentionDays < 0 || input.MaxCount < 0 {
		return errors.New("retention values must be non-negative")
	}
	if len(input.TargetIDs) == 0 {
		return errors.New("select at least one backup target")
	}
	count, err := h.client.BackupTarget.Query().Where(backuptarget.IDIn(input.TargetIDs...)).Count(ctx)
	if err != nil {
		return err
	}
	if count != len(input.TargetIDs) {
		return errors.New("one or more backup targets do not exist")
	}
	return nil
}

func (h *Handler) validateBackupTaskScheduler(input backupTaskInput) error {
	if input.Enabled && input.Schedule != "manual" && h.backupScheduler == nil {
		return errBackupSchedulerUnavailable
	}
	return nil
}

func (h *Handler) backupTaskResponse(task *ent.BackupTask) BackupTaskResponse {
	targetIDs := make([]int, 0, len(task.Edges.Targets))
	targets := make([]BackupTargetResponse, 0, len(task.Edges.Targets))
	for _, target := range task.Edges.Targets {
		targetIDs = append(targetIDs, target.ID)
		targets = append(targets, backupTargetResponse(target))
	}
	sort.Ints(targetIDs)
	return BackupTaskResponse{
		ID:               task.ID,
		Name:             task.Name,
		Enabled:          task.Enabled,
		Schedule:         task.Schedule,
		RetentionDays:    task.RetentionDays,
		MaxCount:         task.MaxCount,
		TargetIDs:        targetIDs,
		Targets:          targets,
		LastBackupStatus: task.LastBackupStatus,
		LastBackupError:  task.LastBackupError,
		LastBackupAt:     optionalTime(task.LastBackupAt),
		NextRunAt:        h.backupTaskNextRunAt(task),
		CreatedAt:        task.CreatedAt,
		UpdatedAt:        task.UpdatedAt,
	}
}

func optionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func intParam(c echo.Context, name string) (int, error) {
	id, err := strconv.Atoi(c.Param(name))
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func normalizeBackupSchedule(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if validBackupSchedule(value) {
		return value
	}
	return "manual"
}

func validBackupSchedule(value string) bool {
	switch value {
	case "manual", "daily", "weekly", "monthly":
		return true
	default:
		return false
	}
}

func backupScheduleDue(schedule string, now time.Time) bool {
	switch schedule {
	case "daily":
		return true
	case "weekly":
		return now.Weekday() == time.Sunday
	case "monthly":
		return now.Day() == 1
	default:
		return false
	}
}

func backupTaskFilename(taskID int, automatic bool, now time.Time) string {
	return fmt.Sprintf("%s%s.tar.gz", backupTaskFilenamePrefix(taskID, automatic), now.Format("20060102_150405"))
}

func backupTaskFilenamePrefix(taskID int, automatic bool) string {
	if automatic {
		return fmt.Sprintf("smarticky_auto_backup_task_%d_", taskID)
	}
	return fmt.Sprintf("smarticky_backup_task_%d_", taskID)
}

func backupTaskFilenamePrefixes(taskID int) []string {
	return []string{
		backupTaskFilenamePrefix(taskID, false),
		backupTaskFilenamePrefix(taskID, true),
	}
}

func uniquePositiveInts(values []int) []int {
	seen := make(map[int]bool, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func validateBackupFilename(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("filename is required")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || path.Clean(name) != name {
		return errors.New("invalid backup filename")
	}
	if !isBackupFilename(name) {
		return errors.New("invalid backup filename")
	}
	return nil
}

func connectionTestResponse(err error) BackupConnectionTestResponse {
	if err != nil {
		return BackupConnectionTestResponse{
			OK:        false,
			Message:   "connection test failed",
			CheckedAt: time.Now(),
		}
	}
	return BackupConnectionTestResponse{
		OK:        true,
		Message:   "connection test successful",
		CheckedAt: time.Now(),
	}
}

func (h *Handler) updateTargetTestStatus(ctx context.Context, targetID int, testErr error) error {
	update := h.client.BackupTarget.UpdateOneID(targetID).
		SetLastTestAt(time.Now())
	if testErr != nil {
		return update.SetLastTestStatus("failed").
			SetLastTestError("connection test failed").
			Exec(ctx)
	}
	return update.SetLastTestStatus("success").
		ClearLastTestError().
		Exec(ctx)
}

func (h *Handler) updateTargetBackupStatus(ctx context.Context, targetID int, backupErr error) error {
	update := h.client.BackupTarget.UpdateOneID(targetID).
		SetLastBackupAt(time.Now())
	if backupErr != nil {
		return update.SetLastBackupStatus("failed").
			SetLastBackupError("backup failed").
			Exec(ctx)
	}
	return update.SetLastBackupStatus("success").
		ClearLastBackupError().
		Exec(ctx)
}

func (h *Handler) updateTaskBackupStatus(ctx context.Context, taskID int, backupErr error) error {
	update := h.client.BackupTask.UpdateOneID(taskID).
		SetLastBackupAt(time.Now())
	if backupErr != nil {
		return update.SetLastBackupStatus("failed").
			SetLastBackupError(backupErr.Error()).
			Exec(ctx)
	}
	return update.SetLastBackupStatus("success").
		ClearLastBackupError().
		Exec(ctx)
}

func newBackupTargetClient(input backupTargetInput) (backupTargetClient, error) {
	switch input.Type {
	case "webdav":
		return &webdavBackupTargetClient{
			client: gowebdav.NewClient(input.WebDAVURL, input.WebDAVUser, input.WebDAVPassword),
		}, nil
	case "s3":
		return newS3BackupTargetClient(context.Background(), input)
	default:
		return nil, errors.New("unsupported backup target type")
	}
}

type webdavBackupTargetClient struct {
	client *gowebdav.Client
}

func (c *webdavBackupTargetClient) List(ctx context.Context) ([]BackupFileInfo, error) {
	files, err := c.client.ReadDir("/")
	if err != nil {
		return nil, err
	}
	backups := make([]BackupFileInfo, 0)
	for _, file := range files {
		if isBackupFilename(file.Name()) {
			backups = append(backups, BackupFileInfo{
				Filename:  file.Name(),
				Size:      file.Size(),
				CreatedAt: file.ModTime(),
			})
		}
	}
	sortBackupFiles(backups)
	return backups, nil
}

func (c *webdavBackupTargetClient) Upload(ctx context.Context, filename string, data []byte) error {
	return c.client.Write(filename, data, 0644)
}

func (c *webdavBackupTargetClient) Download(ctx context.Context, filename string) ([]byte, error) {
	return c.client.Read(filename)
}

func (c *webdavBackupTargetClient) Delete(ctx context.Context, filename string) error {
	return c.client.Remove(filename)
}

func (c *webdavBackupTargetClient) Test(ctx context.Context) error {
	probe := fmt.Sprintf(".smarticky_connection_test_%d.txt", time.Now().UnixNano())
	if _, err := c.client.ReadDir("/"); err != nil {
		return err
	}
	if err := c.client.Write(probe, []byte("smarticky"), 0644); err != nil {
		return err
	}
	data, readErr := c.client.Read(probe)
	removeErr := c.client.Remove(probe)
	if readErr != nil {
		return readErr
	}
	if removeErr != nil {
		return removeErr
	}
	if !bytes.Equal(data, []byte("smarticky")) {
		return errors.New("probe read mismatch")
	}
	return nil
}

type s3BackupTargetClient struct {
	svc    *s3.Client
	bucket string
}

func newS3BackupTargetClient(ctx context.Context, input backupTargetInput) (*s3BackupTargetClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(input.S3Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(input.S3AccessKey, input.S3SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	svc := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(input.S3Endpoint)
		o.UsePathStyle = true
	})
	return &s3BackupTargetClient{svc: svc, bucket: input.S3Bucket}, nil
}

func (c *s3BackupTargetClient) List(ctx context.Context) ([]BackupFileInfo, error) {
	paginator := s3.NewListObjectsV2Paginator(c.svc, &s3.ListObjectsV2Input{
		Bucket: aws.String(c.bucket),
	})
	backups := make([]BackupFileInfo, 0)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			if obj.Key == nil || obj.LastModified == nil || obj.Size == nil {
				continue
			}
			name := *obj.Key
			if isBackupFilename(name) {
				backups = append(backups, BackupFileInfo{
					Filename:  name,
					Size:      *obj.Size,
					CreatedAt: *obj.LastModified,
				})
			}
		}
	}
	sortBackupFiles(backups)
	return backups, nil
}

func (c *s3BackupTargetClient) Upload(ctx context.Context, filename string, data []byte) error {
	_, err := c.svc.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(filename),
		Body:   bytes.NewReader(data),
	})
	return err
}

func (c *s3BackupTargetClient) Download(ctx context.Context, filename string) ([]byte, error) {
	result, err := c.svc.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(filename),
	})
	if err != nil {
		return nil, err
	}
	defer result.Body.Close()
	return io.ReadAll(result.Body)
}

func (c *s3BackupTargetClient) Delete(ctx context.Context, filename string) error {
	_, err := c.svc.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(filename),
	})
	return err
}

func (c *s3BackupTargetClient) Test(ctx context.Context) error {
	probe := fmt.Sprintf(".smarticky_connection_test_%d.txt", time.Now().UnixNano())
	if _, err := c.svc.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		MaxKeys: aws.Int32(1),
	}); err != nil {
		return err
	}
	if err := c.Upload(ctx, probe, []byte("smarticky")); err != nil {
		return err
	}
	_, headErr := c.svc.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(probe),
	})
	deleteErr := c.Delete(ctx, probe)
	if headErr != nil {
		return headErr
	}
	if deleteErr != nil {
		return deleteErr
	}
	return nil
}

func cleanupTargetBackups(ctx context.Context, client backupTargetClient, retentionDays int, maxCount int, filenamePrefixes ...string) error {
	if retentionDays == 0 && maxCount == 0 {
		return nil
	}
	allBackups, err := client.List(ctx)
	if err != nil {
		return err
	}
	backups := make([]BackupFileInfo, 0, len(allBackups))
	for _, backup := range allBackups {
		if backupFilenameHasPrefix(backup.Filename, filenamePrefixes) {
			backups = append(backups, backup)
		}
	}
	sortBackupFiles(backups)
	now := time.Now()
	for index, backup := range backups {
		shouldDelete := false
		if maxCount > 0 && index >= maxCount {
			shouldDelete = true
		}
		if retentionDays > 0 && now.Sub(backup.CreatedAt) > time.Duration(retentionDays)*24*time.Hour {
			shouldDelete = true
		}
		if shouldDelete {
			if err := client.Delete(ctx, backup.Filename); err != nil {
				return err
			}
		}
	}
	return nil
}

func backupFilenameHasPrefix(filename string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(filename, prefix) {
			return true
		}
	}
	return false
}

func sortBackupFiles(files []BackupFileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].CreatedAt.After(files[j].CreatedAt)
	})
}
