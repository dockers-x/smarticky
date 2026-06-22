package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/folder"
	"smarticky/ent/note"
	"smarticky/ent/tag"
	"smarticky/ent/user"
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

func noteToResponse(ctx context.Context, n *ent.Note, revealContent bool) (NoteResponse, error) {
	folderID, err := noteFolderID(ctx, n)
	if err != nil {
		return NoteResponse{}, err
	}

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
		FolderID:         folderID,
		CreatedAt:        n.CreatedAt,
		UpdatedAt:        n.UpdatedAt,
	}, nil
}

func noteFolderID(ctx context.Context, n *ent.Note) (*uuid.UUID, error) {
	f, err := n.QueryFolder().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	id := f.ID
	return &id, nil
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
		return parsed.Add(24*time.Hour - time.Nanosecond).UTC(), nil
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
	ctx := context.Background()
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
	var searchIDs []uuid.UUID
	useIndexSearch := false

	// 只返回当前用户的笔记
	query.Where(note.HasUserWith(user.IDEQ(userID)))

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
			searchIDs = ids
			useIndexSearch = true
			query.Where(note.IDIn(ids...))
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
		// Order by updated_at desc
		query.Order(ent.Desc(note.FieldUpdatedAt))
	}

	notes, err := query.All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if useIndexSearch {
		orderHandlerNotesBySearch(notes, searchIDs)
	}

	response := make([]NoteWithTagsResponse, len(notes))
	for i, n := range notes {
		// Get tags for this note
		tags, err := n.QueryTags().All(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		noteResponse, err := noteToResponse(ctx, n, false)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		response[i] = NoteWithTagsResponse{
			NoteResponse: noteResponse,
			Tags:         tags,
		}
	}

	return c.JSON(http.StatusOK, response)
}

func orderHandlerNotesBySearch(rows []*ent.Note, ids []uuid.UUID) {
	rank := searchsvc.IDRank(ids)
	sort.SliceStable(rows, func(i, j int) bool {
		left, ok := rank[rows[i].ID]
		if !ok {
			left = len(rank)
		}
		right, ok := rank[rows[j].ID]
		if !ok {
			right = len(rank)
		}
		return left < right
	})
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

	ctx := context.Background()
	create := h.client.Note.Create().
		SetTitle(req.Title).
		SetContent(req.Content).
		SetColor(req.Color).
		SetUserID(userID)
	if req.FolderID.Set && req.FolderID.Value != nil {
		if _, err := h.folderForUser(ctx, userID, *req.FolderID.Value); err != nil {
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
	h.indexNoteBestEffort(ctx, n)
	if err := h.notes.SyncUserLinks(ctx, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response, err := noteToResponse(ctx, n, false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, response)
}

func (h *Handler) GetNote(c echo.Context) error {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	userID := c.Get("user_id").(int)

	ctx := context.Background()
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

	noteResponse, err := noteToResponse(ctx, n, false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
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

	ctx := context.Background()

	// 先验证笔记是否存在且属于当前用户
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
			if req.ProtectionPassword == nil || strings.TrimSpace(*req.ProtectionPassword) == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "protection_password is required"})
			}
			hashedPassword, err := hashPassword(*req.ProtectionPassword)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
			}
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
			if _, err := h.folderForUser(ctx, userID, *req.FolderID.Value); err != nil {
				if ent.IsNotFound(err) {
					return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
				}
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			update.SetFolderID(*req.FolderID.Value)
		}
	}

	n, err = update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	h.indexNoteBestEffort(ctx, n)
	if req.Title != nil || req.Content != nil || req.ProtectionMode != nil {
		if err := h.notes.SyncUserLinks(ctx, userID); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	response, err := noteToResponse(ctx, n, false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) DeleteNote(c echo.Context) error {
	// Permanent delete
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	userID := c.Get("user_id").(int)

	ctx := context.Background()
	if err := h.notes.DeleteLinksForNotes(ctx, userID, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// 删除时验证笔记是否属于当前用户
	count, err := h.client.Note.Delete().
		Where(
			note.ID(id),
			note.HasUserWith(user.IDEQ(userID)),
		).
		Exec(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if count == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}
	h.deleteNoteFromIndexBestEffort(id)

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) EmptyTrash(c echo.Context) error {
	userID := c.Get("user_id").(int)
	ctx := context.Background()
	ids, err := h.client.Note.Query().
		Where(
			note.IsDeleted(true),
			note.HasUserWith(user.IDEQ(userID)),
		).
		IDs(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := h.notes.DeleteLinksForNotes(ctx, userID, ids...); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	count, err := h.client.Note.Delete().
		Where(
			note.IsDeleted(true),
			note.HasUserWith(user.IDEQ(userID)),
		).
		Exec(ctx)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	h.rebuildSearchIndexBestEffort(context.Background())

	return c.JSON(http.StatusOK, map[string]int{"deleted_count": count})
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

	ctx := context.Background()
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
	response, err := noteToResponse(ctx, n, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"note":    response,
	})
}
