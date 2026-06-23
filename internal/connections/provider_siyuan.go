package connections

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/lib-x/siyuan"
)

type siYuanProvider struct {
	client *siyuan.Client
}

type siYuanFrontMatter struct {
	Title     string
	Content   string
	Tags      []string
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

var siYuanFrontMatterListPattern = regexp.MustCompile(`[\[\],]`)
var siYuanLooseMetaKeyPattern = regexp.MustCompile(`(?i)(^|\s)(title|date|created|created_at|lastmod|updated|updated_at|tags|tag)\s*:\s*`)

const siYuanDocTargetPrefix = "doc:"

type siYuanImportTarget struct {
	Box   string
	HPath string
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
	notebookNames := make(map[string]string, len(notebooks))
	for _, notebook := range notebooks {
		box := string(notebook.ID)
		notebookNames[box] = notebook.Name
		targets = append(targets, Target{
			ID:   box,
			Name: notebook.Name,
			Kind: "notebook",
		})
	}
	rows, err := p.client.SQL.Query(
		ctx,
		"SELECT id, content, hpath, box, path FROM blocks WHERE type = 'd' AND hpath <> '' ORDER BY box, hpath LIMIT 1000",
	)
	if err != nil {
		return targets, nil
	}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		box := sqlString(row, "box")
		hpath := normalizeSiYuanHPath(sqlString(row, "hpath"))
		if box == "" || hpath == "" {
			continue
		}
		id := encodeSiYuanDocTargetID(box, hpath)
		if seen[id] {
			continue
		}
		seen[id] = true
		name := strings.Trim(hpath, "/")
		if name == "" {
			name = strings.TrimSpace(sqlString(row, "content"))
		}
		if name == "" {
			name = sqlString(row, "id")
		}
		if notebookName := notebookNames[box]; notebookName != "" && name != "" {
			name = notebookName + " / " + name
		}
		targets = append(targets, Target{
			ID:       id,
			Name:     name,
			Kind:     "document",
			ParentID: box,
		})
	}
	return targets, nil
}

func (p *siYuanProvider) ImportNotes(ctx context.Context, targetID string, limit int) ([]RemoteNote, error) {
	limit = clampLimit(limit)
	target := decodeSiYuanImportTargetID(targetID)
	where := "type = 'd'"
	if target.Box != "" {
		where += fmt.Sprintf(" AND box = '%s'", escapeSQLString(target.Box))
	}
	if target.HPath != "" {
		where += fmt.Sprintf(
			" AND (hpath = '%s' OR hpath LIKE '%s' ESCAPE '\\')",
			escapeSQLString(target.HPath),
			escapeSQLString(escapeSQLLike(target.HPath)+"/%"),
		)
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
		metadata := parseSiYuanFrontMatter(content)
		content = metadata.Content
		hpath := sqlString(row, "hpath")
		title := sqlString(row, "content")
		if metadata.Title != "" {
			title = metadata.Title
		}
		box := sqlString(row, "box")
		createdAt := parseSiYuanTime(sqlString(row, "created"))
		if metadata.CreatedAt != nil {
			createdAt = metadata.CreatedAt
		}
		updatedAt := parseSiYuanTime(sqlString(row, "updated"))
		if metadata.UpdatedAt != nil {
			updatedAt = metadata.UpdatedAt
		}
		tags := metadata.Tags
		if attrs, err := p.client.Attributes.GetBlockAttrs(ctx, siyuan.BlockID(id)); err == nil {
			if attrTitle := strings.TrimSpace(attrs["title"]); attrTitle != "" && strings.TrimSpace(title) == "" {
				title = attrTitle
			}
			tags = append(tags, siYuanAttrTags(attrs)...)
		}
		if strings.TrimSpace(title) == "" {
			title = path.Base(strings.Trim(hpath, "/"))
		}
		notes = append(notes, RemoteNote{
			ExternalID: id,
			TargetID:   box,
			TargetName: targetNames[box],
			Path:       hpath,
			Title:      titleOrUntitled(title),
			Content:    content,
			Tags:       uniqueStrings(tags),
			CreatedAt:  createdAt,
			UpdatedAt:  updatedAt,
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

func encodeSiYuanDocTargetID(box, hpath string) string {
	box = strings.TrimSpace(box)
	hpath = normalizeSiYuanHPath(hpath)
	if box == "" || hpath == "" {
		return box
	}
	return siYuanDocTargetPrefix + box + ":" + url.PathEscape(hpath)
}

func decodeSiYuanImportTargetID(targetID string) siYuanImportTarget {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return siYuanImportTarget{}
	}
	if !strings.HasPrefix(targetID, siYuanDocTargetPrefix) {
		return siYuanImportTarget{Box: targetID}
	}
	payload := strings.TrimPrefix(targetID, siYuanDocTargetPrefix)
	box, encodedHPath, ok := strings.Cut(payload, ":")
	if !ok {
		return siYuanImportTarget{Box: targetID}
	}
	hpath, err := url.PathUnescape(encodedHPath)
	if err != nil {
		return siYuanImportTarget{Box: strings.TrimSpace(box)}
	}
	return siYuanImportTarget{
		Box:   strings.TrimSpace(box),
		HPath: normalizeSiYuanHPath(hpath),
	}
}

func normalizeSiYuanHPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if value != "/" {
		value = strings.TrimRight(value, "/")
	}
	return value
}

func escapeSQLString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func escapeSQLLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
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

func parseSiYuanFrontMatter(content string) siYuanFrontMatter {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	trimmed := strings.TrimLeft(normalized, "\ufeff\n\t ")
	delimiter := ""
	switch {
	case strings.HasPrefix(trimmed, "---\n"):
		delimiter = "---"
	case strings.HasPrefix(trimmed, "***\n"):
		delimiter = "***"
	default:
		if meta, ok := parseSiYuanLooseFrontMatter(trimmed, content); ok {
			return meta
		}
		return siYuanFrontMatter{Content: content}
	}

	lines := strings.Split(trimmed, "\n")
	end := -1
	for i := 1; i < len(lines) && i < 80; i++ {
		if isSiYuanFrontMatterDelimiter(strings.TrimSpace(lines[i]), delimiter) {
			end = i
			break
		}
	}
	if end == -1 {
		return siYuanFrontMatter{Content: content}
	}

	meta := siYuanFrontMatter{Content: strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\n")}
	for _, line := range lines[1:end] {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		switch key {
		case "title":
			meta.Title = value
		case "date", "created", "created_at":
			meta.CreatedAt = parseSiYuanMetadataTime(value)
		case "lastmod", "updated", "updated_at":
			meta.UpdatedAt = parseSiYuanMetadataTime(value)
		case "tags", "tag":
			meta.Tags = append(meta.Tags, parseSiYuanTagList(value)...)
		}
	}
	return meta
}

func isSiYuanFrontMatterDelimiter(line, opener string) bool {
	return line == opener || line == "---" || line == "***"
}

func parseSiYuanLooseFrontMatter(trimmed, original string) (siYuanFrontMatter, bool) {
	lines := strings.Split(trimmed, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			start = i
			break
		}
	}
	if start == -1 {
		return siYuanFrontMatter{Content: original}, false
	}

	line := strings.TrimSpace(lines[start])
	line = strings.TrimSpace(strings.TrimLeft(line, "*- "))
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "title:") &&
		!strings.HasPrefix(lower, "date:") &&
		!strings.HasPrefix(lower, "created:") &&
		!strings.HasPrefix(lower, "created_at:") &&
		!strings.HasPrefix(lower, "lastmod:") &&
		!strings.HasPrefix(lower, "updated:") &&
		!strings.HasPrefix(lower, "updated_at:") &&
		!strings.HasPrefix(lower, "tags:") &&
		!strings.HasPrefix(lower, "tag:") {
		return siYuanFrontMatter{Content: original}, false
	}

	meta := siYuanFrontMatter{}
	applySiYuanMetadataPairs(line, &meta)
	if meta.Title == "" && meta.CreatedAt == nil && meta.UpdatedAt == nil && len(meta.Tags) == 0 {
		return siYuanFrontMatter{Content: original}, false
	}

	next := start + 1
	for next < len(lines) {
		value := strings.TrimSpace(lines[next])
		if value == "---" || value == "***" || value == "" {
			next++
			continue
		}
		break
	}
	meta.Content = strings.TrimLeft(strings.Join(lines[next:], "\n"), "\n")
	return meta, true
}

func applySiYuanMetadataPairs(line string, meta *siYuanFrontMatter) {
	matches := siYuanLooseMetaKeyPattern.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return
	}
	for i, match := range matches {
		key := strings.ToLower(strings.TrimSpace(line[match[4]:match[5]]))
		valueStart := match[1]
		valueEnd := len(line)
		if i+1 < len(matches) {
			valueEnd = matches[i+1][0]
		}
		value := cleanSiYuanMetadataValue(line[valueStart:valueEnd])
		switch key {
		case "title":
			meta.Title = value
		case "date", "created", "created_at":
			meta.CreatedAt = parseSiYuanMetadataTime(value)
		case "lastmod", "updated", "updated_at":
			meta.UpdatedAt = parseSiYuanMetadataTime(value)
		case "tags", "tag":
			meta.Tags = append(meta.Tags, parseSiYuanTagList(value)...)
		}
	}
}

func cleanSiYuanMetadataValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "---")
	value = strings.TrimSuffix(value, "***")
	return strings.Trim(strings.TrimSpace(value), `"'`)
}

func parseSiYuanMetadataTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"20060102150405",
	}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, time.Local)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func siYuanAttrTags(attrs map[string]string) []string {
	var tags []string
	for _, key := range []string{"tags", "tag", "custom-tags", "custom-tag"} {
		tags = append(tags, parseSiYuanTagList(attrs[key])...)
	}
	return tags
}

func parseSiYuanTagList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	value = strings.Trim(value, `"'`)
	value = siYuanFrontMatterListPattern.ReplaceAllString(value, " ")
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ';' || r == '，' || r == '、' || r == '#'
	})
	tags := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.Trim(strings.TrimSpace(field), `"'`)
		if field != "" {
			tags = append(tags, field)
		}
	}
	return tags
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
