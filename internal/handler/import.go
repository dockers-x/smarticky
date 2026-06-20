package handler

import (
	"errors"
	"net/http"
	"strconv"

	"smarticky/ent"
	"smarticky/ent/importjob"
	"smarticky/ent/user"
	importsvc "smarticky/internal/importer"

	"github.com/labstack/echo/v4"
)

const maxImportUploadBytes = 50 << 20
const maxImportRequestBytes = maxImportUploadBytes + (1 << 20)

func (h *Handler) PreviewEvernoteImport(c echo.Context) error {
	userID := c.Get("user_id").(int)

	req := c.Request()
	req.Body = http.MaxBytesReader(c.Response().Writer, req.Body, maxImportRequestBytes)

	file, err := c.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "Import file is too large"})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "No file uploaded"})
	}
	if file.Size > maxImportUploadBytes {
		return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "Import file is too large"})
	}

	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to open import file"})
	}
	defer src.Close()

	result, err := h.importer.PreviewEvernote(c.Request().Context(), userID, file.Filename, src)
	if err != nil {
		if errors.Is(err, importsvc.ErrImportTooLarge) {
			return c.JSON(http.StatusRequestEntityTooLarge, map[string]string{"error": "Import file is too large"})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Failed to parse Evernote import file"})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *Handler) ConfirmEvernoteImport(c echo.Context) error {
	userID := c.Get("user_id").(int)

	var req struct {
		JobID int `json:"job_id"`
	}
	if err := c.Bind(&req); err != nil || req.JobID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid import job"})
	}

	result, err := h.importer.ConfirmEvernote(c.Request().Context(), userID, req.JobID)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Import job not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import Evernote file"})
	}

	return c.JSON(http.StatusOK, result)
}

func (h *Handler) ListImportJobs(c echo.Context) error {
	userID := c.Get("user_id").(int)

	jobs, err := h.client.ImportJob.Query().
		Where(importjob.HasUserWith(user.IDEQ(userID))).
		Order(ent.Desc(importjob.FieldCreatedAt)).
		All(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list import jobs"})
	}

	return c.JSON(http.StatusOK, jobs)
}

func (h *Handler) GetImportJob(c echo.Context) error {
	userID := c.Get("user_id").(int)
	jobID, err := strconv.Atoi(c.Param("id"))
	if err != nil || jobID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid import job"})
	}

	job, err := h.client.ImportJob.Query().
		Where(importjob.ID(jobID), importjob.HasUserWith(user.IDEQ(userID))).
		WithItems().
		Only(c.Request().Context())
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Import job not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get import job"})
	}

	return c.JSON(http.StatusOK, job)
}
