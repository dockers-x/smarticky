package handler

import (
	"bytes"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// RenderMarkdown renders markdown to HTML using goldmark
func (h *Handler) RenderMarkdown(c echo.Context) error {
	var req struct {
		Content string `json:"content"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Create goldmark instance with extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,        // GitHub Flavored Markdown
			extension.Table,      // Tables
			extension.Strikethrough, // ~~strikethrough~~
			extension.Linkify,    // Auto-link URLs
			extension.TaskList,   // Task lists
			extension.DefinitionList, // Definition lists
			extension.Footnote,   // Footnotes
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(), // Convert line breaks to <br>
			html.WithXHTML(),     // Use XHTML-style tags
		),
	)

	// Render markdown to HTML
	var buf bytes.Buffer
	if err := md.Convert([]byte(req.Content), &buf); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to render markdown"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"html": buf.String(),
	})
}
