package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"smarticky/ent"
	"smarticky/internal/notes"
	"smarticky/internal/shareimage"
	"smarticky/internal/version"

	"github.com/google/uuid"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type listNotesInput struct {
	Limit  int `json:"limit,omitempty" jsonschema:"maximum number of notes to return, defaults to 20 and caps at 100"`
	Offset int `json:"offset,omitempty" jsonschema:"number of notes to skip"`
}

type searchNotesInput struct {
	Query  string `json:"query" jsonschema:"text to search in note titles and content"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum number of notes to return, defaults to 20 and caps at 100"`
	Offset int    `json:"offset,omitempty" jsonschema:"number of notes to skip"`
}

type getNoteInput struct {
	ID string `json:"id" jsonschema:"note UUID"`
}

type createNoteInput struct {
	Title   string `json:"title,omitempty" jsonschema:"note title, defaults to Untitled"`
	Content string `json:"content,omitempty" jsonschema:"note body"`
	Color   string `json:"color,omitempty" jsonschema:"optional note color"`
}

type generateNoteImageInput struct {
	NoteID  string `json:"note_id,omitempty" jsonschema:"owned note UUID to render"`
	Title   string `json:"title,omitempty" jsonschema:"title to render when note_id is omitted"`
	Content string `json:"content,omitempty" jsonschema:"content to render when note_id is omitted"`
	Theme   string `json:"theme,omitempty" jsonschema:"image theme: classic, paper, or night, defaults to classic"`
	Ratio   string `json:"ratio,omitempty" jsonschema:"image ratio: story or square, defaults to story long image"`
}

const generateNoteImageDescription = "Generate a PNG share image from an owned note or explicit title/content. Locked notes cannot be rendered. Markdown diagrams are not rendered in MCP images yet."

type notesOutput struct {
	Notes []mcpNote `json:"notes"`
	Count int       `json:"count"`
}

type noteOutput struct {
	Note mcpNote `json:"note"`
}

type mcpNote struct {
	ID              string    `json:"id"`
	Title           string    `json:"title"`
	Content         string    `json:"content,omitempty"`
	Color           string    `json:"color"`
	IsLocked        bool      `json:"is_locked"`
	IsStarred       bool      `json:"is_starred"`
	IsDeleted       bool      `json:"is_deleted"`
	ContentRedacted bool      `json:"content_redacted"`
	Tags            []string  `json:"tags,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type imageOutput struct {
	ID          int    `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"download_url"`
}

func NewHTTPHandler(client *ent.Client, noteService *notes.Service, imageService *shareimage.Service, trustLazyCatHeaders bool) http.Handler {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "smarticky",
		Title:   "Smarticky Notes",
		Version: version.Version,
	}, nil)

	registerTools(server, noteService, imageService)

	handler := mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return server },
		&mcpsdk.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)

	return NewAuthenticator(client, trustLazyCatHeaders).Middleware(handler)
}

func registerTools(server *mcpsdk.Server, noteService *notes.Service, imageService *shareimage.Service) {
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "smarticky_list_notes",
		Title:       "List Smarticky Notes",
		Description: "List the current Smarticky user's non-deleted notes. Locked note content is redacted.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, input listNotesInput) (*mcpsdk.CallToolResult, notesOutput, error) {
		principal, err := requirePrincipal(ctx)
		if err != nil {
			return nil, notesOutput{}, err
		}
		rows, err := noteService.List(ctx, principal.UserID, notes.ListOptions{
			Limit:        input.Limit,
			Offset:       input.Offset,
			RedactLocked: true,
		})
		return nil, notesOutput{Notes: mcpNotes(rows), Count: len(rows)}, err
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "smarticky_search_notes",
		Title:       "Search Smarticky Notes",
		Description: "Search the current Smarticky user's non-deleted notes by title or content. Locked note content is redacted.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, input searchNotesInput) (*mcpsdk.CallToolResult, notesOutput, error) {
		principal, err := requirePrincipal(ctx)
		if err != nil {
			return nil, notesOutput{}, err
		}
		rows, err := noteService.List(ctx, principal.UserID, notes.ListOptions{
			Query:        input.Query,
			Limit:        input.Limit,
			Offset:       input.Offset,
			RedactLocked: true,
		})
		return nil, notesOutput{Notes: mcpNotes(rows), Count: len(rows)}, err
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "smarticky_get_note",
		Title:       "Get Smarticky Note",
		Description: "Get one note owned by the current Smarticky user. Locked note content is redacted.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, input getNoteInput) (*mcpsdk.CallToolResult, noteOutput, error) {
		principal, err := requirePrincipal(ctx)
		if err != nil {
			return nil, noteOutput{}, err
		}
		id, err := uuid.Parse(strings.TrimSpace(input.ID))
		if err != nil {
			return nil, noteOutput{}, errors.New("invalid note id")
		}
		row, err := noteService.Get(ctx, principal.UserID, id, true)
		return nil, noteOutput{Note: mcpNoteFrom(row)}, err
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "smarticky_create_note",
		Title:       "Create Smarticky Note",
		Description: "Create a note for the current Smarticky user.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, input createNoteInput) (*mcpsdk.CallToolResult, noteOutput, error) {
		principal, err := requirePrincipal(ctx)
		if err != nil {
			return nil, noteOutput{}, err
		}
		row, err := noteService.Create(ctx, principal.UserID, notes.CreateInput{
			Title:   input.Title,
			Content: input.Content,
			Color:   input.Color,
		})
		return nil, noteOutput{Note: mcpNoteFrom(row)}, err
	})

	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        "smarticky_generate_note_image",
		Title:       "Generate Smarticky Note Image",
		Description: generateNoteImageDescription,
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, input generateNoteImageInput) (*mcpsdk.CallToolResult, imageOutput, error) {
		principal, err := requirePrincipal(ctx)
		if err != nil {
			return nil, imageOutput{}, err
		}

		title := input.Title
		content := input.Content
		if strings.TrimSpace(input.NoteID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(input.NoteID))
			if err != nil {
				return nil, imageOutput{}, errors.New("invalid note id")
			}
			row, err := noteService.Get(ctx, principal.UserID, id, false)
			if err != nil {
				return nil, imageOutput{}, err
			}
			if row.IsLocked {
				return nil, imageOutput{}, errors.New("locked notes cannot be rendered through MCP")
			}
			title = row.Title
			content = row.Content
		}

		generated, err := imageService.Generate(ctx, principal.UserID, shareimage.GenerateInput{
			Title:   title,
			Content: content,
			Theme:   input.Theme,
			Ratio:   input.Ratio,
		})
		if err != nil {
			return nil, imageOutput{}, err
		}

		downloadURL := "/mcp/images/" + strconv.Itoa(generated.ID)
		if principal.BaseURL != "" {
			downloadURL = strings.TrimRight(principal.BaseURL, "/") + downloadURL
		}

		return nil, imageOutput{
			ID:          generated.ID,
			Filename:    generated.Filename,
			ContentType: generated.ContentType,
			Size:        generated.Size,
			DownloadURL: downloadURL,
		}, nil
	})
}

func requirePrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, ErrUnauthorized
	}
	return principal, nil
}

func NewImageDownloadHandler(client *ent.Client, imageService *shareimage.Service, trustLazyCatHeaders bool) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		idText := strings.TrimPrefix(r.URL.Path, "/mcp/images/")
		id, err := strconv.Atoi(idText)
		if err != nil {
			http.Error(w, "invalid image id", http.StatusBadRequest)
			return
		}

		imageFile, err := imageService.GetOwnedImage(r.Context(), principal.UserID, id)
		if err != nil {
			http.Error(w, "image not found", http.StatusNotFound)
			return
		}

		data, err := os.ReadFile(imageFile.Path)
		if err != nil {
			http.Error(w, "image file not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", imageFile.ContentType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+imageFile.Filename+`"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})

	return NewAuthenticator(client, trustLazyCatHeaders).Middleware(handler)
}

func mcpNotes(rows []notes.NoteView) []mcpNote {
	out := make([]mcpNote, 0, len(rows))
	for _, row := range rows {
		out = append(out, mcpNoteFrom(row))
	}
	return out
}

func mcpNoteFrom(row notes.NoteView) mcpNote {
	return mcpNote{
		ID:              row.ID.String(),
		Title:           row.Title,
		Content:         row.Content,
		Color:           row.Color,
		IsLocked:        row.IsLocked,
		IsStarred:       row.IsStarred,
		IsDeleted:       row.IsDeleted,
		ContentRedacted: row.ContentRedacted,
		Tags:            row.Tags,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
