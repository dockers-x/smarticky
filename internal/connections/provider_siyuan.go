package connections

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/lib-x/siyuan"
)

type siYuanProvider struct {
	client *siyuan.Client
}

func newSiYuanProvider(endpoint, token string, httpClient *http.Client) (Provider, error) {
	client, err := siyuan.New(
		siyuan.WithEndpoint(endpoint),
		siyuan.WithToken(token),
		siyuan.WithHTTPClient(httpClient),
		siyuan.WithUserAgent("Smarticky"),
	)
	if err != nil {
		return nil, err
	}
	return &siYuanProvider{client: client}, nil
}

func (p *siYuanProvider) Test(ctx context.Context) error {
	_, err := p.client.System.Version(ctx)
	return err
}

func (p *siYuanProvider) ListTargets(ctx context.Context) ([]Target, error) {
	notebooks, err := p.client.Notebooks.List(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]Target, 0, len(notebooks))
	for _, notebook := range notebooks {
		targets = append(targets, Target{
			ID:   string(notebook.ID),
			Name: notebook.Name,
			Kind: "notebook",
		})
	}
	return targets, nil
}

func (p *siYuanProvider) ImportNotes(ctx context.Context, targetID string, limit int) ([]RemoteNote, error) {
	limit = clampLimit(limit)
	where := "type = 'd'"
	if strings.TrimSpace(targetID) != "" {
		where += fmt.Sprintf(" AND box = '%s'", escapeSQLString(targetID))
	}
	query := fmt.Sprintf(
		"SELECT id, content, markdown, hpath, box, path, created, updated FROM blocks WHERE %s ORDER BY updated DESC LIMIT %d",
		where,
		limit,
	)
	rows, err := p.client.SQL.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	targetNames := map[string]string{}
	if notebooks, err := p.client.Notebooks.List(ctx); err == nil {
		for _, notebook := range notebooks {
			targetNames[string(notebook.ID)] = notebook.Name
		}
	}

	notes := make([]RemoteNote, 0, len(rows))
	for _, row := range rows {
		id := sqlString(row, "id")
		if id == "" {
			continue
		}
		content := sqlString(row, "markdown")
		exported, err := p.client.Export.MarkdownContent(ctx, siyuan.BlockID(id))
		if err == nil && strings.TrimSpace(exported.Content) != "" {
			content = exported.Content
		}
		hpath := sqlString(row, "hpath")
		title := sqlString(row, "content")
		if strings.TrimSpace(title) == "" {
			title = path.Base(strings.Trim(hpath, "/"))
		}
		box := sqlString(row, "box")
		notes = append(notes, RemoteNote{
			ExternalID: id,
			TargetID:   box,
			TargetName: targetNames[box],
			Path:       hpath,
			Title:      titleOrUntitled(title),
			Content:    content,
			CreatedAt:  parseSiYuanTime(sqlString(row, "created")),
			UpdatedAt:  parseSiYuanTime(sqlString(row, "updated")),
		})
	}
	return notes, nil
}

func (p *siYuanProvider) PushNote(ctx context.Context, input PushInput) (PushResult, error) {
	targetID := strings.TrimSpace(input.TargetID)
	if targetID == "" {
		return PushResult{}, ErrMissingTarget
	}
	if input.ExistingExternalID != "" {
		_, err := p.client.Blocks.Update(ctx, siyuan.UpdateBlockRequest{
			DataType: siyuan.DataTypeMarkdown,
			Data:     input.Content,
			ID:       siyuan.BlockID(input.ExistingExternalID),
		})
		if err != nil {
			return PushResult{}, err
		}
		return PushResult{
			ExternalID: input.ExistingExternalID,
			TargetID:   targetID,
		}, nil
	}

	docPath := "/" + safeRemoteSegment(input.Title) + "-" + input.NoteID.String()[:8]
	id, err := p.client.Documents.CreateWithMarkdown(ctx, siyuan.CreateDocWithMarkdownRequest{
		Notebook: siyuan.NotebookID(targetID),
		Path:     docPath,
		Markdown: input.Content,
	})
	if err != nil {
		return PushResult{}, err
	}
	return PushResult{
		ExternalID: string(id),
		TargetID:   targetID,
		Path:       docPath,
	}, nil
}

func escapeSQLString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func sqlString(row siyuan.SQLRow, key string) string {
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case fmt.Stringer:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func parseSiYuanTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if len(value) < len("20060102150405") {
		return nil
	}
	parsed, err := time.ParseInLocation("20060102150405", value[:14], time.Local)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
