# ENEX Parser and Evernote Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Release `github.com/lib-x/enex v1.1.0` with a stable ENEX/ENML parser API, then update Smarticky to import Evernote tags, attachments, and per-note notebook folders from `.enex` and `.zip` uploads.

**Architecture:** The reusable parser lives in `/home/czyt/code/go/enex` and exposes source-level and note-level notebook metadata. Smarticky stores import sources in job options, parses each source through the released library, resolves folders per note, and keeps import UI changes additive.

**Tech Stack:** Go 1.20 for `github.com/lib-x/enex`; Go 1.25.4, Ent, Echo, SQLite, Svelte 5, TypeScript, Vite for Smarticky.

## Global Constraints

- `github.com/lib-x/enex` remains MIT licensed.
- Release target for `github.com/lib-x/enex` is `v1.1.0`.
- Preserve `Decode(io.Reader)` in `github.com/lib-x/enex` for compatibility.
- Smarticky uses the released `github.com/lib-x/enex v1.1.0`; it does not use a local replace in committed code.
- Smarticky does not create a new app version tag in this Evernote phase; the next Smarticky tag waits until backup restore is complete.
- Smarticky supports single `.enex` and `.zip` containing one or more `.enex` files.
- Smarticky supports single `.enex` files containing multiple note-level notebooks.
- Standard ENEX DTD has no notebook field; parser supports real-world note-level notebook metadata and source-level fallback.
- No database table rebuilds, no user data deletion, and no schema change for this feature.
- Old import API compatibility is not required.
- Zip import rejects path traversal entries, directories, non-ENEX entries, empty archives, and archives above the existing import size budget.
- Frontend file picker accepts `.enex,.zip`.

---

## File Structure

`/home/czyt/code/go/enex`:

- Modify `enex.go`: public API, data model, parser, stream decoder, resource decoding, ENML plain text helper.
- Create `enex_test.go`: library parser, decoder, resource, ENML, and notebook tests.
- Modify `README.md`: document the new API with one single-file example and one stream decoder example.

`/home/czyt/code/go/smarticky`:

- Modify `go.mod` and `go.sum`: add `github.com/lib-x/enex v1.1.0`.
- Delete `internal/importer/evernote/parser.go` after imports are moved.
- Delete `internal/importer/evernote/parser_test.go` after equivalent library and importer tests exist.
- Modify `internal/importer/service.go`: import source extraction, zip support, library integration, folder resolution, preview counts.
- Modify `internal/importer/service_test.go`: importer tests for `.enex`, `.zip`, tags, attachments, folders, duplicate skipping.
- Modify `internal/handler/import.go`: allow `.zip` uploads through the existing endpoint and keep response shape additive.
- Modify `web/app/src/lib/api/imports.ts`: add notebook preview types.
- Modify `web/app/src/lib/stores/imports.ts`: accept `.enex` and `.zip`.
- Modify `web/app/src/lib/components/import/ImportCenter.svelte`: update file picker accept attribute.
- Modify `web/app/src/lib/components/import/ImportSummary.svelte`: show notebook count and notebook list.
- Modify localization files discovered by `rg -n "selectImportFile|notebooks|notebook" web/app/src -S`: add labels used by the import UI.

---

### Task 1: `github.com/lib-x/enex` Core API and Model

**Files:**
- Modify: `/home/czyt/code/go/enex/enex.go`
- Create: `/home/czyt/code/go/enex/enex_test.go`
- Modify: `/home/czyt/code/go/enex/README.md`

**Interfaces:**
- Produces: `Parse(io.Reader, ...Option) (*Document, error)`
- Produces: `Decode(io.Reader) (*Document, error)`
- Produces: `NewDecoder(io.Reader, ...Option) (*Decoder, error)`
- Produces: `(*Decoder).Next() (*Note, error)`
- Produces: `WithNotebookName(string) Option`
- Produces: `WithStrictXML(bool) Option`
- Produces: `WithNotebookFieldNames(...string) Option`
- Produces: `Document`, `Note`, `NoteAttributes`, `Resource`, `ResourceAttributes`, `Recognition`, `Data`

- [ ] **Step 1: Write failing parser tests**

Create `/home/czyt/code/go/enex/enex_test.go` with these tests:

```go
package enex

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestParseExportAttributesTagsAndSourceNotebook(t *testing.T) {
	input := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE en-export SYSTEM "http://xml.evernote.com/pub/evernote-export3.dtd">
<en-export export-date="20260623T010203Z" application="Evernote/Mac" version="10">
  <note>
    <title> Meeting </title>
    <content><![CDATA[<?xml version="1.0" encoding="UTF-8"?><en-note><div>Hello</div></en-note>]]></content>
    <created>20260620T111213Z</created>
    <updated>20260621T141516Z</updated>
    <tag> work </tag>
    <tag>project</tag>
  </note>
</en-export>`)

	doc, err := Parse(input, WithNotebookName("Projects"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.ExportDate != "20260623T010203Z" || doc.Application != "Evernote/Mac" || doc.Version != "10" {
		t.Fatalf("unexpected export attributes: %+v", doc)
	}
	if doc.NotebookName != "Projects" {
		t.Fatalf("expected document notebook fallback, got %q", doc.NotebookName)
	}
	if len(doc.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(doc.Notes))
	}
	note := doc.Notes[0]
	if note.Title != "Meeting" {
		t.Fatalf("expected trimmed title, got %q", note.Title)
	}
	if note.NotebookName != "Projects" {
		t.Fatalf("expected source notebook fallback, got %q", note.NotebookName)
	}
	if got := note.Tags; len(got) != 2 || got[0] != "work" || got[1] != "project" {
		t.Fatalf("unexpected tags: %#v", got)
	}
	if note.Created.Format(time.RFC3339) != "2026-06-20T11:12:13Z" {
		t.Fatalf("unexpected created time: %s", note.Created.Format(time.RFC3339))
	}
	if !strings.Contains(note.Content, "<en-note>") {
		t.Fatalf("content should preserve ENML, got %q", note.Content)
	}
}

func TestParseNoteLevelNotebookFields(t *testing.T) {
	input := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<en-export>
  <note>
    <title>One</title>
    <notebook-name>Notebook A</notebook-name>
    <content><![CDATA[<en-note>One</en-note>]]></content>
  </note>
  <note>
    <title>Two</title>
    <notebookGuid>guid-2</notebookGuid>
    <note-attributes>
      <application-data key="notebook">Notebook B</application-data>
    </note-attributes>
    <content><![CDATA[<en-note>Two</en-note>]]></content>
  </note>
</en-export>`)

	doc, err := Parse(input, WithNotebookName("Fallback"))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Notes[0].NotebookName != "Notebook A" {
		t.Fatalf("expected direct notebook metadata, got %q", doc.Notes[0].NotebookName)
	}
	if doc.Notes[1].NotebookName != "Notebook B" {
		t.Fatalf("expected application-data notebook metadata, got %q", doc.Notes[1].NotebookName)
	}
	if doc.Notes[1].NotebookGUID != "guid-2" {
		t.Fatalf("expected notebook guid, got %q", doc.Notes[1].NotebookGUID)
	}
}

func TestStreamDecoderNext(t *testing.T) {
	decoder, err := NewDecoder(strings.NewReader(`<en-export><note><title>A</title><content><![CDATA[<en-note>A</en-note>]]></content></note><note><title>B</title><content><![CDATA[<en-note>B</en-note>]]></content></note></en-export>`), WithNotebookName("Stream"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := decoder.Next()
	if err != nil {
		t.Fatal(err)
	}
	second, err := decoder.Next()
	if err != nil {
		t.Fatal(err)
	}
	_, err = decoder.Next()
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
	if first.Title != "A" || second.Title != "B" || first.NotebookName != "Stream" || second.NotebookName != "Stream" {
		t.Fatalf("unexpected decoded notes: %+v %+v", first, second)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd /home/czyt/code/go/enex
go test ./...
```

Expected: FAIL because `Parse`, `WithNotebookName`, `WithNotebookFieldNames`, and the new fields do not exist.

- [ ] **Step 3: Implement the parser API and model**

Replace `/home/czyt/code/go/enex/enex.go` with an implementation that includes these concrete definitions:

```go
const evernoteTimeLayout = "20060102T150405Z"

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

type Option func(*parseOptions)

func Parse(r io.Reader, opts ...Option) (*Document, error) {
	options := newParseOptions(opts...)
	var raw enExport
	decoder := xml.NewDecoder(r)
	decoder.Strict = options.strictXML
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("parse enex: %w", err)
	}
	doc := &Document{
		ExportDate:   strings.TrimSpace(raw.ExportDate),
		Application:  strings.TrimSpace(raw.Application),
		Version:      strings.TrimSpace(raw.Version),
		NotebookName: options.notebookName,
		Notes:        make([]Note, 0, len(raw.Notes)),
	}
	for _, rawNote := range raw.Notes {
		note, err := convertNote(rawNote, options)
		if err != nil {
			return nil, err
		}
		doc.Notes = append(doc.Notes, note)
	}
	return doc, nil
}

func Decode(r io.Reader) (*Document, error) {
	return Parse(r)
}
```

Keep unexported XML structs in the same file. The raw note struct must include:

```go
type rawNote struct {
	Title             string              `xml:"title"`
	Content           string              `xml:"content"`
	Created           string              `xml:"created"`
	Updated           string              `xml:"updated"`
	Tags              []string            `xml:"tag"`
	Notebook          string              `xml:"notebook"`
	NotebookNameKebab string              `xml:"notebook-name"`
	NotebookNameCamel string              `xml:"notebookName"`
	NotebookGUIDSnake string              `xml:"notebook_guid"`
	NotebookGUIDKebab string              `xml:"notebook-guid"`
	NotebookGUIDCamel string              `xml:"notebookGuid"`
	Attributes        rawNoteAttributes   `xml:"note-attributes"`
	Resources         []rawResource       `xml:"resource"`
	InnerXML          string              `xml:",innerxml"`
}
```

- [ ] **Step 4: Implement stream decoder**

Add these methods in `/home/czyt/code/go/enex/enex.go`:

```go
type Decoder struct {
	xml     *xml.Decoder
	options parseOptions
}

func NewDecoder(r io.Reader, opts ...Option) (*Decoder, error) {
	options := newParseOptions(opts...)
	decoder := xml.NewDecoder(r)
	decoder.Strict = options.strictXML
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("parse enex: missing en-export root: %w", err)
			}
			return nil, fmt.Errorf("parse enex: %w", err)
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Local == "en-export" {
			return &Decoder{xml: decoder, options: options}, nil
		}
	}
}

func (d *Decoder) Next() (*Note, error) {
	for {
		token, err := d.xml.Token()
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "note" {
			continue
		}
		var raw rawNote
		if err := d.xml.DecodeElement(&raw, &start); err != nil {
			return nil, err
		}
		note, err := convertNote(raw, d.options)
		if err != nil {
			return nil, err
		}
		return &note, nil
	}
}
```

- [ ] **Step 5: Run parser tests**

Run:

```bash
cd /home/czyt/code/go/enex
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Update README examples**

Add this example to `/home/czyt/code/go/enex/README.md`:

```go
doc, err := enex.Parse(file, enex.WithNotebookName("Imported"))
if err != nil {
	return err
}
for _, note := range doc.Notes {
	fmt.Println(note.Title, note.NotebookName, note.Tags)
}
```

Add this stream example:

```go
decoder, err := enex.NewDecoder(file)
if err != nil {
	return err
}
for {
	note, err := decoder.Next()
	if errors.Is(err, io.EOF) {
		break
	}
	if err != nil {
		return err
	}
	fmt.Println(note.Title)
}
```

- [ ] **Step 7: Commit Task 1**

Run:

```bash
cd /home/czyt/code/go/enex
git add enex.go enex_test.go README.md
git commit -m "Add stable ENEX parser API"
```

Expected: commit created in `/home/czyt/code/go/enex`.

---

### Task 2: `github.com/lib-x/enex` Resources, ENML Plain Text, and Recovery Semantics

**Files:**
- Modify: `/home/czyt/code/go/enex/enex.go`
- Modify: `/home/czyt/code/go/enex/enex_test.go`

**Interfaces:**
- Consumes: `Parse`, `Data.Decode`, `Resource.BodyHash`, `PlainText`
- Produces: recoverable invalid resource data, ENML text extraction, resource body MD5 hashes

- [ ] **Step 1: Add failing resource and ENML tests**

Append these tests to `/home/czyt/code/go/enex/enex_test.go`:

```go
func TestResourceDecodeHashAndInvalidDataRecovery(t *testing.T) {
	input := strings.NewReader(`<en-export><note><title>Files</title><content><![CDATA[<en-note><en-media type="text/plain" hash="5d41402abc4b2a76b9719d911017c592"/></en-note>]]></content><resource><data encoding="base64">a G V s b G 8=</data><mime>text/plain</mime><resource-attributes><file-name>hello.txt</file-name></resource-attributes></resource><resource><data encoding="base64">not-valid</data><mime>text/plain</mime></resource></note></en-export>`)
	doc, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	resources := doc.Notes[0].Resources
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resources))
	}
	data, err := resources[0].Data.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("expected decoded hello, got %q", string(data))
	}
	if resources[0].BodyHash != "5d41402abc4b2a76b9719d911017c592" {
		t.Fatalf("unexpected hash %q", resources[0].BodyHash)
	}
	if _, err := resources[1].Data.Decode(); err == nil {
		t.Fatalf("expected invalid base64 error")
	}
}

func TestPlainTextEvernoteElements(t *testing.T) {
	content := `<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE en-note SYSTEM "http://xml.evernote.com/pub/enml2.dtd"><en-note><div><en-todo checked="true"/>Done</div><div><en-todo/>Open<br/>Next</div><en-media type="image/png" hash="abc123"/><en-crypt hint="h">cipher</en-crypt></en-note>`
	got := PlainText(content)
	want := "- [x] Done\n- [ ] Open\nNext\n[attachment:abc123]\n[encrypted content]"
	if got != want {
		t.Fatalf("PlainText mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestInvalidTimestampFailsDocument(t *testing.T) {
	_, err := Parse(strings.NewReader(`<en-export><note><title>Bad</title><content><![CDATA[<en-note>Bad</en-note>]]></content><created>bad-date</created></note></en-export>`))
	if err == nil || !strings.Contains(err.Error(), "created") {
		t.Fatalf("expected created timestamp error, got %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd /home/czyt/code/go/enex
go test ./...
```

Expected: FAIL because `PlainText`, `BodyHash`, and recoverable invalid data behavior are incomplete.

- [ ] **Step 3: Implement data decoding and body hash**

Add these concrete functions to `/home/czyt/code/go/enex/enex.go`:

```go
func (d Data) Decode() ([]byte, error) {
	encoded := compactBase64(d.Content)
	if encoded == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(encoded)
}

func compactBase64(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for _, r := range value {
		if !unicode.IsSpace(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func bodyHash(data []byte) string {
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}
```

In `convertResource`, call `raw.Data.Decode()`. If decoding succeeds and data is not empty, set `BodyHash` from `bodyHash(data)`. If decoding fails, return the resource without failing the note.

- [ ] **Step 4: Implement `PlainText`**

Add `PlainText` in `/home/czyt/code/go/enex/enex.go` with this behavior:

```go
func PlainText(content string) string {
	content = xmlDeclarationPattern.ReplaceAllString(content, "")
	content = doctypePattern.ReplaceAllString(content, "")
	decoder := xml.NewDecoder(strings.NewReader(content))
	decoder.Strict = false
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
				writeNewlineIfNeeded(&builder)
			case "div", "p":
				writeNewlineIfNeeded(&builder)
			case "en-todo":
				writeNewlineIfNeeded(&builder)
				if attrValue(t.Attr, "checked") == "true" {
					builder.WriteString("- [x] ")
				} else {
					builder.WriteString("- [ ] ")
				}
			case "en-media":
				writeNewlineIfNeeded(&builder)
				builder.WriteString("[attachment:")
				builder.WriteString(attrValue(t.Attr, "hash"))
				builder.WriteString("]")
			case "en-crypt":
				writeNewlineIfNeeded(&builder)
				builder.WriteString("[encrypted content]")
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "div", "p", "en-media", "en-crypt":
				writeNewlineIfNeeded(&builder)
			}
		}
	}
	return strings.TrimSpace(builder.String())
}
```

- [ ] **Step 5: Run library tests**

Run:

```bash
cd /home/czyt/code/go/enex
go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit Task 2**

Run:

```bash
cd /home/czyt/code/go/enex
git add enex.go enex_test.go
git commit -m "Add ENML text and resource decoding"
```

Expected: commit created in `/home/czyt/code/go/enex`.

---

### Task 3: Release `github.com/lib-x/enex v1.1.0`

**Files:**
- Modify if needed: `/home/czyt/code/go/enex/go.mod`
- Validate: `/home/czyt/code/go/enex/enex.go`
- Validate: `/home/czyt/code/go/enex/enex_test.go`

**Interfaces:**
- Produces: pushed `v1.1.0` tag available to `go get github.com/lib-x/enex@v1.1.0`

- [ ] **Step 1: Verify repository cleanliness and tests**

Run:

```bash
cd /home/czyt/code/go/enex
gofmt -w enex.go enex_test.go
go test ./...
git status --short --branch -uall
```

Expected: tests PASS and status shows branch ahead with no unstaged files.

- [ ] **Step 2: Confirm tag does not exist**

Run:

```bash
cd /home/czyt/code/go/enex
git tag --list v1.1.0
gh release view v1.1.0 -R lib-x/enex
```

Expected: local tag output is empty and `gh release view` exits non-zero because `v1.1.0` is not published.

- [ ] **Step 3: Push commits and tag**

Run:

```bash
cd /home/czyt/code/go/enex
git push origin main
git tag v1.1.0
git push origin v1.1.0
```

Expected: remote `main` and tag push succeed.

- [ ] **Step 4: Verify module version is fetchable**

Run:

```bash
cd /home/czyt/code/go/enex
GONOSUMDB=github.com/lib-x/enex go list -m github.com/lib-x/enex@v1.1.0
```

Expected output includes:

```text
github.com/lib-x/enex v1.1.0
```

---

### Task 4: Smarticky Backend Import Sources, Parser Integration, Folders, Tags, and Attachments

**Files:**
- Modify: `/home/czyt/code/go/smarticky/go.mod`
- Modify: `/home/czyt/code/go/smarticky/go.sum`
- Modify: `/home/czyt/code/go/smarticky/internal/importer/service.go`
- Modify: `/home/czyt/code/go/smarticky/internal/importer/service_test.go`
- Delete: `/home/czyt/code/go/smarticky/internal/importer/evernote/parser.go`
- Delete: `/home/czyt/code/go/smarticky/internal/importer/evernote/parser_test.go`

**Interfaces:**
- Consumes: `github.com/lib-x/enex.Parse`, `github.com/lib-x/enex.PlainText`, `github.com/lib-x/enex.Data.Decode`
- Produces: `PreviewResult.NotebookCount int`
- Produces: `PreviewResult.Notebooks []ImportNotebook`
- Produces: `jobOptions{Sources []jobSource}`
- Produces: `.zip` and `.enex` source extraction

- [ ] **Step 1: Add dependency**

Run:

```bash
cd /home/czyt/code/go/smarticky
go get github.com/lib-x/enex@v1.1.0
```

Expected: `go.mod` contains `github.com/lib-x/enex v1.1.0`.

- [ ] **Step 2: Write failing importer tests**

Append these tests to `/home/czyt/code/go/smarticky/internal/importer/service_test.go`:

```go
func TestConfirmEvernoteImportsSingleFileMultipleNotebooksTagsAndAttachments(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestConfirmEvernoteImportsSingleFileMultipleNotebooksTagsAndAttachments?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	u := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)

	input := `<?xml version="1.0" encoding="UTF-8"?>
<en-export>
  <note>
    <title>Alpha</title>
    <notebook-name>Work</notebook-name>
    <content><![CDATA[<en-note><div>Hello alpha</div></en-note>]]></content>
    <created>20260620T010203Z</created>
    <tag>project</tag>
    <resource><data encoding="base64">aGVsbG8=</data><mime>text/plain</mime><resource-attributes><file-name>hello.txt</file-name></resource-attributes></resource>
  </note>
  <note>
    <title>Beta</title>
    <note-attributes><application-data key="notebook">Personal</application-data></note-attributes>
    <content><![CDATA[<en-note><div>Hello beta</div></en-note>]]></content>
    <created>20260621T010203Z</created>
    <tag>home</tag>
  </note>
</en-export>`

	service := NewService(client, storage.NewMemoryFileSystem())
	preview, err := service.PreviewEvernote(ctx, u.ID, "all.enex", strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if preview.NotebookCount != 2 || len(preview.Notebooks) != 2 {
		t.Fatalf("expected 2 notebooks, got count=%d notebooks=%#v", preview.NotebookCount, preview.Notebooks)
	}
	result, err := service.ConfirmEvernote(ctx, u.ID, preview.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 || result.Failed != 0 {
		t.Fatalf("expected 2 imported, got imported=%d failed=%d", result.Imported, result.Failed)
	}
	folders := client.Folder.Query().AllX(ctx)
	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}
	notes := client.Note.Query().WithFolder().WithTags().WithAttachments().AllX(ctx)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Edges.Folder == nil || notes[1].Edges.Folder == nil {
		t.Fatalf("expected imported notes to have folders")
	}
	if totalTags := client.Tag.Query().CountX(ctx); totalTags != 2 {
		t.Fatalf("expected 2 tags, got %d", totalTags)
	}
	if totalAttachments := client.Attachment.Query().CountX(ctx); totalAttachments != 1 {
		t.Fatalf("expected 1 attachment, got %d", totalAttachments)
	}
}

func TestPreviewEvernoteZipCreatesNotebookSources(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestPreviewEvernoteZipCreatesNotebookSources?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	u := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	for name, title := range map[string]string{
		"Work.enex":     "Work Note",
		"Personal.enex": "Personal Note",
	} {
		entry, err := zipWriter.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = entry.Write([]byte(`<en-export><note><title>` + title + `</title><content><![CDATA[<en-note>` + title + `</en-note>]]></content><created>20260620T010203Z</created></note></en-export>`))
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	service := NewService(client, storage.NewMemoryFileSystem())
	preview, err := service.PreviewEvernote(ctx, u.ID, "notebooks.zip", bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if preview.NoteCount != 2 || preview.NotebookCount != 2 {
		t.Fatalf("expected 2 notes and 2 notebooks, got notes=%d notebooks=%d", preview.NoteCount, preview.NotebookCount)
	}
	result, err := service.ConfirmEvernote(ctx, u.ID, preview.Job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Imported != 2 {
		t.Fatalf("expected 2 imported, got %d", result.Imported)
	}
	if folders := client.Folder.Query().CountX(ctx); folders != 2 {
		t.Fatalf("expected 2 folders, got %d", folders)
	}
}

func TestPreviewEvernoteZipRejectsUnsafeEntries(t *testing.T) {
	ctx := context.Background()
	client := enttest.Open(t, "sqlite3", "file:TestPreviewEvernoteZipRejectsUnsafeEntries?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	defer client.Close()
	u := client.User.Create().SetUsername("owner").SetPasswordHash("hash").SaveX(ctx)

	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	entry, err := zipWriter.Create("../evil.enex")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte(`<en-export><note><title>Evil</title><content><![CDATA[<en-note>Evil</en-note>]]></content></note></en-export>`))
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	service := NewService(client, storage.NewMemoryFileSystem())
	if _, err := service.PreviewEvernote(ctx, u.ID, "evil.zip", bytes.NewReader(buf.Bytes())); err == nil {
		t.Fatalf("expected unsafe zip entry error")
	}
}
```

Update imports in the test file to include:

```go
import (
	"archive/zip"
	"bytes"
	"context"
	"strings"
	"testing"

	"smarticky/ent/enttest"
	"smarticky/internal/storage"

	_ "github.com/lib-x/entsqlite"
)
```

- [ ] **Step 3: Run importer tests to verify they fail**

Run:

```bash
cd /home/czyt/code/go/smarticky
go test ./internal/importer
```

Expected: FAIL because `NotebookCount`, zip support, and folder assignment are not implemented.

- [ ] **Step 4: Replace local parser usage**

In `/home/czyt/code/go/smarticky/internal/importer/service.go`:

Replace:

```go
"smarticky/internal/importer/evernote"
```

with:

```go
enex "github.com/lib-x/enex"
```

Change note type references from `evernote.Note` to `enex.Note`.

Change:

```go
func importedContent(parsedNote evernote.Note) string {
	return enmlToText(parsedNote.Content)
}
```

to:

```go
func importedContent(parsedNote enex.Note) string {
	return enex.PlainText(parsedNote.Content)
}
```

Remove `enmlToText`, `writeNewlineIfNeeded`, `xmlDeclarationPattern`, and `doctypePattern` from `service.go`.

- [ ] **Step 5: Implement import source extraction**

Add these types to `/home/czyt/code/go/smarticky/internal/importer/service.go`:

```go
type ImportNotebook struct {
	Name          string `json:"name"`
	NoteCount     int    `json:"note_count"`
	ResourceCount int    `json:"resource_count"`
	WarningCount  int    `json:"warning_count"`
}

type jobOptions struct {
	Sources []jobSource `json:"sources"`
}

type jobSource struct {
	Path         string `json:"path"`
	NotebookName string `json:"notebook_name,omitempty"`
}

type parsedSource struct {
	Source jobSource
	Doc    *enex.Document
}
```

Add fields to `PreviewResult`:

```go
NotebookCount int              `json:"notebook_count"`
Notebooks     []ImportNotebook `json:"notebooks"`
```

Implement:

```go
func importSourcesFromUpload(filename string, data []byte) ([]jobSource, map[string][]byte, error)
func singleENEXSource(filename string, data []byte) ([]jobSource, map[string][]byte)
func zipENEXSources(filename string, data []byte) ([]jobSource, map[string][]byte, error)
func notebookNameFromFilename(filename string) string
func isGenericNotebookName(name string) bool
func safeZipEntryName(name string) bool
```

The source `Path` values should be storage keys such as `imports/evernote/<jobID>/<index>.enex` after the job ID is known. During preview, keep the raw bytes in memory until the job row exists, then write each source under that directory.

- [ ] **Step 6: Implement preview parsing and counts**

In `PreviewEvernote`:

1. Read upload bytes with `readLimited`.
2. Call `importSourcesFromUpload`.
3. Parse each source with `enex.Parse(bytes.NewReader(sourceBytes), enex.WithNotebookName(source.NotebookName))`.
4. Create one `ImportItem` per parsed note.
5. Store `jobOptions{Sources: sources}`.
6. Return aggregate note, tag, resource, warning, and notebook counts.

Implement counts:

```go
func previewCounts(sources []parsedSource) (tagCount int, resourceCount int, warningCount int, notebooks []ImportNotebook)
```

For warning count, call `resource.Data.Decode()` and count decode failures.

- [ ] **Step 7: Implement confirm folder resolution and attachment decode**

Add:

```go
func (s *Service) findOrCreateFolder(ctx context.Context, userID int, name string) (*ent.Folder, error)
func noteNotebookName(parsedNote enex.Note) string
```

In `importNote`, before saving the note:

```go
if notebookName := noteNotebookName(parsedNote); notebookName != "" {
	folderRow, err := s.findOrCreateFolder(ctx, userID, notebookName)
	if err != nil {
		return nil, "", err
	}
	create.SetFolderID(folderRow.ID)
}
```

Change resource saving to decode via:

```go
data, err := resource.Data.Decode()
if err != nil || len(data) == 0 {
	skippedResources++
	continue
}
if err := s.saveResource(ctx, userID, createdNote.ID, index, resource, data); err != nil {
	return nil, "", err
}
```

Change `saveResource` signature:

```go
func (s *Service) saveResource(ctx context.Context, userID int, noteID uuid.UUID, index int, resource enex.Resource, data []byte) error
```

- [ ] **Step 8: Delete local parser package**

Run:

```bash
cd /home/czyt/code/go/smarticky
git rm internal/importer/evernote/parser.go internal/importer/evernote/parser_test.go
```

Expected: local parser files staged for deletion.

- [ ] **Step 9: Run backend tests**

Run:

```bash
cd /home/czyt/code/go/smarticky
go test ./internal/importer ./internal/handler ./internal/notes
```

Expected: PASS.

- [ ] **Step 10: Commit Task 4**

Run:

```bash
cd /home/czyt/code/go/smarticky
git add go.mod go.sum internal/importer/service.go internal/importer/service_test.go internal/importer/evernote/parser.go internal/importer/evernote/parser_test.go
git commit -m "Import Evernote notebooks with enex library"
```

Expected: commit created in Smarticky.

---

### Task 5: Smarticky Frontend Import UX for Notebooks and Zip

**Files:**
- Modify: `/home/czyt/code/go/smarticky/web/app/src/lib/api/imports.ts`
- Modify: `/home/czyt/code/go/smarticky/web/app/src/lib/stores/imports.ts`
- Modify: `/home/czyt/code/go/smarticky/web/app/src/lib/components/import/ImportCenter.svelte`
- Modify: `/home/czyt/code/go/smarticky/web/app/src/lib/components/import/ImportSummary.svelte`
- Modify: localization files found by `rg -n "selectImportFile|attachments|warnings|importStart" /home/czyt/code/go/smarticky/web/app/src -S`

**Interfaces:**
- Consumes: backend preview response fields `notebook_count` and `notebooks`
- Produces: import UI that accepts `.enex` and `.zip`, shows notebook count and notebook rows

- [ ] **Step 1: Update API types**

In `/home/czyt/code/go/smarticky/web/app/src/lib/api/imports.ts`, add:

```ts
export interface ImportNotebook {
  name: string;
  note_count: number;
  resource_count: number;
  warning_count: number;
}
```

Add fields to `ImportPreview`:

```ts
notebook_count: number;
notebooks: ImportNotebook[];
```

- [ ] **Step 2: Update accepted file extensions**

In `/home/czyt/code/go/smarticky/web/app/src/lib/stores/imports.ts`, replace:

```ts
if (!file.name.toLowerCase().endsWith(".enex")) {
```

with:

```ts
const lowerName = file.name.toLowerCase();
if (!lowerName.endsWith(".enex") && !lowerName.endsWith(".zip")) {
```

In `/home/czyt/code/go/smarticky/web/app/src/lib/components/import/ImportCenter.svelte`, replace:

```svelte
accept=".enex"
```

with:

```svelte
accept=".enex,.zip"
```

- [ ] **Step 3: Update import summary UI**

In `/home/czyt/code/go/smarticky/web/app/src/lib/components/import/ImportSummary.svelte`, add notebook count into `rows` before tags:

```ts
{ label: t("notebooks", $preferencesStore.language), value: preview.notebook_count },
```

Below the summary grid, add:

```svelte
{#if preview?.notebooks?.length}
  <div class="import-summary__notebooks" aria-label={t("notebooks", $preferencesStore.language)}>
    {#each preview.notebooks as notebook}
      <div class="import-summary__notebook">
        <span title={notebook.name}>{notebook.name}</span>
        <small>{notebook.note_count} {t("notes", $preferencesStore.language)}</small>
      </div>
    {/each}
  </div>
{/if}
```

Add CSS in the same component:

```css
.import-summary__notebooks {
  display: grid;
  gap: 0.5rem;
  margin-top: 0.75rem;
}

.import-summary__notebook {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-width: 0;
  border: 1px solid var(--border-muted);
  border-radius: 8px;
  padding: 0.5rem 0.625rem;
}

.import-summary__notebook span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.import-summary__notebook small {
  flex: 0 0 auto;
}
```

If `--border-muted` is not defined in nearby styles, use the existing border token used by `.import-summary__item`.

- [ ] **Step 4: Update localization**

Run:

```bash
cd /home/czyt/code/go/smarticky
rg -n "\"notes\"|\"attachments\"|\"warnings\"|\"selectFile\"" web/app/src -S
```

For each localization map that contains `notes`, add:

```ts
notebooks: "Notebooks"
```

For Chinese maps, add:

```ts
notebooks: "笔记本"
```

Update file selection copy from ENEX-only to ENEX/ZIP where the existing string mentions `.enex`.

- [ ] **Step 5: Run frontend checks**

Run:

```bash
cd /home/czyt/code/go/smarticky/web/app
npm run check
npm run build
```

Expected: both commands succeed. Existing Radix `use client` warnings are acceptable if build exits 0.

- [ ] **Step 6: Commit Task 5**

Run:

```bash
cd /home/czyt/code/go/smarticky
git add web/app/src/lib/api/imports.ts web/app/src/lib/stores/imports.ts web/app/src/lib/components/import/ImportCenter.svelte web/app/src/lib/components/import/ImportSummary.svelte web/app/src
git commit -m "Show Evernote notebook import preview"
```

Expected: commit created in Smarticky.

---

### Task 6: End-to-End Verification and Push

**Files:**
- Validate: `/home/czyt/code/go/enex`
- Validate: `/home/czyt/code/go/smarticky`

**Interfaces:**
- Consumes: all previous tasks
- Produces: pushed library release and pushed Smarticky commits

- [ ] **Step 1: Verify `enex` release state**

Run:

```bash
cd /home/czyt/code/go/enex
git status --short --branch -uall
git tag --list v1.1.0
go test ./...
```

Expected: clean or ahead only if push was intentionally deferred, tag `v1.1.0` exists, tests PASS.

- [ ] **Step 2: Verify Smarticky backend**

Run:

```bash
cd /home/czyt/code/go/smarticky
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Verify Smarticky frontend**

Run:

```bash
cd /home/czyt/code/go/smarticky/web/app
npm run check
npm run build
```

Expected: PASS. Radix module-level directive warnings are acceptable if the process exits 0.

- [ ] **Step 4: Inspect final diffs and commits**

Run:

```bash
cd /home/czyt/code/go/smarticky
git status --short --branch -uall
git log --oneline --decorate --max-count=8
```

Expected: Smarticky has only intentional commits after `a578aff Add ENEX import design`.

- [ ] **Step 5: Push Smarticky commits**

Run:

```bash
cd /home/czyt/code/go/smarticky
git push origin main
```

Expected: push succeeds. Do not create a Smarticky app tag in this Evernote phase; the next Smarticky tag waits until backup restore is complete.
