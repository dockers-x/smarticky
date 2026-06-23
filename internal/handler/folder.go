package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/folder"
	"smarticky/ent/note"
	"smarticky/ent/user"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	defaultFolderMaxDepth      = 3
	minConfigurableFolderDepth = 1
	maxConfigurableFolderDepth = 50
	maxBulkMoveNotes           = 200
)

type OptionalUUID struct {
	Set   bool
	Value *uuid.UUID
}

func (id *OptionalUUID) UnmarshalJSON(data []byte) error {
	id.Set = true
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) {
		id.Value = nil
		return nil
	}

	var raw string
	if err := json.Unmarshal(trimmed, &raw); err != nil {
		return err
	}
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "unfiled" {
		id.Value = nil
		return nil
	}

	parsed, err := uuid.Parse(raw)
	if err != nil {
		return err
	}
	id.Value = &parsed
	return nil
}

type FolderResponse struct {
	ID         uuid.UUID  `json:"id"`
	Name       string     `json:"name"`
	ParentID   *uuid.UUID `json:"parent_id"`
	SortOrder  int        `json:"sort_order"`
	IsStarred  bool       `json:"is_starred"`
	NoteCount  int        `json:"note_count"`
	ChildCount int        `json:"child_count"`
	Depth      int        `json:"depth"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type FolderSettingsResponse struct {
	MaxDepth int `json:"max_depth"`
}

type CreateFolderRequest struct {
	Name      string       `json:"name"`
	ParentID  OptionalUUID `json:"parent_id"`
	SortOrder *int         `json:"sort_order"`
}

type UpdateFolderRequest struct {
	Name      *string      `json:"name"`
	ParentID  OptionalUUID `json:"parent_id"`
	SortOrder *int         `json:"sort_order"`
	IsStarred *bool        `json:"is_starred"`
}

type MoveNotesRequest struct {
	NoteIDs  []uuid.UUID  `json:"note_ids"`
	FolderID OptionalUUID `json:"folder_id"`
}

func (h *Handler) ListFolders(c echo.Context) error {
	ctx := context.Background()
	userID := c.Get("user_id").(int)

	folders, err := h.client.Folder.Query().
		Where(folder.HasUserWith(user.IDEQ(userID))).
		Order(ent.Asc(folder.FieldSortOrder), ent.Asc(folder.FieldName)).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response := make([]FolderResponse, 0, len(folders))
	for _, f := range folders {
		item, err := h.folderToResponse(ctx, userID, f)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		response = append(response, item)
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) CreateFolder(c echo.Context) error {
	var req CreateFolderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder name is required"})
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	maxDepth, err := h.folderMaxDepth(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	create := h.client.Folder.Create().
		SetName(name).
		SetUserID(userID)
	if req.SortOrder != nil {
		create.SetSortOrder(*req.SortOrder)
	}

	depth := 1
	if req.ParentID.Set && req.ParentID.Value != nil {
		parent, err := h.folderForUser(ctx, userID, *req.ParentID.Value)
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
		}
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		parentDepth, err := h.folderDepth(ctx, userID, parent.ID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		depth = parentDepth + 1
		create.SetParent(parent)
	}
	if depth > maxDepth {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder depth exceeds limit"})
	}

	created, err := create.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response, err := h.folderToResponse(ctx, userID, created)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, response)
}

func (h *Handler) UpdateFolder(c echo.Context) error {
	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid folder id"})
	}

	var req UpdateFolderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	current, err := h.folderForUser(ctx, userID, folderID)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	update := current.Update()
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder name is required"})
		}
		update.SetName(name)
	}
	if req.SortOrder != nil {
		update.SetSortOrder(*req.SortOrder)
	}
	if req.IsStarred != nil {
		update.SetIsStarred(*req.IsStarred)
	}
	if req.ParentID.Set {
		if req.ParentID.Value == nil {
			subtreeHeight, err := h.folderSubtreeHeight(ctx, current)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			maxDepth, err := h.folderMaxDepth(ctx)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if subtreeHeight > maxDepth {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder depth exceeds limit"})
			}
			update.ClearParent()
		} else {
			parentID := *req.ParentID.Value
			if parentID == current.ID {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder cannot be its own parent"})
			}
			parent, err := h.folderForUser(ctx, userID, parentID)
			if ent.IsNotFound(err) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
			}
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			descendant, err := h.folderIsDescendant(ctx, userID, parent.ID, current.ID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if descendant {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder cannot move under its descendant"})
			}
			parentDepth, err := h.folderDepth(ctx, userID, parent.ID)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			subtreeHeight, err := h.folderSubtreeHeight(ctx, current)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			maxDepth, err := h.folderMaxDepth(ctx)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			if parentDepth+subtreeHeight > maxDepth {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder depth exceeds limit"})
			}
			update.SetParent(parent)
		}
	}

	updated, err := update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response, err := h.folderToResponse(ctx, userID, updated)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, response)
}

func (h *Handler) DeleteFolder(c echo.Context) error {
	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid folder id"})
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	f, err := h.folderForUser(ctx, userID, folderID)
	if ent.IsNotFound(err) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	children, err := f.QueryChildren().Count(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	notes, err := f.QueryNotes().Where(note.IsDeleted(false)).Count(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if children > 0 || notes > 0 {
		return c.JSON(http.StatusConflict, map[string]string{"error": "folder is not empty"})
	}
	if err := h.client.Note.Update().
		Where(
			note.IsDeleted(true),
			note.HasFolderWith(folder.ID(folderID)),
			note.HasUserWith(user.IDEQ(userID)),
		).
		ClearFolder().
		Exec(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := h.client.Folder.DeleteOne(f).Exec(ctx); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) GetFolderSettings(c echo.Context) error {
	maxDepth, err := h.folderMaxDepth(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, FolderSettingsResponse{MaxDepth: maxDepth})
}

func (h *Handler) UpdateFolderSettings(c echo.Context) error {
	var req struct {
		MaxDepth *int `json:"max_depth"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.MaxDepth == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "max_depth is required"})
	}
	if *req.MaxDepth < minConfigurableFolderDepth || *req.MaxDepth > maxConfigurableFolderDepth {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "max_depth out of range"})
	}

	ctx := context.Background()
	currentDepth, err := h.currentMaxFolderDepth(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if *req.MaxDepth < currentDepth {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "max_depth is less than existing folder depth"})
	}

	config, err := h.getOrCreateBackupConfig(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	config, err = config.Update().SetFolderMaxDepth(*req.MaxDepth).Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, FolderSettingsResponse{MaxDepth: normalizedFolderMaxDepth(config.FolderMaxDepth)})
}

func (h *Handler) MoveNotes(c echo.Context) error {
	var req MoveNotesRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if len(req.NoteIDs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "note_ids is required"})
	}
	if len(req.NoteIDs) > maxBulkMoveNotes {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "too many notes"})
	}
	if !req.FolderID.Set {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
	}

	ctx := context.Background()
	userID := c.Get("user_id").(int)
	noteIDs := uniqueUUIDs(req.NoteIDs)
	if req.FolderID.Set && req.FolderID.Value != nil {
		if _, err := h.folderForUser(ctx, userID, *req.FolderID.Value); err != nil {
			if ent.IsNotFound(err) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "folder not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	count, err := h.client.Note.Query().
		Where(note.IDIn(noteIDs...), note.HasUserWith(user.IDEQ(userID))).
		Count(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if count != len(noteIDs) {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
	}

	update := h.client.Note.Update().
		Where(note.IDIn(noteIDs...), note.HasUserWith(user.IDEQ(userID))).
		SetUpdatedAt(time.Now())
	if req.FolderID.Set && req.FolderID.Value != nil {
		update.SetFolderID(*req.FolderID.Value)
	} else {
		update.ClearFolder()
	}

	updated, err := update.Save(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]int{"updated_count": updated})
}

func (h *Handler) folderToResponse(ctx context.Context, userID int, f *ent.Folder) (FolderResponse, error) {
	parentID, err := h.folderParentID(ctx, f)
	if err != nil {
		return FolderResponse{}, err
	}
	noteCount, err := f.QueryNotes().Where(note.IsDeleted(false)).Count(ctx)
	if err != nil {
		return FolderResponse{}, err
	}
	childCount, err := f.QueryChildren().Count(ctx)
	if err != nil {
		return FolderResponse{}, err
	}
	depth, err := h.folderDepth(ctx, userID, f.ID)
	if err != nil {
		return FolderResponse{}, err
	}

	return FolderResponse{
		ID:         f.ID,
		Name:       f.Name,
		ParentID:   parentID,
		SortOrder:  f.SortOrder,
		IsStarred:  f.IsStarred,
		NoteCount:  noteCount,
		ChildCount: childCount,
		Depth:      depth,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
	}, nil
}

func (h *Handler) folderParentID(ctx context.Context, f *ent.Folder) (*uuid.UUID, error) {
	parent, err := f.QueryParent().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	id := parent.ID
	return &id, nil
}

func (h *Handler) folderForUser(ctx context.Context, userID int, folderID uuid.UUID) (*ent.Folder, error) {
	return h.client.Folder.Query().
		Where(folder.ID(folderID), folder.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
}

func (h *Handler) folderDepth(ctx context.Context, userID int, folderID uuid.UUID) (int, error) {
	depth := 1
	current, err := h.folderForUser(ctx, userID, folderID)
	if err != nil {
		return 0, err
	}

	for depth <= maxConfigurableFolderDepth+1 {
		parent, err := current.QueryParent().Only(ctx)
		if ent.IsNotFound(err) {
			return depth, nil
		}
		if err != nil {
			return 0, err
		}
		depth++
		current = parent
	}
	return depth, nil
}

func (h *Handler) folderSubtreeHeight(ctx context.Context, f *ent.Folder) (int, error) {
	children, err := f.QueryChildren().All(ctx)
	if err != nil {
		return 0, err
	}
	height := 1
	for _, child := range children {
		childHeight, err := h.folderSubtreeHeight(ctx, child)
		if err != nil {
			return 0, err
		}
		if childHeight+1 > height {
			height = childHeight + 1
		}
	}
	return height, nil
}

func (h *Handler) folderIsDescendant(ctx context.Context, userID int, possibleDescendantID uuid.UUID, ancestorID uuid.UUID) (bool, error) {
	current, err := h.folderForUser(ctx, userID, possibleDescendantID)
	if err != nil {
		return false, err
	}
	for {
		if current.ID == ancestorID {
			return true, nil
		}
		parent, err := current.QueryParent().Only(ctx)
		if ent.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		current = parent
	}
}

func (h *Handler) folderMaxDepth(ctx context.Context) (int, error) {
	config, err := h.getOrCreateBackupConfig(ctx)
	if err != nil {
		return 0, err
	}
	return normalizedFolderMaxDepth(config.FolderMaxDepth), nil
}

func (h *Handler) getOrCreateBackupConfig(ctx context.Context) (*ent.BackupConfig, error) {
	config, err := h.client.BackupConfig.Query().First(ctx)
	if err == nil {
		return config, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}
	return h.client.BackupConfig.Create().Save(ctx)
}

func (h *Handler) currentMaxFolderDepth(ctx context.Context) (int, error) {
	folders, err := h.client.Folder.Query().All(ctx)
	if err != nil {
		return 0, err
	}
	maxDepth := 0
	for _, f := range folders {
		owner, err := f.QueryUser().Only(ctx)
		if err != nil {
			return 0, err
		}
		depth, err := h.folderDepth(ctx, owner.ID, f.ID)
		if err != nil {
			return 0, err
		}
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth, nil
}

func normalizedFolderMaxDepth(value int) int {
	if value < minConfigurableFolderDepth {
		return defaultFolderMaxDepth
	}
	if value > maxConfigurableFolderDepth {
		return maxConfigurableFolderDepth
	}
	return value
}

func uniqueUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}
