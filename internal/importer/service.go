package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/ent/folder"
	"smarticky/ent/importitem"
	"smarticky/ent/importjob"
	"smarticky/ent/note"
	"smarticky/ent/tag"
	"smarticky/ent/user"
	notesvc "smarticky/internal/notes"
	"smarticky/internal/storage"

	"github.com/google/uuid"
	enex "github.com/lib-x/enex"
)

const maxENEXBytes = 50 << 20

var ErrImportTooLarge = errors.New("import file too large")

type Service struct {
	client *ent.Client
	fs     *storage.FileSystem
}

type PreviewResult struct {
	Job           *ent.ImportJob    `json:"job"`
	Items         []*ent.ImportItem `json:"items"`
	JobID         int               `json:"job_id"`
	Filename      string            `json:"filename"`
	NoteCount     int               `json:"note_count"`
	NotebookCount int               `json:"notebook_count"`
	Notebooks     []ImportNotebook  `json:"notebooks"`
	TagCount      int               `json:"tag_count"`
	ResourceCount int               `json:"resource_count"`
	WarningCount  int               `json:"warning_count"`
}

type ImportNotebook struct {
	Name          string `json:"name"`
	NoteCount     int    `json:"note_count"`
	ResourceCount int    `json:"resource_count"`
	WarningCount  int    `json:"warning_count"`
}

type ImportResult struct {
	Job           *ent.ImportJob `json:"job"`
	JobID         int            `json:"job_id"`
	Status        string         `json:"status"`
	Imported      int            `json:"imported"`
	Skipped       int            `json:"skipped"`
	Failed        int            `json:"failed"`
	ImportedCount int            `json:"imported_count"`
	SkippedCount  int            `json:"skipped_count"`
	FailedCount   int            `json:"failed_count"`
}

type jobOptions struct {
	Sources []jobSource `json:"sources"`
}

type jobSource struct {
	Path         string `json:"path"`
	NotebookName string `json:"notebook_name,omitempty"`
}

type uploadSource struct {
	Filename     string
	NotebookName string
	Data         []byte
}

type parsedSource struct {
	Source jobSource
	Doc    *enex.Document
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

	uploads, err := importSourcesFromUpload(filename, data)
	if err != nil {
		return nil, err
	}
	parsedSources := make([]parsedSource, 0, len(uploads))
	noteCount := 0
	for _, upload := range uploads {
		doc, err := enex.Parse(bytes.NewReader(upload.Data), enex.WithNotebookName(upload.NotebookName))
		if err != nil {
			return nil, err
		}
		parsedSources = append(parsedSources, parsedSource{
			Source: jobSource{NotebookName: upload.NotebookName},
			Doc:    doc,
		})
		noteCount += len(doc.Notes)
	}

	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, err
	}

	txClient := tx.Client()
	savedPaths := make([]string, 0, len(uploads))
	committed := false
	defer func() {
		if committed {
			return
		}
		_ = tx.Rollback()
		for _, path := range savedPaths {
			_ = s.fs.Remove(path)
		}
	}()

	job, err := txClient.ImportJob.Create().
		SetSource("evernote").
		SetFilename(cleanFilename(filename)).
		SetStatus("previewed").
		SetNoteCount(noteCount).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	options := jobOptions{Sources: make([]jobSource, 0, len(uploads))}
	for index, upload := range uploads {
		sourcePath := filepath.Join(
			s.fs.GetDataDir(),
			"imports",
			"evernote",
			fmt.Sprintf("%d", job.ID),
			fmt.Sprintf("%03d-%s", index+1, cleanFilename(upload.Filename)),
		)
		if err := s.fs.WriteFile(sourcePath, upload.Data, 0600); err != nil {
			return nil, fmt.Errorf("save enex: %w", err)
		}
		savedPaths = append(savedPaths, sourcePath)
		parsedSources[index].Source.Path = sourcePath
		options.Sources = append(options.Sources, parsedSources[index].Source)
	}

	optionsJSON, err := json.Marshal(options)
	if err != nil {
		return nil, fmt.Errorf("encode import options: %w", err)
	}
	job, err = job.Update().SetOptionsJSON(string(optionsJSON)).Save(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*ent.ImportItem, 0, noteCount)
	for _, source := range parsedSources {
		for _, parsedNote := range source.Doc.Notes {
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

	tagCount, resourceCount, warningCount, notebooks := previewCounts(parsedSources)
	return &PreviewResult{
		Job:           job,
		Items:         items,
		JobID:         job.ID,
		Filename:      job.Filename,
		NoteCount:     job.NoteCount,
		NotebookCount: len(notebooks),
		Notebooks:     notebooks,
		TagCount:      tagCount,
		ResourceCount: resourceCount,
		WarningCount:  warningCount,
	}, nil
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
	if isTerminalImportStatus(job.Status) {
		return resultFromJob(job), nil
	}

	options, err := decodeJobOptions(job.OptionsJSON)
	if err != nil {
		return nil, err
	}
	parsedSources := make([]parsedSource, 0, len(options.Sources))
	for _, source := range options.Sources {
		rawENEX, err := s.fs.ReadFile(source.Path)
		if err != nil {
			return nil, fmt.Errorf("read enex: %w", err)
		}
		doc, err := enex.Parse(bytes.NewReader(rawENEX), enex.WithNotebookName(source.NotebookName))
		if err != nil {
			return nil, err
		}
		parsedSources = append(parsedSources, parsedSource{Source: source, Doc: doc})
	}

	pendingItems, err := job.QueryItems().
		Where(importitem.StatusEQ("pending")).
		All(ctx)
	if err != nil {
		return nil, err
	}

	notesByKey := make(map[string]enex.Note)
	for _, source := range parsedSources {
		for _, parsedNote := range source.Doc.Notes {
			notesByKey[sourceNoteKey(parsedNote)] = parsedNote
		}
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
	if err := notesvc.NewService(s.client).SyncUserLinks(ctx, userID); err != nil {
		return nil, err
	}

	status := finalImportStatus(result)
	job, err = job.Update().
		SetStatus(status).
		SetImportedCount(result.Imported).
		SetSkippedCount(result.Skipped).
		SetFailedCount(result.Failed).
		SetCompletedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	result.Job = job
	result.JobID = job.ID
	result.Status = job.Status
	result.ImportedCount = result.Imported
	result.SkippedCount = result.Skipped
	result.FailedCount = result.Failed
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

func (s *Service) importNote(ctx context.Context, userID int, parsedNote enex.Note) (*ent.Note, string, error) {
	create := s.client.Note.Create().
		SetTitle(titleOrUntitled(parsedNote.Title)).
		SetContent(importedContent(parsedNote)).
		SetUserID(userID)
	if notebookName := noteNotebookName(parsedNote); notebookName != "" {
		folderRow, err := s.findOrCreateFolder(ctx, userID, notebookName)
		if err != nil {
			return nil, "", err
		}
		create.SetFolderID(folderRow.ID)
	}
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
		data, err := resource.Data.Decode()
		if err != nil || len(data) == 0 {
			skippedResources++
			continue
		}
		if err := s.saveResource(ctx, userID, createdNote.ID, index, resource, data); err != nil {
			return nil, "", err
		}
	}
	if skippedResources > 0 {
		return createdNote, fmt.Sprintf("%d resource(s) skipped", skippedResources), nil
	}
	return createdNote, "", nil
}

func (s *Service) findOrCreateFolder(ctx context.Context, userID int, name string) (*ent.Folder, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("empty folder")
	}

	existing, err := s.client.Folder.Query().
		Where(folder.NameEQ(name), folder.HasUserWith(user.IDEQ(userID)), folder.Not(folder.HasParent())).
		Order(ent.Asc(folder.FieldCreatedAt)).
		First(ctx)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, err
	}

	return s.client.Folder.Create().
		SetName(name).
		SetUserID(userID).
		Save(ctx)
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

func (s *Service) saveResource(ctx context.Context, userID int, noteID uuid.UUID, index int, resource enex.Resource, data []byte) error {
	filename := cleanFilename(resource.Attributes.FileName)
	if filename == "import.enex" {
		filename = fmt.Sprintf("resource-%d", index+1)
	}
	ext := filepath.Ext(filename)
	storedName := uuid.New().String() + ext
	filePath := filepath.Join(s.fs.GetUploadsDir("attachments"), storedName)

	if err := s.fs.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	_, err := s.client.Attachment.Create().
		SetFilename(filename).
		SetFilePath(filePath).
		SetFileSize(int64(len(data))).
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
	if len(options.Sources) == 0 {
		return options, errors.New("missing enex sources")
	}
	for _, source := range options.Sources {
		if source.Path == "" {
			return options, errors.New("missing enex path")
		}
	}
	return options, nil
}

func importSourcesFromUpload(filename string, data []byte) ([]uploadSource, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".zip":
		return zipENEXSources(data)
	default:
		return []uploadSource{{
			Filename:     cleanFilename(filename),
			NotebookName: notebookNameFromFilename(filename),
			Data:         data,
		}}, nil
	}
}

func zipENEXSources(data []byte) ([]uploadSource, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("read zip: %w", err)
	}
	sources := make([]uploadSource, 0, len(reader.File))
	totalBytes := 0
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			return nil, fmt.Errorf("zip contains directory %q", file.Name)
		}
		if !safeZipEntryName(file.Name) {
			return nil, fmt.Errorf("unsafe zip entry %q", file.Name)
		}
		if strings.ToLower(filepath.Ext(file.Name)) != ".enex" {
			return nil, fmt.Errorf("zip contains non-enex entry %q", file.Name)
		}
		src, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %q: %w", file.Name, err)
		}
		entryData, err := io.ReadAll(io.LimitReader(src, maxENEXBytes+1))
		closeErr := src.Close()
		if err != nil {
			return nil, fmt.Errorf("read zip entry %q: %w", file.Name, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close zip entry %q: %w", file.Name, closeErr)
		}
		if len(entryData) == 0 {
			return nil, fmt.Errorf("zip entry %q is empty", file.Name)
		}
		if len(entryData) > maxENEXBytes {
			return nil, fmt.Errorf("%w: max %d bytes", ErrImportTooLarge, maxENEXBytes)
		}
		totalBytes += len(entryData)
		if totalBytes > maxENEXBytes {
			return nil, fmt.Errorf("%w: max %d bytes", ErrImportTooLarge, maxENEXBytes)
		}
		sources = append(sources, uploadSource{
			Filename:     filepath.Base(file.Name),
			NotebookName: notebookNameFromFilename(file.Name),
			Data:         entryData,
		})
	}
	if len(sources) == 0 {
		return nil, errors.New("zip contains no enex files")
	}
	return sources, nil
}

func safeZipEntryName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") {
		return false
	}
	cleaned := filepath.Clean(name)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, ".."+string(filepath.Separator))
}

func cleanFilename(filename string) string {
	filename = strings.TrimSpace(filepath.Base(filename))
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		return "import.enex"
	}
	return filename
}

func notebookNameFromFilename(filename string) string {
	name := strings.TrimSpace(filepath.Base(filename))
	ext := filepath.Ext(name)
	if ext != "" {
		name = strings.TrimSuffix(name, ext)
	}
	name = strings.TrimSpace(name)
	if isGenericNotebookName(name) {
		return ""
	}
	return name
}

func isGenericNotebookName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "import", "evernote", "notes", "notebooks":
		return true
	default:
		return false
	}
}

func titleOrUntitled(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "Untitled"
	}
	return title
}

func previewCounts(sources []parsedSource) (int, int, int, []ImportNotebook) {
	tags := make(map[string]bool)
	resourceCount := 0
	warningCount := 0
	notebooksByName := make(map[string]*ImportNotebook)

	for _, source := range sources {
		for _, note := range source.Doc.Notes {
			notebookName := noteNotebookName(note)
			var notebook *ImportNotebook
			if notebookName != "" {
				notebook = notebooksByName[notebookName]
				if notebook == nil {
					notebook = &ImportNotebook{Name: notebookName}
					notebooksByName[notebookName] = notebook
				}
				notebook.NoteCount++
			}
			for _, tagName := range note.Tags {
				tags[tagName] = true
			}
			for _, resource := range note.Resources {
				resourceCount++
				if notebook != nil {
					notebook.ResourceCount++
				}
				if _, err := resource.Data.Decode(); err != nil {
					warningCount++
					if notebook != nil {
						notebook.WarningCount++
					}
				}
			}
		}
	}

	notebooks := make([]ImportNotebook, 0, len(notebooksByName))
	for _, notebook := range notebooksByName {
		notebooks = append(notebooks, *notebook)
	}
	sort.Slice(notebooks, func(i, j int) bool {
		return notebooks[i].Name < notebooks[j].Name
	})

	return len(tags), resourceCount, warningCount, notebooks
}

func resultFromJob(job *ent.ImportJob) *ImportResult {
	return &ImportResult{
		Job:           job,
		JobID:         job.ID,
		Status:        job.Status,
		Imported:      job.ImportedCount,
		Skipped:       job.SkippedCount,
		Failed:        job.FailedCount,
		ImportedCount: job.ImportedCount,
		SkippedCount:  job.SkippedCount,
		FailedCount:   job.FailedCount,
	}
}

func finalImportStatus(result *ImportResult) string {
	if result.Failed == 0 {
		return "completed"
	}
	if result.Imported == 0 && result.Skipped == 0 {
		return "failed"
	}
	return "completed_with_errors"
}

func isTerminalImportStatus(status string) bool {
	return status == "completed" || status == "completed_with_errors" || status == "failed"
}

func duplicateKey(title string, created time.Time, content string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(content)))
	return strings.ToLower(strings.TrimSpace(title)) + "|" + created.UTC().Format(time.RFC3339) + "|" + hex.EncodeToString(sum[:])
}

func sourceNoteKey(parsedNote enex.Note) string {
	return duplicateKey(parsedNote.Title, parsedNote.Created, importedContent(parsedNote))
}

func importedContent(parsedNote enex.Note) string {
	return enex.PlainText(parsedNote.Content)
}

func noteNotebookName(parsedNote enex.Note) string {
	name := strings.TrimSpace(parsedNote.NotebookName)
	if isGenericNotebookName(name) {
		return ""
	}
	return name
}
