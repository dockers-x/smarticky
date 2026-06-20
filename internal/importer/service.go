package importer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/importitem"
	"smarticky/ent/importjob"
	"smarticky/ent/note"
	"smarticky/ent/tag"
	"smarticky/ent/user"
	"smarticky/internal/importer/evernote"
	"smarticky/internal/storage"

	"github.com/google/uuid"
)

const maxENEXBytes = 50 << 20

var ErrImportTooLarge = errors.New("import file too large")

type Service struct {
	client *ent.Client
	fs     *storage.FileSystem
}

type PreviewResult struct {
	Job   *ent.ImportJob    `json:"job"`
	Items []*ent.ImportItem `json:"items"`
}

type ImportResult struct {
	Job      *ent.ImportJob `json:"job"`
	Imported int            `json:"imported"`
	Skipped  int            `json:"skipped"`
	Failed   int            `json:"failed"`
}

type jobOptions struct {
	ENEXPath string `json:"enex_path"`
}

func NewService(client *ent.Client, fs *storage.FileSystem) *Service {
	if fs == nil {
		fs = storage.NewFileSystem("")
	}
	return &Service{client: client, fs: fs}
}

func (s *Service) PreviewEvernote(ctx context.Context, userID int, filename string, r io.Reader) (*PreviewResult, error) {
	data, err := readLimited(r, maxENEXBytes)
	if err != nil {
		return nil, err
	}

	doc, err := evernote.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}

	txClient := tx.Client()
	var enexPath string
	fileWritten := false
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
		if fileWritten {
			_ = s.fs.Remove(enexPath)
		}
	}()

	job, err := txClient.ImportJob.Create().
		SetSource("evernote").
		SetFilename(cleanFilename(filename)).
		SetStatus("previewed").
		SetNoteCount(len(doc.Notes)).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	enexPath = filepath.Join(s.fs.GetDataDir(), "imports", "evernote", fmt.Sprintf("%d.enex", job.ID))
	if err := s.fs.WriteFile(enexPath, data, 0600); err != nil {
		return nil, fmt.Errorf("save enex: %w", err)
	}
	fileWritten = true

	optionsJSON, err := json.Marshal(jobOptions{ENEXPath: enexPath})
	if err != nil {
		return nil, fmt.Errorf("encode import options: %w", err)
	}
	job, err = job.Update().SetOptionsJSON(string(optionsJSON)).Save(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*ent.ImportItem, 0, len(doc.Notes))
	for _, parsedNote := range doc.Notes {
		item, err := txClient.ImportItem.Create().
			SetJobID(job.ID).
			SetSourceNoteKey(sourceNoteKey(parsedNote)).
			SetTitle(titleOrUntitled(parsedNote.Title)).
			SetStatus("pending").
			Save(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	jobID := job.ID
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	committed = true

	job, err = s.client.ImportJob.Query().
		Where(importjob.ID(jobID)).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	items, err = job.QueryItems().All(ctx)
	if err != nil {
		return nil, err
	}

	return &PreviewResult{Job: job, Items: items}, nil
}

func (s *Service) ConfirmEvernote(ctx context.Context, userID int, jobID int) (*ImportResult, error) {
	job, err := s.client.ImportJob.Query().
		Where(importjob.ID(jobID), importjob.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if job.Status == "completed" {
		return &ImportResult{
			Job:      job,
			Imported: job.ImportedCount,
			Skipped:  job.SkippedCount,
			Failed:   job.FailedCount,
		}, nil
	}

	options, err := decodeJobOptions(job.OptionsJSON)
	if err != nil {
		return nil, err
	}
	rawENEX, err := s.fs.ReadFile(options.ENEXPath)
	if err != nil {
		return nil, fmt.Errorf("read enex: %w", err)
	}
	doc, err := evernote.Parse(bytes.NewReader(rawENEX))
	if err != nil {
		return nil, err
	}

	pendingItems, err := job.QueryItems().
		Where(importitem.StatusEQ("pending")).
		All(ctx)
	if err != nil {
		return nil, err
	}

	notesByKey := make(map[string]evernote.Note, len(doc.Notes))
	for _, parsedNote := range doc.Notes {
		notesByKey[sourceNoteKey(parsedNote)] = parsedNote
	}

	existingKeys, err := s.existingNoteKeys(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{}
	for _, item := range pendingItems {
		parsedNote, ok := notesByKey[item.SourceNoteKey]
		if !ok {
			result.Failed++
			_ = item.Update().SetStatus("failed").SetMessage("source note missing").Exec(ctx)
			continue
		}
		if existingKeys[item.SourceNoteKey] {
			result.Skipped++
			_ = item.Update().SetStatus("skipped").SetMessage("duplicate note").Exec(ctx)
			continue
		}

		createdNote, message, err := s.importNote(ctx, userID, parsedNote)
		if err != nil {
			result.Failed++
			_ = item.Update().SetStatus("failed").SetMessage("failed to import note").Exec(ctx)
			continue
		}

		existingKeys[item.SourceNoteKey] = true
		result.Imported++
		update := item.Update().SetStatus("imported").SetNoteID(createdNote.ID)
		if message != "" {
			update.SetMessage(message)
		}
		if err := update.Exec(ctx); err != nil {
			return nil, err
		}
	}

	job, err = job.Update().
		SetStatus("completed").
		SetImportedCount(result.Imported).
		SetSkippedCount(result.Skipped).
		SetFailedCount(result.Failed).
		SetCompletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	result.Job = job
	return result, nil
}

func (s *Service) existingNoteKeys(ctx context.Context, userID int) (map[string]bool, error) {
	existingNotes, err := s.client.Note.Query().
		Where(note.HasUserWith(user.IDEQ(userID))).
		All(ctx)
	if err != nil {
		return nil, err
	}

	keys := make(map[string]bool, len(existingNotes))
	for _, existing := range existingNotes {
		keys[duplicateKey(existing.Title, existing.CreatedAt, existing.Content)] = true
	}
	return keys, nil
}

func (s *Service) importNote(ctx context.Context, userID int, parsedNote evernote.Note) (*ent.Note, string, error) {
	create := s.client.Note.Create().
		SetTitle(titleOrUntitled(parsedNote.Title)).
		SetContent(importedContent(parsedNote)).
		SetUserID(userID)
	if !parsedNote.Created.IsZero() {
		create.SetCreatedAt(parsedNote.Created)
	}
	if !parsedNote.Updated.IsZero() {
		create.SetUpdatedAt(parsedNote.Updated)
	}

	createdNote, err := create.Save(ctx)
	if err != nil {
		return nil, "", err
	}

	for _, tagName := range parsedNote.Tags {
		t, err := s.findOrCreateTag(ctx, userID, tagName)
		if err != nil {
			return nil, "", fmt.Errorf("tag %q: %w", tagName, err)
		}
		if err := createdNote.Update().AddTags(t).Exec(ctx); err != nil {
			return nil, "", err
		}
	}

	skippedResources := 0
	for index, resource := range parsedNote.Resources {
		if resource.DecodeError != "" || len(resource.Data) == 0 {
			skippedResources++
			continue
		}
		if err := s.saveResource(ctx, userID, createdNote.ID, index, resource); err != nil {
			return nil, "", err
		}
	}
	if skippedResources > 0 {
		return createdNote, fmt.Sprintf("%d resource(s) skipped", skippedResources), nil
	}
	return createdNote, "", nil
}

func (s *Service) findOrCreateTag(ctx context.Context, userID int, name string) (*ent.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("empty tag")
	}

	existing, err := s.client.Tag.Query().
		Where(tag.NameEQ(name), tag.HasUserWith(user.IDEQ(userID))).
		Only(ctx)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}

	return s.client.Tag.Create().
		SetName(name).
		SetColor("#E8450A").
		SetUserID(userID).
		Save(ctx)
}

func (s *Service) saveResource(ctx context.Context, userID int, noteID uuid.UUID, index int, resource evernote.Resource) error {
	filename := cleanFilename(resource.FileName)
	if filename == "import.enex" {
		filename = fmt.Sprintf("resource-%d", index+1)
	}
	ext := filepath.Ext(filename)
	storedName := uuid.New().String() + ext
	filePath := filepath.Join(s.fs.GetUploadsDir("attachments"), storedName)

	if err := s.fs.WriteFile(filePath, resource.Data, 0644); err != nil {
		return err
	}

	_, err := s.client.Attachment.Create().
		SetFilename(filename).
		SetFilePath(filePath).
		SetFileSize(int64(len(resource.Data))).
		SetMimeType(resource.MIME).
		SetNoteID(noteID).
		SetUserID(userID).
		Save(ctx)
	return err
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: max %d bytes", ErrImportTooLarge, limit)
	}
	return data, nil
}

func decodeJobOptions(raw string) (jobOptions, error) {
	var options jobOptions
	if err := json.Unmarshal([]byte(raw), &options); err != nil {
		return options, fmt.Errorf("decode import options: %w", err)
	}
	if options.ENEXPath == "" {
		return options, errors.New("missing enex path")
	}
	return options, nil
}

func cleanFilename(filename string) string {
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return "import.enex"
	}
	return filename
}

func titleOrUntitled(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled"
	}
	return title
}

func duplicateKey(title string, created time.Time, content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return strings.ToLower(strings.TrimSpace(title)) + "|" + created.UTC().Format(time.RFC3339) + "|" + hex.EncodeToString(sum[:])
}

func sourceNoteKey(parsedNote evernote.Note) string {
	return duplicateKey(parsedNote.Title, parsedNote.Created, importedContent(parsedNote))
}

func importedContent(parsedNote evernote.Note) string {
	return enmlToText(parsedNote.Content)
}

var (
	xmlDeclarationPattern = regexp.MustCompile(`(?is)<\?xml.*?\?>`)
	doctypePattern        = regexp.MustCompile(`(?is)<!DOCTYPE.*?>`)
)

func enmlToText(content string) string {
	content = xmlDeclarationPattern.ReplaceAllString(content, "")
	content = doctypePattern.ReplaceAllString(content, "")

	decoder := xml.NewDecoder(strings.NewReader(content))
	var builder strings.Builder
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return strings.TrimSpace(content)
		}

		switch t := token.(type) {
		case xml.CharData:
			builder.Write([]byte(t))
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "br":
				builder.WriteString("\n")
			case "div", "p":
				writeNewlineIfNeeded(&builder)
			case "en-todo":
				writeNewlineIfNeeded(&builder)
				builder.WriteString("- [ ] ")
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "div", "p":
				writeNewlineIfNeeded(&builder)
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func writeNewlineIfNeeded(builder *strings.Builder) {
	value := builder.String()
	if value != "" && !strings.HasSuffix(value, "\n") {
		builder.WriteString("\n")
	}
}
