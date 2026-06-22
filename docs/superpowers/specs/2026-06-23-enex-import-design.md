# ENEX Parser and Evernote Import Design

Date: 2026-06-23

## Goal

Smarticky should import Evernote notes with tags, attachments, and notebook structure. The parsing work should live in `github.com/lib-x/enex` as a reusable MIT-licensed Go library, then Smarticky should consume the released version instead of its local hand-written ENEX parser.

## Official Format Constraints

The design follows Evernote's official references:

- Note Export Format: https://dev.evernote.com/doc/articles/note_export.php
- ENEX DTD: https://xml.evernote.com/pub/evernote-export3.dtd
- ENML: https://dev.evernote.com/doc/articles/enml.php
- EDAM Types: https://dev.evernote.com/doc/reference/Types.html

Important constraints:

- ENEX exports contain `en-export` with one or more `note` elements.
- A note contains `title`, `content`, optional `created`, optional `updated`, zero or more `tag`, optional `note-attributes`, and zero or more `resource`.
- Standard ENEX DTD does not define a notebook field. Evernote's API has `Note.notebookGuid`, and real-world exports or backup tools may preserve notebook information through non-standard note-level fields or application data.
- ENML content can contain Evernote-specific elements including `en-note`, `en-media`, `en-crypt`, and `en-todo`.
- `en-media` references a resource by the resource body's MD5 hash, not by file name.

Notebook import therefore uses a layered model: prefer explicit note-level notebook metadata when present, then fall back to source-level context from parser options, uploaded file name, or zip entry name.

## `github.com/lib-x/enex` API Design

Release target: `v1.1.0`.

The module keeps the MIT license and preserves the existing `Decode(io.Reader)` entry point for compatibility, while adding a clearer API for new consumers.

### Public API

```go
package enex

func Parse(r io.Reader, opts ...Option) (*Document, error)
func Decode(r io.Reader) (*Document, error)
func NewDecoder(r io.Reader, opts ...Option) (*Decoder, error)
func PlainText(content string) string

type Option func(*parseOptions)

func WithNotebookName(name string) Option
func WithStrictXML(strict bool) Option
func WithNotebookFieldNames(names ...string) Option

type Decoder struct {
    // unexported
}

func (d *Decoder) Next() (*Note, error)
```

`Decode` delegates to `Parse` with default options.
`NewDecoder` validates enough of the stream to find the `en-export` root and returns an error if the file is not a valid ENEX stream.

### Data Model

```go
type Document struct {
    ExportDate   string
    Application  string
    Version      string
    NotebookName string
    Notes        []Note
}

type Note struct {
    Title        string
    Content      string
    Created      time.Time
    Updated      time.Time
    Tags         []string
    Attributes   NoteAttributes
    Resources    []Resource
    NotebookName string
    NotebookGUID string
}

type NoteAttributes struct {
    SubjectDate      time.Time
    Latitude         string
    Longitude        string
    Altitude         string
    Author           string
    Source           string
    SourceURL        string
    SourceApplication string
    ReminderOrder    string
    ReminderTime     time.Time
    ReminderDoneTime time.Time
    PlaceName        string
    ContentClass     string
    ApplicationData  map[string]string
    Unknown          map[string]string
}

type Resource struct {
    Data            Data
    MIME            string
    Width           int
    Height          int
    Duration        int
    BodyHash        string
    Recognition     Recognition
    Attributes      ResourceAttributes
    AlternateData   Data
}

type Data struct {
    Encoding string
    Content  string
}

func (d Data) Decode() ([]byte, error)
```

The parser computes `Resource.BodyHash` from decoded resource data. If resource data cannot be decoded, parsing still returns the note and leaves the decode error for callers through `Data.Decode()`.
`Note.Content` stores the original ENML content string from the export, including the `en-note` document structure when present. Plain text conversion is always explicit through `PlainText`.
`Note.NotebookName` is populated from note-level notebook metadata when the export contains it, otherwise from `WithNotebookName`.

### Notebook Metadata Extraction

The parser recognizes notebook context in this order:

1. Direct note child elements with common names: `notebook`, `notebook-name`, `notebookName`, `notebook_guid`, `notebook-guid`, `notebookGuid`.
2. `note-attributes/application-data` entries whose keys match those names.
3. Additional names supplied by `WithNotebookFieldNames`.
4. The source-level fallback from `WithNotebookName`.

This keeps the official ENEX path strict enough for normal files while allowing Smarticky to import enriched single-file exports that contain multiple notebooks.

### ENML Helpers

The library exposes a small text extraction helper rather than a full HTML renderer:

- `PlainText` strips XML declaration and doctype.
- Text nodes are preserved.
- `br` writes a newline.
- `div` and `p` write paragraph boundaries.
- `en-todo checked="true"` writes `- [x] `.
- `en-todo checked="false"` or missing checked writes `- [ ] `.
- `en-media` writes a stable attachment marker such as `[attachment:<hash>]`.
- `en-crypt` writes `[encrypted content]`.

Rich rendering remains a caller concern.

### Error Behavior

- Malformed ENEX XML returns an error from `Parse`.
- Invalid note timestamps return an error because the import order and duplicate key logic depend on stable dates.
- Invalid resource base64 does not fail the whole document; callers can count or skip that resource.
- `Decoder.Next()` returns `io.EOF` after the final note.

## Smarticky Import Design

Smarticky replaces `internal/importer/evernote` with `github.com/lib-x/enex`.

### Upload Formats

Smarticky supports:

- Single `.enex`
- `.zip` containing one or more `.enex` files

The existing `.enex` flow remains available.
Zip handling rejects path traversal entries, directories, non-ENEX entries, empty archives, and archives that exceed the existing import size budget.

### Notebook Mapping

Smarticky maps notebook context as follows:

- If a parsed note has `Note.NotebookName`, use it.
- If a parsed note does not have `Note.NotebookName`, use the source fallback:
  - For a single `.enex`, source fallback is `strings.TrimSuffix(cleanFilename(uploadName), ext)`.
  - For a `.zip`, source fallback is each `.enex` entry's base file name without extension.
- Empty or generic names such as `import` fall back to no folder assignment.
- Smarticky creates or reuses a same-user top-level folder with that notebook name.
- Imported notes are assigned per note, so one `.enex` file can create and populate multiple folders when it contains note-level notebook metadata.

This does not require a database schema change and does not rebuild or delete existing data.

### Preview Result

The existing preview response is extended with additive fields:

```go
type PreviewResult struct {
    Job           *ent.ImportJob
    Items         []*ent.ImportItem
    JobID         int
    Filename      string
    NoteCount     int
    TagCount      int
    ResourceCount int
    WarningCount  int
    NotebookCount int
    Notebooks     []ImportNotebook
}

type ImportNotebook struct {
    Name          string `json:"name"`
    NoteCount     int    `json:"note_count"`
    ResourceCount int    `json:"resource_count"`
    WarningCount  int    `json:"warning_count"`
}
```

The frontend import summary shows notebook count and, when present, the notebook names with note counts.

### Job Storage

`jobOptions` changes from one `ENEXPath` to a list of import sources:

```go
type jobOptions struct {
    Sources []jobSource `json:"sources"`
}

type jobSource struct {
    Path         string `json:"path"`
    NotebookName string `json:"notebook_name,omitempty"`
}
```

Old API compatibility is not required. Existing completed jobs remain historical records; new preview jobs use the new options shape.

### Confirm Flow

On confirm:

1. Read each stored source.
2. Parse with `enex.Parse(..., enex.WithNotebookName(source.NotebookName))`.
3. Rebuild the source note key from title, created time, and plain text content.
4. Skip duplicate notes using the existing duplicate logic.
5. Resolve folder from each note's notebook name, using note metadata first and source fallback second.
6. Create the note with folder assignment when a folder exists.
7. Create or reuse tags and attach them to the note.
8. Decode valid resources and create Smarticky attachments.
9. Skip invalid resources and increment warnings/messages.
10. Sync backlinks after import.

### Frontend

The file picker accepts `.enex,.zip`.

Preview UI additions:

- notebook count in the summary row
- compact notebook list under the summary when there are notebooks
- existing note/tag/attachment/warning numbers remain visible

No import confirmation danger dialog is needed because this operation only creates notes, tags, folders, and attachments.

## Testing

### `github.com/lib-x/enex`

Add tests for:

- ENEX export attributes
- tags
- note attributes
- resource attributes
- base64 decoding with whitespace
- invalid resource base64 staying recoverable
- `en-media` hash extraction and body hash computation
- `en-todo` checked and unchecked plain text
- `en-crypt` plain text marker
- stream decoder `Next`
- `WithNotebookName`
- explicit note-level notebook metadata
- notebook metadata from application data

### Smarticky

Add tests for:

- previewing single `.enex` with notebook inferred from file name
- previewing single `.enex` with multiple note-level notebooks
- confirming single `.enex` creates/reuses a folder and assigns notes
- confirming single `.enex` with multiple notebooks assigns notes to matching folders
- tags are imported and associated
- resources are saved as attachments
- invalid resources are skipped without failing the whole import
- previewing `.zip` with multiple `.enex` entries produces multiple notebooks
- confirming `.zip` imports notes into matching folders
- duplicate note skipping still works

## Rollout

1. Implement and tag `github.com/lib-x/enex` `v1.1.0`.
2. Update Smarticky `go.mod` to use `github.com/lib-x/enex v1.1.0`.
3. Remove Smarticky's local Evernote parser package after the importer uses the library.
4. Run Go tests for both repositories.
5. Run frontend checks for import API type changes.
6. Commit Smarticky changes separately from the `lib-x/enex` library release.
