package notes

import (
	"context"
	"sort"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/tag"
	"smarticky/ent/user"
	searchsvc "smarticky/internal/search"

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
	search *searchsvc.Service
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

func NewService(client *ent.Client, searchService ...*searchsvc.Service) *Service {
	var index *searchsvc.Service
	if len(searchService) > 0 {
		index = searchService[0]
	}
	return &Service{client: client, search: index}
}

func (s *Service) List(ctx context.Context, userID int, opts ListOptions) ([]NoteView, error) {
	limit := clampLimit(opts.Limit)
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}
	q := strings.TrimSpace(opts.Query)
	var searchIDs []uuid.UUID
	useSearch := false

	if q != "" && s.search != nil {
		ids, err := s.search.Search(ctx, searchsvc.SearchOptions{
			UserID:       userID,
			Query:        q,
			IncludeTrash: opts.IncludeTrash,
			Limit:        searchsvc.CandidateLimit(limit, offset),
		})
		if err == nil {
			if len(ids) == 0 {
				return []NoteView{}, nil
			}
			searchIDs = ids
			useSearch = true
		}
	}

	query := s.client.Note.Query().
		Where(note.HasUserWith(user.IDEQ(userID)))

	if !opts.IncludeTrash {
		query.Where(note.IsDeleted(false))
	}

	if q != "" {
		if useSearch {
			query.Where(note.IDIn(searchIDs...))
		} else {
			query.Where(note.Or(
				note.TitleContainsFold(q),
				note.And(
					note.ProtectionModeNEQ(note.ProtectionModeEncrypted),
					note.ContentContainsFold(q),
				),
			))
		}
	}

	if !useSearch {
		query.Limit(limit).
			Offset(offset).
			Order(ent.Desc(note.FieldUpdatedAt))
	}

	rows, err := query.All(ctx)
	if err != nil {
		return nil, err
	}
	if useSearch {
		orderNotesBySearch(rows, searchIDs)
		rows = paginateNotes(rows, offset, limit)
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

func orderNotesBySearch(rows []*ent.Note, ids []uuid.UUID) {
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

func paginateNotes(rows []*ent.Note, offset, limit int) []*ent.Note {
	if offset >= len(rows) {
		return []*ent.Note{}
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end]
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
	if s.search != nil {
		_ = s.search.IndexNote(ctx, row)
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
