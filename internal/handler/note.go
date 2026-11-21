package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/attachment"
	"smarticky/ent/folder"
	"smarticky/ent/note"
	"smarticky/ent/noteconnectionitemmap"
	"smarticky/ent/predicate"
	"smarticky/ent/tag"
	"smarticky/ent/user"
	"smarticky/ent/whiteboard"
	notesvc "smarticky/internal/notes"
	searchsvc "smarticky/internal/search"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"golang.org/x/crypto/argon2"
)

// Argon2 parameters
const (
	argon2Time    = 1
	argon2Memory  = 64 * 1024
	argon2Threads = 4
	argon2KeyLen  = 32
	saltLen       = 16
)

// hashPassword generates an argon2id hash of the password
func hashPassword(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash the password
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Encode to base64 for storage: $argon2id$salt$hash
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$%s$%s", encodedSalt, encodedHash), nil
}

// verifyPassword checks if the provided password matches the stored hash
func verifyPassword(password, storedHash string) (bool, error) {
	// Parse the stored hash: $argon2id$salt$hash
	parts := strings.Split(storedHash, "$")
	if len(parts) != 4 || parts[0] != "" || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, err
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false, err
	}

	// Hash the provided password with the same salt
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Compare using constant-time comparison
	return subtle.ConstantTimeCompare(hash, expectedHash) == 1, nil
}

type NoteResponse struct {
	ID               uuid.UUID  `json:"id"`
	Title            string     `json:"title"`
	Content          string     `json:"content"`
	Color            string     `json:"color"`
	ProtectionMode   string     `json:"protection_mode"`
	ContentRedacted  bool       `json:"content_redacted"`
	EncryptedContent string     `json:"encrypted_content,omitempty"`
	EncryptionAlg    string     `json:"encryption_alg,omitempty"`
	EncryptionKDF    string     `json:"encryption_kdf,omitempty"`
	EncryptionSalt   string     `json:"encryption_salt,omitempty"`
	EncryptionNonce  string     `json:"encryption_nonce,omitempty"`
	IsStarred        bool       `json:"is_starred"`
	IsDeleted        bool       `json:"is_deleted"`
	FolderID         *uuid.UUID `json:"folder_id"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func noteToResponse(n *ent.Note, revealContent bool) NoteResponse {
	content := n.Content
	redacted := false
	if !revealContent {
		switch n.ProtectionMode {
		case note.ProtectionModePassword:
			content = ""
			redacted = true
		case note.ProtectionModeEncrypted:
			content = ""
			redacted = true
		}
	}

	return NoteResponse{
		ID:               n.ID,
		Title:            n.Title,
		Content:          content,
		Color:            n.Color,
		ProtectionMode:   string(n.ProtectionMode),
		ContentRedacted:  redacted,
		EncryptedContent: n.EncryptedContent,
		EncryptionAlg:    n.EncryptionAlg,
		EncryptionKDF:    n.EncryptionKdf,
		EncryptionSalt:   n.EncryptionSalt,
		EncryptionNonce:  n.EncryptionNonce,
		IsStarred:        n.IsStarred,
		IsDeleted:        n.IsDeleted,
		FolderID:         n.FolderID,
		CreatedAt:        n.CreatedAt,
		UpdatedAt:        n.UpdatedAt,
	}
}

type CreateNoteRequest struct {
	Title    string       `json:"title"`
	Content  string       `json:"content"`
	Color    string       `json:"color"`
	FolderID OptionalUUID `json:"folder_id"`
}

type UpdateNoteRequest struct {
	Title              *string      `json:"title"`
	Content            *string      `json:"content"`
	Color              *string      `json:"color"`
	ProtectionMode     *string      `json:"protection_mode"`
	ProtectionPassword *string      `json:"protection_password"`
	EncryptedContent   *string      `json:"encrypted_content"`
	EncryptionAlg      *string      `json:"encryption_alg"`
	EncryptionKDF      *string      `json:"encryption_kdf"`
	EncryptionSalt     *string      `json:"encryption_salt"`
	EncryptionNonce    *string      `json:"encryption_nonce"`
	IsStarred          *bool        `json:"is_starred"`
	IsDeleted          *bool        `json:"is_deleted"`
	FolderID           OptionalUUID `json:"folder_id"`
}

func parseNoteTimeParam(value string, endOfDay bool, location *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}

	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}

	parsed, err := time.ParseInLocation("2006-01-02", value, location)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return parsed.AddDate(0, 0, 1).Add(-time.Nanosecond).UTC(), nil
	}
	return parsed.UTC(), nil
}

func noteQueryLocation(c echo.Context) (*time.Location, error) {
	value := strings.TrimSpace(c.QueryParam("timezone"))
	if value == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(value)
}

func applyNoteTimeFilter(c echo.Context, location *time.Location, param string, endOfDay bool, apply func(time.Time)) error {
	value := strings.TrimSpace(c.QueryParam(param))
	if value == "" {
		return nil
	}
	parsed, err := parseNoteTimeParam(value, endOfDay, location)
	if err != nil {
		return err
	}
	apply(parsed)
	return nil
}

func (h *Handler) ListNotes(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Get("user_id").(int)
	location, err := noteQueryLocation(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid timezone"})
	}

	type NoteWithTagsResponse struct {
		NoteResponse
		Tags []*ent.Tag `json:"tags"`
	}

	query := h.client.Note.Query()
	var orderedIDs []uuid.UUID
	useIndexSearch := false

	// 只返回当前用户的笔记
	query.Where(note.UserIDEQ(userID))

	// Filters
	if c.QueryParam("starred") == "true" {
		query.Where(note.IsStarred(true))
	}
	if c.QueryParam("trash") == "true" {
		query.Where(note.IsDeleted(true))
	} else {
		// Default: not deleted
		query.Where(note.IsDeleted(false))
	}

	// Color filtering
	if color := c.QueryParam("color"); color != "" {
		query.Where(note.ColorEQ(color))
	}

	// Tag filtering
	if tagsParam := c.QueryParam("tags"); tagsParam != "" {
		tagNames := strings.Split(tagsParam, ",")
		for _, tagName := range tagNames {
			tagName = strings.TrimSpace(tagName)
			if tagName == "" {
				continue
			}
			query.Where(note.HasTagsWith(tag.NameEQ(tagName), tag.HasUserWith(user.IDEQ(userID))))
		}
	}
	if folderParam := strings.TrimSpace(c.QueryParam("folder_id")); folderParam != "" {
		if folderParam == "unfiled" {
			query.Where(note.Not(note.HasFolder()))
		} else {
			folderID, err := uuid.Parse(folderParam)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid folder_id"})
			}
			query.Where(note.HasFolderWith(folder.ID(folderID), folder.HasUserWith(user.IDEQ(userID))))
		}
	}

	searchText := strings.TrimSpace(c.QueryParam("q"))
	if titleSearch := strings.TrimSpace(c.QueryParam("title")); titleSearch != "" {
		query.Where(note.TitleContainsFold(titleSearch))
	}
	if err := applyNoteTimeFilter(c, location, "created_from", false, func(value time.Time) {
		query.Where(note.CreatedAtGTE(value))
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid created_from"})
	}
	if err := applyNoteTimeFilter(c, location, "created_to", true, func(value time.Time) {
		query.Where(note.CreatedAtLTE(value))
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid created_to"})
	}
	if err := applyNoteTimeFilter(c, location, "updated_from", false, func(value time.Time) {
		query.Where(note.UpdatedAtGTE(value))
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid updated_from"})
	}
	if err := applyNoteTimeFilter(c, location, "updated_to", true, func(value time.Time) {
		query.Where(note.UpdatedAtLTE(value))
	}); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid updated_to"})
	}

	if searchText != "" && h.search != nil {
		candidateIDs, err := query.Clone().IDs(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if len(candidateIDs) == 0 {
			return c.JSON(http.StatusOK, []NoteWithTagsResponse{})
		}

		ids, err := h.search.Search(ctx, searchsvc.SearchOptions{
			UserID:       userID,
			Query:        searchText,
			IncludeTrash: c.QueryParam("trash") == "true",
			Limit:        len(candidateIDs),
			CandidateIDs: candidateIDs,
		})
		if err != nil {
			zap.L().Warn("Failed to search note index", zap.Error(err))
		} else {
			if len(ids) == 0 {
				return c.JSON(http.StatusOK, []NoteWithTagsResponse{})
			}
			orderedIDs = ids
			useIndexSearch = true
		}
	}
	if searchText != "" && !useIndexSearch {
		query.Where(
			note.Or(
				note.TitleContainsFold(searchText),
				note.And(
					note.ProtectionModeNEQ(note.ProtectionModeEncrypted),
					note.ContentContainsFold(searchText),
				),
			),
		)
	}

	if !useIndexSearch {
		orderedIDs, err = query.Clone().Order(ent.Desc(note.FieldUpdatedAt), note.ByID()).IDs(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	// Bound both the note and eager tag IN lists on large libraries. Keep the
	// order from the updated-at index or search engine across batch boundaries.
	response := make([]NoteWithTagsResponse, 0, len(orderedIDs))
	for ids := range slices.Chunk(orderedIDs, 512) {
		rows, err := query.Clone().Where(note.IDIn(ids...)).WithTags().All(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		byID := make(map[uuid.UUID]*ent.Note, len(rows))
		for _, row := range rows {
			byID[row.ID] = row
		}
		for _, id := range ids {
			if row := byID[id]; row != nil {
				response = append(response, NoteWithTagsResponse{NoteResponse: noteToResponse(row, false), Tags: row.Edges.Tags})
			}
		}
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) indexNoteBestEffort(ctx context.Context, n *ent.Note) {
	if h.search == nil {
		return
	}
	if err := h.search.IndexNote(ctx, n); err != nil {
		zap.L().Warn("Failed to index note", zap.String("note_id", n.ID.String()), zap.Error(err))
	}
}

func (h *Handler) deleteNoteFromIndexBestEffort(id uuid.UUID) {
	if h.search == nil {
		return
	}
	if err := h.search.DeleteNote(id); err != nil {
		zap.L().Warn("Failed to delete note from index", zap.String("note_id", id.String()), zap.Error(err))
	}
}

func (h *Handler) rebuildSearchIndexBestEffort(ctx context.Context) {
	if h.search == nil {
		return
	}
	if err := h.search.Rebuild(ctx, h.client); err != nil {
		zap.L().Warn("Failed to rebuild note index", zap.Error(err))
	}
}

func (h *Handler) CreateNote(c echo.Context) error {
	var req CreateNoteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// 获取当前用户ID
	userID := c.Get("user_id").(int)

	ctx := c.Request().Context()
	tx, err := h.client.Tx(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer tx.Rollback()
	create := tx.Note.Create().
		SetTitle(req.Title).
		SetContent(req.Content).
		SetColor(req.Color).
		SetUserID(userID)
	if req.FolderID.Set && req.FolderID.Value != nil {
		if _, err := tx.Folder.Query().Where(folder.ID(*req.FolderID.Value), folder.HasUserWith(user.IDEQ(userID))).Only(ctx); err != nil {
			if ent.IsNotFound(err) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		create.SetFolderID(*req.FolderID.Value)
	}

	n, err := create.Save(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := notesvc.SyncChangedLinks(ctx, tx, userID, n, n.Title); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := tx.Commit(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	n.Unwrap()
	h.indexNoteBestEffort(ctx, n)
	response := noteToResponse(n, false)
	return c.JSON(http.StatusCreated, response)
}

func (h *Handler) GetNote(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	userID := c.Get("user_id").(int)

	ctx := c.Request().Context()
	// 查询note，并验证是否属于当前用户
	n, err := h.client.Note.Query().
		Where(
			note.ID(id),
			note.HasUserWith(user.IDEQ(userID)),
		).
		Only(ctx)

	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Get tags for this note
	tags, err := n.QueryTags().All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Convert to response format that includes tags
	type NoteWithTagsResponse struct {
		NoteResponse
		Tags []*ent.Tag `json:"tags"`
	}

	noteResponse := noteToResponse(n, false)
	response := NoteWithTagsResponse{
		NoteResponse: noteResponse,
		Tags:         tags,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) UpdateNote(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	userID := c.Get("user_id").(int)

	var req UpdateNoteRequest
	if err := bindStrictJSON(c, &req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := c.Request().Context()

	var hashedPassword string
	if req.ProtectionMode != nil && *req.ProtectionMode == "password" {
		exists, err := h.client.Note.Query().Where(note.IDEQ(id), note.UserIDEQ(userID)).Exist(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if !exists {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
		}
		if req.ProtectionPassword == nil || strings.TrimSpace(*req.ProtectionPassword) == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "protection_password is required"})
		}
		hashedPassword, err = hashPassword(*req.ProtectionPassword)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		}
	}

	tx, err := h.client.Tx(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer tx.Rollback()
	n, err := tx.Note.Query().
		Where(
			note.ID(id),
			note.HasUserWith(user.IDEQ(userID)),
		).
		Only(ctx)

	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// 更新笔记
	update := n.Update().SetUpdatedAt(time.Now())

	if req.Title != nil {
		update.SetTitle(*req.Title)
	}
	if req.Content != nil {
		requestedMode := ""
		if req.ProtectionMode != nil {
			requestedMode = *req.ProtectionMode
		}
		if n.ProtectionMode == note.ProtectionModeEncrypted || requestedMode == "encrypted" {
			if requestedMode != "none" && requestedMode != "password" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "content cannot be updated while note is encrypted"})
			}
		}
		update.SetContent(*req.Content)
	}
	if req.Color != nil {
		update.SetColor(*req.Color)
	}
	if req.ProtectionMode != nil {
		switch *req.ProtectionMode {
		case "none":
			update.SetProtectionMode(note.ProtectionModeNone)
			update.ClearProtectionPasswordHash()
			update.ClearEncryptedContent()
			update.ClearEncryptionAlg()
			update.ClearEncryptionKdf()
			update.ClearEncryptionSalt()
			update.ClearEncryptionNonce()
		case "password":
			update.SetProtectionMode(note.ProtectionModePassword)
			update.SetProtectionPasswordHash(hashedPassword)
			update.ClearEncryptedContent()
			update.ClearEncryptionAlg()
			update.ClearEncryptionKdf()
			update.ClearEncryptionSalt()
			update.ClearEncryptionNonce()
		case "encrypted":
			if req.EncryptedContent == nil || *req.EncryptedContent == "" ||
				req.EncryptionAlg == nil || *req.EncryptionAlg == "" ||
				req.EncryptionKDF == nil || *req.EncryptionKDF == "" ||
				req.EncryptionSalt == nil || *req.EncryptionSalt == "" ||
				req.EncryptionNonce == nil || *req.EncryptionNonce == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "encrypted payload is required"})
			}
			update.SetProtectionMode(note.ProtectionModeEncrypted)
			update.SetContent("")
			update.ClearProtectionPasswordHash()
			update.SetEncryptedContent(*req.EncryptedContent)
			update.SetEncryptionAlg(*req.EncryptionAlg)
			update.SetEncryptionKdf(*req.EncryptionKDF)
			update.SetEncryptionSalt(*req.EncryptionSalt)
			update.SetEncryptionNonce(*req.EncryptionNonce)
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid protection_mode"})
		}
	}
	if req.IsStarred != nil {
		update.SetIsStarred(*req.IsStarred)
	}
	if req.IsDeleted != nil {
		update.SetIsDeleted(*req.IsDeleted)
	}
	if req.FolderID.Set {
		if req.FolderID.Value == nil {
			update.ClearFolder()
		} else {
			if _, err := tx.Folder.Query().Where(folder.ID(*req.FolderID.Value), folder.HasUserWith(user.IDEQ(userID))).Only(ctx); err != nil {
				if ent.IsNotFound(err) {
					return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
				}
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			update.SetFolderID(*req.FolderID.Value)
		}
	}

	oldTitle := n.Title
	n, err = update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if req.Title != nil || req.Content != nil || req.ProtectionMode != nil {
		var titles []string
		if n.Title != oldTitle {
			titles = []string{oldTitle, n.Title}
		}
		if err := notesvc.SyncChangedLinks(ctx, tx, userID, n, titles...); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}
	if err := tx.Commit(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	n.Unwrap()
	h.indexNoteBestEffort(ctx, n)
	response := noteToResponse(n, false)
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) DeleteNote(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	count, err := h.deleteNotes(c.Request().Context(), c.Get("user_id").(int), note.IDEQ(id))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if count == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) EmptyTrash(c echo.Context) error {
	count, err := h.deleteNotes(c.Request().Context(), c.Get("user_id").(int), note.IsDeleted(true))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]int{"deleted_count": count})
}

func (h *Handler) deleteNotes(ctx context.Context, userID int, filter predicate.Note) (int, error) {
	tx, err := h.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.Note.Query().Where(note.UserIDEQ(userID), filter).Select(note.FieldID, note.FieldTitle).All(ctx)
	if err != nil {
		return 0, err
	}

	var paths []string
	for batch := range slices.Chunk(rows, 256) {
		ids := make([]uuid.UUID, 0, len(batch))
		titles := make([]string, 0, len(batch))
		for _, row := range batch {
			ids = append(ids, row.ID)
			titles = append(titles, row.Title)
		}
		files := attachment.HasNoteWith(note.IDIn(ids...))
		batchPaths, err := tx.Attachment.Query().Where(files).Select(attachment.FieldFilePath).Strings(ctx)
		if err != nil {
			return 0, err
		}
		paths = append(paths, batchPaths...)
		if _, err := tx.Attachment.Delete().Where(files).Exec(ctx); err != nil {
			return 0, err
		}
		if _, err := tx.Whiteboard.Delete().Where(whiteboard.HasNoteWith(note.IDIn(ids...))).Exec(ctx); err != nil {
			return 0, err
		}
		if _, err := tx.NoteConnectionItemMap.Delete().Where(noteconnectionitemmap.NoteIDIn(ids...)).Exec(ctx); err != nil {
			return 0, err
		}
		// The schema cascades outgoing links and clears the foreign keys on
		// incoming links and connection jobs. Re-resolve surviving references.
		if _, err := tx.Note.Delete().Where(note.UserIDEQ(userID), note.IDIn(ids...)).Exec(ctx); err != nil {
			return 0, err
		}
		if err := notesvc.SyncChangedLinks(ctx, tx, userID, nil, titles...); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	for _, path := range paths {
		if err := h.fs.Remove(path); err != nil {
			zap.L().Warn("Failed to remove deleted note attachment", zap.String("path", path), zap.Error(err))
		}
	}
	for _, row := range rows {
		h.deleteNoteFromIndexBestEffort(row.ID)
	}
	return len(rows), nil
}

// VerifyNotePassword verifies if the provided password matches the note's password
func (h *Handler) VerifyNotePassword(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	type VerifyPasswordRequest struct {
		Password string `json:"password"`
	}

	var req VerifyPasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := c.Request().Context()
	userID := c.Get("user_id").(int)
	n, err := h.client.Note.Query().
		Where(
			note.ID(id),
			note.HasUserWith(user.IDEQ(userID)),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if n.ProtectionMode != note.ProtectionModePassword || n.ProtectionPasswordHash == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "note is not password protected"})
	}

	// Verify password
	valid, err := verifyPassword(req.Password, n.ProtectionPasswordHash)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to verify password"})
	}

	if !valid {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "incorrect password"})
	}

	// Return success with note content
	response := noteToResponse(n, true)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"note":    response,
	})
}
