package search

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/ent/tag"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/lang/cjk"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/google/uuid"
)

const defaultLimit = 100

var errClosed = errors.New("search index is closed")

type Service struct {
	mu       sync.RWMutex
	index    bleve.Index
	path     string
	inMemory bool
}

type Document struct {
	ID             string    `json:"id"`
	UserID         int       `json:"user_id"`
	Title          string    `json:"title"`
	Content        string    `json:"content"`
	Tags           []string  `json:"tags"`
	FolderID       string    `json:"folder_id"`
	ProtectionMode string    `json:"protection_mode"`
	IsDeleted      bool      `json:"is_deleted"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type SearchOptions struct {
	UserID       int
	Query        string
	IncludeTrash bool
	Limit        int
	Offset       int
	// CandidateIDs restricts search to IDs already vetted by the caller.
	CandidateIDs []uuid.UUID
}

func Open(path string) (*Service, error) {
	idx, err := bleve.Open(path)
	if err == nil {
		return &Service{index: idx, path: path}, nil
	}
	if err != bleve.ErrorIndexPathDoesNotExist && err != bleve.ErrorIndexMetaMissing {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	if err == bleve.ErrorIndexMetaMissing {
		if err := os.RemoveAll(path); err != nil {
			return nil, err
		}
	}
	idx, err = bleve.New(path, newMapping())
	if err != nil {
		return nil, err
	}
	return &Service{index: idx, path: path}, nil
}

func NewMemory() (*Service, error) {
	idx, err := bleve.NewMemOnly(newMapping())
	if err != nil {
		return nil, err
	}
	return &Service{index: idx, inMemory: true}, nil
}

func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index == nil {
		return nil
	}
	err := s.index.Close()
	s.index = nil
	return err
}

func (s *Service) Rebuild(ctx context.Context, client *ent.Client) error {
	rows, err := client.Note.Query().All(ctx)
	if err != nil {
		return err
	}

	var old bleve.Index
	if !s.inMemory {
		s.mu.Lock()
		old = s.index
		s.index = nil
		s.mu.Unlock()
		if old != nil {
			_ = old.Close()
			old = nil
		}
	}

	idx, err := s.newEmptyIndex()
	if err != nil {
		return err
	}

	s.mu.Lock()
	old = s.index
	s.index = idx
	s.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}

	for _, row := range rows {
		if err := s.IndexNote(ctx, row); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) IndexNote(ctx context.Context, row *ent.Note) error {
	doc, err := documentFromNote(ctx, row)
	if err != nil {
		return err
	}
	return s.IndexDocument(doc)
}

func (s *Service) IndexDocument(doc Document) error {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx == nil {
		return errClosed
	}
	return idx.Index(doc.ID, doc)
}

func (s *Service) DeleteNote(id uuid.UUID) error {
	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx == nil {
		return errClosed
	}
	return idx.Delete(id.String())
}

func (s *Service) Search(ctx context.Context, opts SearchOptions) ([]uuid.UUID, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	req := bleve.NewSearchRequestOptions(searchQuery(opts), limit, offset, false)
	req.Fields = []string{"id"}

	s.mu.RLock()
	idx := s.index
	s.mu.RUnlock()
	if idx == nil {
		return nil, errClosed
	}

	result, err := idx.SearchInContext(ctx, req)
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(result.Hits))
	for _, hit := range result.Hits {
		id, err := uuid.Parse(hit.ID)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Service) newEmptyIndex() (bleve.Index, error) {
	if s.inMemory {
		return bleve.NewMemOnly(newMapping())
	}

	if err := os.RemoveAll(s.path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return nil, err
	}
	return bleve.New(s.path, newMapping())
}

func newMapping() *mapping.IndexMappingImpl {
	doc := bleve.NewDocumentMapping()
	doc.Dynamic = false

	doc.AddFieldMappingsAt("id", bleve.NewKeywordFieldMapping())
	doc.AddFieldMappingsAt("user_id", bleve.NewNumericFieldMapping())
	doc.AddFieldMappingsAt("title", textFieldMapping())
	doc.AddFieldMappingsAt("content", textFieldMapping())
	doc.AddFieldMappingsAt("tags", textFieldMapping())
	doc.AddFieldMappingsAt("folder_id", bleve.NewKeywordFieldMapping())
	doc.AddFieldMappingsAt("protection_mode", bleve.NewKeywordFieldMapping())
	doc.AddFieldMappingsAt("is_deleted", bleve.NewBooleanFieldMapping())
	doc.AddFieldMappingsAt("created_at", bleve.NewDateTimeFieldMapping())
	doc.AddFieldMappingsAt("updated_at", bleve.NewDateTimeFieldMapping())

	mapping := bleve.NewIndexMapping()
	mapping.DefaultMapping = doc
	mapping.DefaultField = "content"
	return mapping
}

func textFieldMapping() *mapping.FieldMapping {
	field := bleve.NewTextFieldMapping()
	field.Analyzer = cjk.AnalyzerName
	return field
}

func documentFromNote(ctx context.Context, row *ent.Note) (Document, error) {
	owner, err := row.QueryUser().Only(ctx)
	if err != nil {
		return Document{}, err
	}

	tagRows, err := row.QueryTags().Order(ent.Asc(tag.FieldName)).All(ctx)
	if err != nil {
		return Document{}, err
	}
	tags := make([]string, 0, len(tagRows))
	for _, tagRow := range tagRows {
		tags = append(tags, tagRow.Name)
	}

	folderID := ""
	folderRow, err := row.QueryFolder().Only(ctx)
	if ent.IsNotFound(err) {
		err = nil
	}
	if err != nil {
		return Document{}, err
	}
	if folderRow != nil {
		folderID = folderRow.ID.String()
	}

	content := row.Content
	if row.ProtectionMode == note.ProtectionModeEncrypted {
		content = ""
	}

	return Document{
		ID:             row.ID.String(),
		UserID:         owner.ID,
		Title:          row.Title,
		Content:        content,
		Tags:           tags,
		FolderID:       folderID,
		ProtectionMode: string(row.ProtectionMode),
		IsDeleted:      row.IsDeleted,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}, nil
}

func searchQuery(opts SearchOptions) query.Query {
	var parts []query.Query
	if len(opts.CandidateIDs) > 0 {
		parts = append(parts, candidateIDsQuery(opts.CandidateIDs))
	} else {
		parts = append(parts, userIDQuery(opts.UserID))
	}
	if len(opts.CandidateIDs) == 0 && !opts.IncludeTrash {
		deleted := bleve.NewBoolFieldQuery(false)
		deleted.SetField("is_deleted")
		parts = append(parts, deleted)
	}

	if q := strings.TrimSpace(opts.Query); q != "" {
		title := bleve.NewMatchQuery(q)
		title.SetField("title")
		content := bleve.NewMatchQuery(q)
		content.SetField("content")
		parts = append(parts, bleve.NewDisjunctionQuery(title, content))
	}
	return bleve.NewConjunctionQuery(parts...)
}

func candidateIDsQuery(ids []uuid.UUID) query.Query {
	docIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		docIDs = append(docIDs, id.String())
	}
	return bleve.NewDocIDQuery(docIDs)
}

func userIDQuery(userID int) query.Query {
	value := float64(userID)
	inclusive := true
	q := bleve.NewNumericRangeInclusiveQuery(&value, &value, &inclusive, &inclusive)
	q.SetField("user_id")
	return q
}

func IDRank(ids []uuid.UUID) map[uuid.UUID]int {
	rank := make(map[uuid.UUID]int, len(ids))
	for i, id := range ids {
		rank[id] = i
	}
	return rank
}

func CandidateLimit(limit, offset int) int {
	if limit <= 0 {
		limit = defaultLimit
	}
	if offset < 0 {
		offset = 0
	}
	n := limit + offset
	if n < defaultLimit {
		return defaultLimit
	}
	if n > 10000 {
		return 10000
	}
	return n
}
