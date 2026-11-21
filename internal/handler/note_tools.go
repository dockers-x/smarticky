package handler

import (
	"errors"
	"net/http"

	"smarticky/ent"
	"smarticky/ent/note"
	"smarticky/internal/notetools"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func (h *Handler) RegisterNoteTools(group *echo.Group, registry *notetools.Registry) {
	group.GET("/note-tools", func(c echo.Context) error {
		return c.JSON(http.StatusOK, registry.List())
	})
	group.POST("/notes/:id/tools/:tool", func(c echo.Context) error {
		id, err := uuid.Parse(c.Param("id"))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid note ID")
		}
		ctx := c.Request().Context()
		row, err := h.client.Note.Query().Where(note.IDEQ(id), note.UserIDEQ(c.Get("user_id").(int)), note.ProtectionModeEQ(note.ProtectionModeNone)).
			Select(note.FieldTitle, note.FieldContent).Only(ctx)
		if ent.IsNotFound(err) {
			return echo.NewHTTPError(http.StatusNotFound, "note unavailable")
		}
		if err != nil {
			return err
		}
		output, err := registry.Run(ctx, c.Param("tool"), row.Title, row.Content)
		if errors.Is(err, notetools.ErrUnknown) {
			return echo.NewHTTPError(http.StatusNotFound, "unknown note tool")
		}
		if err != nil {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "note tool could not finish").SetInternal(err)
		}
		return c.JSON(http.StatusOK, map[string]string{"text": output})
	})
}
