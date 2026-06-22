package notes

import (
	"context"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/tag"
	"smarticky/ent/user"

	"github.com/google/uuid"
)

const (
	DefaultLimit  = 20
	MaxLimit      = 100
	MaxTitleLen   = 240
	MaxContentLen = 500000
)

type Service struct {
	client *ent.Client
}

type ListOptions struct {
	Query        string
	Limit        int
	Offset       int
	IncludeTrash bool
	RedactLocked bool
}

type CreateInput struct {
	Title   string
	Content string
	Color   string
}

type NoteView struct {
	ID              uuid.UUID  `json:"id"`
	Title           string     `json:"title"`
	Content         string     `json:"content,omitempty"`
	Color           string     `json:"color"`
	ProtectionMode  string     `json:"protection_mode"`
	IsStarred       bool       `json:"is_starred"`
	IsDeleted       bool       `json:"is_deleted"`
	ContentRedacted bool       `json:"content_redacted"`
	FolderID        *uuid.UUID `json:"folder_id"`
	Tags            []string   `json:"tags,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func NewService(client *ent.Client) *Service {
	return &Service{client: client}
}

func (s *Service) List(ctx context.Context, userID int, opts ListOptions) ([]NoteView, error) {
	limit := clampLimit(opts.Limit)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	query := s.client.Note.Query().
		Where(note.HasUserWith(user.IDEQ(userID))).
		Limit(limit).
		Offset(offset).
		Order(ent.Desc(note.FieldUpdatedAt))

	if !opts.IncludeTrash {
		query.Where(note.IsDeleted(false))
	}

	if q := strings.TrimSpace(opts.Query); q != "" {
		query.Where(note.Or(
			note.TitleContainsFold(q),
			note.And(
				note.ProtectionModeNEQ(note.ProtectionModeEncrypted),
				note.ContentContainsFold(q),
			),
		))
	}

	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]NoteView, 0, len(rows))
	for _, row := range rows {
		view, err := s.noteToView(ctx, row, opts.RedactLocked)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, userID int, id uuid.UUID, redactLocked bool) (NoteView, error) {
	row, err := s.client.Note.Query().
		Where(note.IDEQ(id), note.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if err != nil {
		return NoteView{}, err
	}
	return s.noteToView(ctx, row, redactLocked)
}

func (s *Service) Create(ctx context.Context, userID int, input CreateInput) (NoteView, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Untitled"
	}
	if len([]rune(title)) > MaxTitleLen {
		title = string([]rune(title)[:MaxTitleLen])
	}

	content := input.Content
	if len([]rune(content)) > MaxContentLen {
		content = string([]rune(content)[:MaxContentLen])
	}

	row, err := s.client.Note.Create().
		SetTitle(title).
		SetContent(content).
		SetColor(strings.TrimSpace(input.Color)).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		return NoteView{}, err
	}
	return s.noteToView(ctx, row, false)
}

func (s *Service) noteToView(ctx context.Context, row *ent.Note, redactLocked bool) (NoteView, error) {
	tagRows, err := row.QueryTags().Order(ent.Asc(tag.FieldName)).All(ctx)
	if err != nil {
		return NoteView{}, err
	}

	tags := make([]string, 0, len(tagRows))
	for _, tagRow := range tagRows {
		tags = append(tags, tagRow.Name)
	}

	content := row.Content
	redacted := false
	if redactLocked && row.ProtectionMode != note.ProtectionModeNone {
		content = ""
		redacted = true
	}
	folderID, err := noteFolderID(ctx, row)
	if err != nil {
		return NoteView{}, err
	}

	return NoteView{
		ID:              row.ID,
		Title:           row.Title,
		Content:         content,
		Color:           row.Color,
		ProtectionMode:  string(row.ProtectionMode),
		IsStarred:       row.IsStarred,
		IsDeleted:       row.IsDeleted,
		ContentRedacted: redacted,
		FolderID:        folderID,
		Tags:            tags,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}, nil
}

func noteFolderID(ctx context.Context, row *ent.Note) (*uuid.UUID, error) {
	folderRow, err := row.QueryFolder().Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	id := folderRow.ID
	return &id, nil
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	if limit > MaxLimit {
		return MaxLimit
	}
	return limit
}
