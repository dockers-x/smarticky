package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/notelink"
	"smarticky/ent/user"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type NoteMetadataResponse struct {
	ID              uuid.UUID  `json:"id"`
	Title           string     `json:"title"`
	Color           string     `json:"color"`
	ProtectionMode  string     `json:"protection_mode"`
	ContentRedacted bool       `json:"content_redacted"`
	IsStarred       bool       `json:"is_starred"`
	IsDeleted       bool       `json:"is_deleted"`
	FolderID        *uuid.UUID `json:"folder_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type NoteLinkResponse struct {
	ID              uuid.UUID             `json:"id"`
	TargetRef       string                `json:"target_ref"`
	TargetRefNorm   string                `json:"target_ref_norm"`
	TargetKey       string                `json:"target_key"`
	DisplayText     string                `json:"display_text"`
	LinkType        string                `json:"link_type"`
	OccurrenceCount int                   `json:"occurrence_count"`
	SourceNote      *NoteMetadataResponse `json:"source_note,omitempty"`
	TargetNote      *NoteMetadataResponse `json:"target_note,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type NoteLinksResponse struct {
	Outgoing   []NoteLinkResponse `json:"outgoing"`
	Backlinks  []NoteLinkResponse `json:"backlinks"`
	Unresolved []NoteLinkResponse `json:"unresolved"`
}

type NoteLinkGraphResponse struct {
	Nodes []NoteMetadataResponse `json:"nodes"`
	Edges []NoteLinkGraphEdge    `json:"edges"`
}

type NoteLinkGraphEdge struct {
	ID              uuid.UUID `json:"id"`
	Source          uuid.UUID `json:"source"`
	Target          uuid.UUID `json:"target"`
	LinkType        string    `json:"link_type"`
	DisplayText     string    `json:"display_text"`
	OccurrenceCount int       `json:"occurrence_count"`
}

func (h *Handler) GetNoteLinks(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := c.Get("user_id").(int)
	ctx := context.Background()

	if _, err := h.client.Note.Query().
		Where(note.IDEQ(id), note.HasUserWith(user.IDEQ(userID))).
		Only(ctx); err != nil {
		if ent.IsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	outgoingRows, err := h.client.NoteLink.Query().
		Where(notelink.UserIDEQ(userID), notelink.SourceNoteIDEQ(id)).
		WithSourceNote().
		WithTargetNote().
		Order(notelink.ByTargetRef()).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	backlinkRows, err := h.client.NoteLink.Query().
		Where(notelink.UserIDEQ(userID), notelink.TargetNoteIDEQ(id)).
		WithSourceNote().
		WithTargetNote().
		Order(notelink.ByTargetRef()).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response := NoteLinksResponse{
		Outgoing:   []NoteLinkResponse{},
		Backlinks:  []NoteLinkResponse{},
		Unresolved: []NoteLinkResponse{},
	}
	for _, row := range outgoingRows {
		item, err := h.noteLinkToResponse(ctx, row)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		if row.TargetNoteID == nil {
			response.Unresolved = append(response.Unresolved, item)
		} else {
			response.Outgoing = append(response.Outgoing, item)
		}
	}
	for _, row := range backlinkRows {
		item, err := h.noteLinkToResponse(ctx, row)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		response.Backlinks = append(response.Backlinks, item)
	}

	return c.JSON(http.StatusOK, response)
}

func (h *Handler) ListNoteLinkGraph(c echo.Context) error {
	userID := c.Get("user_id").(int)
	includeTrash := strings.EqualFold(c.QueryParam("include_trash"), "true")
	ctx := context.Background()

	rows, err := h.client.NoteLink.Query().
		Where(notelink.UserIDEQ(userID), notelink.TargetNoteIDNotNil()).
		WithSourceNote().
		WithTargetNote().
		Order(notelink.ByUpdatedAt()).
		All(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	nodesByID := make(map[uuid.UUID]NoteMetadataResponse)
	edges := make([]NoteLinkGraphEdge, 0, len(rows))
	for _, row := range rows {
		source := row.Edges.SourceNote
		target := row.Edges.TargetNote
		if source == nil || target == nil {
			continue
		}
		if !includeTrash && (source.IsDeleted || target.IsDeleted) {
			continue
		}

		sourceMeta, err := h.noteMetadata(ctx, source)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		targetMeta, err := h.noteMetadata(ctx, target)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		nodesByID[source.ID] = sourceMeta
		nodesByID[target.ID] = targetMeta
		edges = append(edges, NoteLinkGraphEdge{
			ID:              row.ID,
			Source:          source.ID,
			Target:          target.ID,
			LinkType:        string(row.LinkType),
			DisplayText:     row.DisplayText,
			OccurrenceCount: row.OccurrenceCount,
		})
	}

	nodes := make([]NoteMetadataResponse, 0, len(nodesByID))
	for _, node := range nodesByID {
		nodes = append(nodes, node)
	}
	return c.JSON(http.StatusOK, NoteLinkGraphResponse{Nodes: nodes, Edges: edges})
}

func (h *Handler) noteLinkToResponse(ctx context.Context, row *ent.NoteLink) (NoteLinkResponse, error) {
	var sourceMeta *NoteMetadataResponse
	if row.Edges.SourceNote != nil {
		meta, err := h.noteMetadata(ctx, row.Edges.SourceNote)
		if err != nil {
			return NoteLinkResponse{}, err
		}
		sourceMeta = &meta
	}
	var targetMeta *NoteMetadataResponse
	if row.Edges.TargetNote != nil {
		meta, err := h.noteMetadata(ctx, row.Edges.TargetNote)
		if err != nil {
			return NoteLinkResponse{}, err
		}
		targetMeta = &meta
	}

	return NoteLinkResponse{
		ID:              row.ID,
		TargetRef:       row.TargetRef,
		TargetRefNorm:   row.TargetRefNorm,
		TargetKey:       row.TargetKey,
		DisplayText:     row.DisplayText,
		LinkType:        string(row.LinkType),
		OccurrenceCount: row.OccurrenceCount,
		SourceNote:      sourceMeta,
		TargetNote:      targetMeta,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func (h *Handler) noteMetadata(ctx context.Context, row *ent.Note) (NoteMetadataResponse, error) {
	folderID, err := noteFolderID(ctx, row)
	if err != nil {
		return NoteMetadataResponse{}, err
	}
	return NoteMetadataResponse{
		ID:              row.ID,
		Title:           row.Title,
		Color:           row.Color,
		ProtectionMode:  string(row.ProtectionMode),
		ContentRedacted: row.ProtectionMode != note.ProtectionModeNone,
		IsStarred:       row.IsStarred,
		IsDeleted:       row.IsDeleted,
		FolderID:        folderID,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}
