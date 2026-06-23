package connections

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jomei/notionapi"
)

type notionProvider struct {
	client *notionapi.Client
}

func newNotionProvider(token string, httpClient *http.Client) Provider {
	return &notionProvider{
		client: notionapi.NewClient(notionapi.Token(token), notionapi.WithHTTPClient(httpClient)),
	}
}

func (p *notionProvider) Test(ctx context.Context) error {
	_, err := p.client.User.Me(ctx)
	return err
}

func (p *notionProvider) ListTargets(ctx context.Context) ([]Target, error) {
	var targets []Target
	for _, objectType := range []notionapi.ObjectType{notionapi.ObjectTypePage, notionapi.ObjectTypeDatabase} {
		resp, err := p.client.Search.Do(ctx, &notionapi.SearchRequest{
			Filter:   notionapi.SearchFilter{Property: "object", Value: objectType.String()},
			PageSize: 50,
		})
		if err != nil {
			return nil, err
		}
		for _, object := range resp.Results {
			switch item := object.(type) {
			case *notionapi.Page:
				targets = append(targets, Target{
					ID:   notionTargetID("page", string(item.ID)),
					Name: titleOrUntitled(notionPageTitle(item)),
					Kind: "page",
				})
			case *notionapi.Database:
				targets = append(targets, Target{
					ID:   notionTargetID("database", string(item.ID)),
					Name: titleOrUntitled(richTextPlain(item.Title)),
					Kind: "database",
				})
			}
		}
	}
	return targets, nil
}

func (p *notionProvider) ImportNotes(ctx context.Context, targetID string, limit int) ([]RemoteNote, error) {
	kind, id := parseNotionTargetID(targetID)
	if id == "" {
		return nil, ErrMissingTarget
	}
	limit = clampLimit(limit)
	switch kind {
	case "page":
		page, err := p.client.Page.Get(ctx, notionapi.PageID(id))
		if err != nil {
			return nil, err
		}
		content, err := p.pageMarkdown(ctx, notionapi.BlockID(id), 0)
		if err != nil {
			return nil, err
		}
		return []RemoteNote{{
			ExternalID: string(page.ID),
			TargetID:   targetID,
			TargetName: titleOrUntitled(notionPageTitle(page)),
			Title:      titleOrUntitled(notionPageTitle(page)),
			Content:    content,
			CreatedAt:  &page.CreatedTime,
			UpdatedAt:  &page.LastEditedTime,
			URL:        page.URL,
		}}, nil
	case "database":
		return p.importDatabasePages(ctx, id, targetID, limit)
	default:
		return nil, ErrMissingTarget
	}
}

func (p *notionProvider) importDatabasePages(ctx context.Context, databaseID, targetID string, limit int) ([]RemoteNote, error) {
	var notes []RemoteNote
	var cursor notionapi.Cursor
	for len(notes) < limit {
		resp, err := p.client.Database.Query(ctx, notionapi.DatabaseID(databaseID), &notionapi.DatabaseQueryRequest{
			StartCursor: cursor,
			PageSize:    minInt(100, limit-len(notes)),
		})
		if err != nil {
			return nil, err
		}
		for _, page := range resp.Results {
			content, err := p.pageMarkdown(ctx, notionapi.BlockID(page.ID), 0)
			if err != nil {
				content = ""
			}
			notes = append(notes, RemoteNote{
				ExternalID: string(page.ID),
				TargetID:   targetID,
				Title:      titleOrUntitled(notionPageTitle(&page)),
				Content:    content,
				CreatedAt:  &page.CreatedTime,
				UpdatedAt:  &page.LastEditedTime,
				URL:        page.URL,
			})
			if len(notes) >= limit {
				break
			}
		}
		if !resp.HasMore {
			break
		}
		cursor = resp.NextCursor
	}
	return notes, nil
}

func (p *notionProvider) PushNote(ctx context.Context, input PushInput) (PushResult, error) {
	if input.ExistingExternalID != "" {
		if err := p.replacePageChildren(ctx, notionapi.BlockID(input.ExistingExternalID), input.Content); err != nil {
			return PushResult{}, err
		}
		_, _ = p.client.Page.Update(ctx, notionapi.PageID(input.ExistingExternalID), &notionapi.PageUpdateRequest{
			Properties: notionapi.Properties{
				"title": notionapi.TitleProperty{Title: notionRichText(input.Title)},
			},
		})
		return PushResult{
			ExternalID: input.ExistingExternalID,
			TargetID:   input.TargetID,
		}, nil
	}

	kind, id := parseNotionTargetID(input.TargetID)
	if id == "" {
		return PushResult{}, ErrMissingTarget
	}
	parent := notionapi.Parent{}
	properties := notionapi.Properties{}
	switch kind {
	case "page":
		parent = notionapi.Parent{Type: notionapi.ParentTypePageID, PageID: notionapi.PageID(id)}
		properties["title"] = notionapi.TitleProperty{Title: notionRichText(input.Title)}
	case "database":
		db, err := p.client.Database.Get(ctx, notionapi.DatabaseID(id))
		if err != nil {
			return PushResult{}, err
		}
		titleProperty := notionDatabaseTitleProperty(db)
		if titleProperty == "" {
			return PushResult{}, fmt.Errorf("notion database has no title property")
		}
		parent = notionapi.Parent{Type: notionapi.ParentTypeDatabaseID, DatabaseID: notionapi.DatabaseID(id)}
		properties[titleProperty] = notionapi.TitleProperty{Title: notionRichText(input.Title)}
	default:
		return PushResult{}, ErrMissingTarget
	}

	page, err := p.client.Page.Create(ctx, &notionapi.PageCreateRequest{
		Parent:     parent,
		Properties: properties,
		Children:   markdownToNotionBlocks(input.Content),
	})
	if err != nil {
		return PushResult{}, err
	}
	return PushResult{
		ExternalID: string(page.ID),
		TargetID:   input.TargetID,
		URL:        page.URL,
	}, nil
}

func (p *notionProvider) replacePageChildren(ctx context.Context, pageID notionapi.BlockID, markdown string) error {
	var cursor notionapi.Cursor
	for {
		resp, err := p.client.Block.GetChildren(ctx, pageID, &notionapi.Pagination{StartCursor: cursor, PageSize: 100})
		if err != nil {
			return err
		}
		for _, block := range resp.Results {
			if _, err := p.client.Block.Delete(ctx, block.GetID()); err != nil {
				return err
			}
		}
		if !resp.HasMore {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}
	blocks := markdownToNotionBlocks(markdown)
	for start := 0; start < len(blocks); start += 100 {
		end := start + 100
		if end > len(blocks) {
			end = len(blocks)
		}
		if _, err := p.client.Block.AppendChildren(ctx, pageID, &notionapi.AppendBlockChildrenRequest{Children: blocks[start:end]}); err != nil {
			return err
		}
	}
	return nil
}

func (p *notionProvider) pageMarkdown(ctx context.Context, blockID notionapi.BlockID, depth int) (string, error) {
	if depth > 4 {
		return "", nil
	}
	var builder strings.Builder
	var cursor notionapi.Cursor
	for {
		resp, err := p.client.Block.GetChildren(ctx, blockID, &notionapi.Pagination{StartCursor: cursor, PageSize: 100})
		if err != nil {
			return "", err
		}
		for _, block := range resp.Results {
			builder.WriteString(notionBlockMarkdown(block))
			if block.GetHasChildren() {
				child, err := p.pageMarkdown(ctx, block.GetID(), depth+1)
				if err == nil && strings.TrimSpace(child) != "" {
					builder.WriteString(child)
				}
			}
		}
		if !resp.HasMore {
			break
		}
		cursor = notionapi.Cursor(resp.NextCursor)
	}
	return strings.TrimSpace(builder.String()), nil
}

func notionBlockMarkdown(block notionapi.Block) string {
	switch typed := block.(type) {
	case *notionapi.ParagraphBlock:
		return richTextPlain(typed.Paragraph.RichText) + "\n\n"
	case notionapi.ParagraphBlock:
		return richTextPlain(typed.Paragraph.RichText) + "\n\n"
	case *notionapi.Heading1Block:
		return "# " + richTextPlain(typed.Heading1.RichText) + "\n\n"
	case notionapi.Heading1Block:
		return "# " + richTextPlain(typed.Heading1.RichText) + "\n\n"
	case *notionapi.Heading2Block:
		return "## " + richTextPlain(typed.Heading2.RichText) + "\n\n"
	case notionapi.Heading2Block:
		return "## " + richTextPlain(typed.Heading2.RichText) + "\n\n"
	case *notionapi.Heading3Block:
		return "### " + richTextPlain(typed.Heading3.RichText) + "\n\n"
	case notionapi.Heading3Block:
		return "### " + richTextPlain(typed.Heading3.RichText) + "\n\n"
	case *notionapi.BulletedListItemBlock:
		return "- " + richTextPlain(typed.BulletedListItem.RichText) + "\n"
	case notionapi.BulletedListItemBlock:
		return "- " + richTextPlain(typed.BulletedListItem.RichText) + "\n"
	case *notionapi.NumberedListItemBlock:
		return "1. " + richTextPlain(typed.NumberedListItem.RichText) + "\n"
	case notionapi.NumberedListItemBlock:
		return "1. " + richTextPlain(typed.NumberedListItem.RichText) + "\n"
	case *notionapi.ToDoBlock:
		return "- [" + checkboxMark(typed.ToDo.Checked) + "] " + richTextPlain(typed.ToDo.RichText) + "\n"
	case notionapi.ToDoBlock:
		return "- [" + checkboxMark(typed.ToDo.Checked) + "] " + richTextPlain(typed.ToDo.RichText) + "\n"
	case *notionapi.QuoteBlock:
		return "> " + richTextPlain(typed.Quote.RichText) + "\n\n"
	case notionapi.QuoteBlock:
		return "> " + richTextPlain(typed.Quote.RichText) + "\n\n"
	case *notionapi.CodeBlock:
		return "```" + typed.Code.Language + "\n" + richTextPlain(typed.Code.RichText) + "\n```\n\n"
	case notionapi.CodeBlock:
		return "```" + typed.Code.Language + "\n" + richTextPlain(typed.Code.RichText) + "\n```\n\n"
	case *notionapi.DividerBlock, notionapi.DividerBlock:
		return "---\n\n"
	case *notionapi.ChildPageBlock:
		return "## " + typed.ChildPage.Title + "\n\n"
	case notionapi.ChildPageBlock:
		return "## " + typed.ChildPage.Title + "\n\n"
	default:
		return ""
	}
}

func markdownToNotionBlocks(markdown string) []notionapi.Block {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	blocks := make([]notionapi.Block, 0, 32)
	var paragraph []string
	inCode := false
	codeLang := ""
	var codeLines []string

	flushParagraph := func() {
		text := strings.TrimSpace(strings.Join(paragraph, "\n"))
		paragraph = nil
		if text == "" {
			return
		}
		for _, chunk := range textChunks(text, 1900) {
			blocks = append(blocks, notionParagraph(chunk))
		}
	}
	flushCode := func() {
		blocks = append(blocks, notionapi.CodeBlock{
			BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeCode},
			Code: notionapi.Code{
				Language: defaultString(codeLang, "plain text"),
				RichText: notionRichText(strings.Join(codeLines, "\n")),
			},
		})
		codeLang = ""
		codeLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCode {
				flushCode()
				inCode = false
			} else {
				flushParagraph()
				inCode = true
				codeLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
			}
			continue
		}
		if inCode {
			codeLines = append(codeLines, line)
			continue
		}
		if trimmed == "" {
			flushParagraph()
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			flushParagraph()
			blocks = append(blocks, notionHeading(1, strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))))
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			flushParagraph()
			blocks = append(blocks, notionHeading(2, strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))))
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			flushParagraph()
			blocks = append(blocks, notionHeading(3, strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))))
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			flushParagraph()
			blocks = append(blocks, notionapi.BulletedListItemBlock{
				BasicBlock:       notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeBulletedListItem},
				BulletedListItem: notionapi.ListItem{RichText: notionRichText(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))},
			})
			continue
		}
		paragraph = append(paragraph, line)
		if len(blocks) >= 90 {
			break
		}
	}
	if inCode {
		flushCode()
	}
	flushParagraph()
	if len(blocks) == 0 {
		blocks = append(blocks, notionParagraph(""))
	}
	if len(blocks) > 100 {
		return blocks[:100]
	}
	return blocks
}

func notionParagraph(text string) notionapi.ParagraphBlock {
	return notionapi.ParagraphBlock{
		BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeParagraph},
		Paragraph:  notionapi.Paragraph{RichText: notionRichText(text)},
	}
}

func notionHeading(level int, text string) notionapi.Block {
	heading := notionapi.Heading{RichText: notionRichText(text)}
	switch level {
	case 1:
		return notionapi.Heading1Block{
			BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeHeading1},
			Heading1:   heading,
		}
	case 2:
		return notionapi.Heading2Block{
			BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeHeading2},
			Heading2:   heading,
		}
	default:
		return notionapi.Heading3Block{
			BasicBlock: notionapi.BasicBlock{Object: notionapi.ObjectTypeBlock, Type: notionapi.BlockTypeHeading3},
			Heading3:   heading,
		}
	}
}

func notionRichText(text string) []notionapi.RichText {
	return []notionapi.RichText{{
		Type: notionapi.ObjectTypeText,
		Text: &notionapi.Text{Content: text},
	}}
}

func richTextPlain(values []notionapi.RichText) string {
	var builder strings.Builder
	for _, value := range values {
		if value.PlainText != "" {
			builder.WriteString(value.PlainText)
			continue
		}
		if value.Text != nil {
			builder.WriteString(value.Text.Content)
		}
	}
	return builder.String()
}

func notionPageTitle(page *notionapi.Page) string {
	for _, property := range page.Properties {
		if title, ok := property.(notionapi.TitleProperty); ok {
			return richTextPlain(title.Title)
		}
		if title, ok := property.(*notionapi.TitleProperty); ok {
			return richTextPlain(title.Title)
		}
	}
	return ""
}

func notionDatabaseTitleProperty(db *notionapi.Database) string {
	for name, property := range db.Properties {
		if property.GetType() == notionapi.PropertyConfigTypeTitle {
			return name
		}
	}
	return ""
}

func notionTargetID(kind, id string) string {
	return kind + ":" + id
}

func parseNotionTargetID(value string) (string, string) {
	kind, id, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok {
		return "page", strings.TrimSpace(value)
	}
	return kind, id
}

func checkboxMark(checked bool) string {
	if checked {
		return "x"
	}
	return " "
}
